package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"text/template"

	"github.com/codegangsta/cli"
	"github.com/golang/glog"
	"github.com/ryanuber/go-glob"
	"github.com/tower-services/utils/cliutils"
)

func main() {
	defer glog.Flush()
	flag.CommandLine.Parse([]string{"-logtostderr"})

	app := cli.NewApp()
	app.Name = "tdc"
	app.Version = ""
	app.HideVersion = true
	app.Usage = `Templates Directory Compiler. Compiles directory of Go style templates.

   It will read all files in directory in directory unless path specified in --just-copy flag.
   All environment variables with prefix (flag: --prefix) will be applied to templates.`
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "output",
			Usage: "output destination (at least one is required)",
		},
		cli.StringSliceFlag{
			Name:  "input",
			Usage: "input path (at least one is required)",
		},
		cli.StringSliceFlag{
			Name:  "just-copy",
			Usage: "wildcard path for files to just copy",
		},
		cli.StringSliceFlag{
			Name:  "json-file",
			Usage: "json file containing variables",
		},
		cli.StringSliceFlag{
			Name:  "ignore-ext",
			Usage: "extensions to ignore",
		},
		cli.StringFlag{
			Name:  "prefix",
			Usage: "environment keys prefix",
			Value: "TDC_",
		},
		cli.GenericFlag{
			Name:   "size-limit",
			EnvVar: "TDC_FILE_SIZE_LIMIT",
			Usage:  "template size limit",
			Value:  &cliutils.Megabytes{Value: 1},
		},
		cli.IntFlag{
			Name:  "concurrency",
			Value: 100,
		},
		cli.BoolFlag{
			Name:  "verbose, v",
			Usage: "verbose",
		},
	}
	app.Action = func(c *cli.Context) {
		if c.String("output") == "" || len(c.StringSlice("input")) == 0 {
			cli.ShowAppHelp(c)
			return
		}

		// Disable log outut if --verbose flag is on
		if c.Bool("v") {
			flag.Set("v", "1")
		}

		// Get --ignore-ext list and make sure they are prefixed with a dot
		ignoreExts := c.StringSlice("ignore-ext")
		for i, ext := range ignoreExts {
			if !strings.HasPrefix(ext, ".") {
				ignoreExts[i] = fmt.Sprintf(".%s", ext)
			}
		}

		// Get template data from environment variables with specified prefix
		data, err := getEnvData(c.String("prefix"))
		if err != nil {
			glog.Fatal(err)
		}

		// Compiler runtime compiles the templates
		cr := &compilerRuntime{
			files: make(chan *inputFile, 10000),
			data:  data,
		}

		// Set extra data from --json-file flag
		for _, filename := range c.StringSlice("json-file") {
			if err := cr.setExtraDataFromFile(filename); err != nil {
				glog.Fatal(err)
			}
		}

		// Start --concurrency workers for compilation
		var wg sync.WaitGroup
		for i := 0; i < c.Int("concurrency"); i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := cr.consumeFiles(); err != nil {
					glog.Fatal(err)
				}
			}()
		}

		// Map of processed files
		processed := make(map[string]bool)

		// Visit all --input directories
		for _, input := range c.StringSlice("input") {
			filepath.Walk(input, func(path string, info os.FileInfo, err error) error {
				// Ignore if it's directory or already processed
				if (info != nil && info.IsDir()) || processed[path] == true {
					return nil
				}

				// Return error if not empty
				if err != nil {
					return err
				}

				// Add to map of processed files
				processed[path] = true

				// Ignore the file if extension is in --ignore-ext
				if stringIn(filepath.Ext(path), ignoreExts) {
					glog.V(1).Infof("[ext] ignoring: %q", path)
					return nil
				}

				// Ignore if the file size is bigger than --size-limit (default: 1 megabyte)
				if limit, ok := c.Generic("size-limit").(*cliutils.Megabytes); ok && uint64(info.Size()) > limit.Bytes() {
					glog.V(1).Infof("not copying %q: too big", path)
					return nil
				}

				// The file with template or to copy
				file := &inputFile{
					file: path,
					dest: filepath.Join(c.String("output"), strings.TrimPrefix(path, input)),
				}

				// If filename matches path given in --just-copy, set justCopy to true
				for _, globPath := range c.StringSlice("just-copy") {
					if glob.Glob(globPath, path) {
						file.justCopy = true
					}
				}

				cr.files <- file
				return nil
			})
		}
		close(cr.files)
		wg.Wait()
	}

	if err := app.Run(os.Args); err != nil {
		glog.Fatal(err)
	}
}

type compilerRuntime struct {
	data  map[string]interface{}
	files chan *inputFile
}

type inputFile struct {
	file, dest string
	justCopy   bool
}

func (cr *compilerRuntime) consumeFiles() (err error) {
	for file := range cr.files {
		err = cr.handleFile(file)
		if err != nil {
			return
		}
	}
	return
}

func (cr *compilerRuntime) handleFile(file *inputFile) (err error) {
	dest, _ := filepath.Abs(file.dest)
	fname, _ := filepath.Abs(file.file)

	// Make sure output directory exists
	if err := os.MkdirAll(filepath.Dir(dest), os.ModePerm); err != nil {
		return err
	}

	// Copy the file if --just-copy flag was enabled on this path
	if file.justCopy {
		glog.V(1).Infof("[copy] %q => %q", fname, dest)
		return copyFile(fname, dest)
	}

	// Parse the template
	tmpl, err := template.ParseFiles(file.file)
	if err != nil {
		return
	}

	// Create output file
	out, err := os.Create(file.dest)
	if err != nil {
		return err
	}
	defer out.Close()

	// Execute the template with given data into the file
	glog.V(1).Infof("[template] %q => %q", fname, dest)
	return tmpl.Execute(out, cr.data)
}

func (cr *compilerRuntime) setExtraData(extra map[string]interface{}) {
	for key, value := range extra {
		cr.data[key] = value
	}
}

func (cr *compilerRuntime) setExtraDataFromFile(filename string) (err error) {
	body, err := ioutil.ReadFile(filename)
	if err != nil {
		return
	}
	data := make(map[string]interface{})
	err = json.Unmarshal(body, &data)
	if err != nil {
		return
	}
	cr.setExtraData(data)
	return
}

func getEnvData(prefix string) (data map[string]interface{}, err error) {
	data = make(map[string]interface{})

	// Get map of environment variables
	env, err := listToMap(os.Environ())
	if err != nil {
		return
	}

	// Extract only values with specified prefix
	for key, value := range env {
		if strings.HasPrefix(key, prefix) {
			key = strings.TrimPrefix(key, prefix)
			data[key] = value
		}
	}
	return
}

func listToMap(list []string) (result map[string]string, err error) {
	result = make(map[string]string)
	for _, keyValue := range list {
		i := strings.Index(keyValue, "=")
		if i <= 0 {
			return nil, fmt.Errorf("%q is not valid", keyValue)
		}
		key := keyValue[:i]
		value := keyValue[i+1:]
		result[key] = value
	}
	return
}

func copyFile(src, dest string) (err error) {
	r, err := os.Open(src)
	if err != nil {
		return
	}
	defer r.Close()
	w, err := os.Create(dest)
	if err != nil {
		return
	}
	defer w.Close()
	_, err = io.Copy(w, r)
	return
}

func stringIn(str string, list []string) bool {
	for _, v := range list {
		if v == str {
			return true
		}
	}
	return false
}
