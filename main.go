package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"text/template"

	"github.com/codegangsta/cli"
	"github.com/ryanuber/go-glob"
	"github.com/tower-services/utils/cliutils"
)

func init() {
	log.SetFlags(0)
}

func main() {
	app := cli.NewApp()
	app.Name = "tdc"
	app.Version = "0.0.1"
	app.Usage = `Templates Directory Compiler. Compiles directory of Go style templates.

   It will read all files in directory in directory unless path specified in --just-copy flag.
   All environment variables with prefix (flag: --prefix) will be applied to templates.`
	app.ArgsUsage = "<templates_dir>"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "output",
			Usage: "Output destination (at least one is required)",
		},
		cli.StringSliceFlag{
			Name:  "input",
			Usage: "Input path (at least one is required)",
		},
		cli.StringSliceFlag{
			Name:  "just-copy",
			Usage: "Doesnt compiles templates in specified path, just copies them; accepts glob-like wildcards",
		},
		cli.StringFlag{
			Name:  "prefix",
			Value: "TDC_",
			Usage: "Values environment prefix",
		},
		cli.GenericFlag{
			Name:   "size-limit",
			Usage:  "template size limit",
			Value:  cliutils.Megabytes{Value: 1},
			EnvVar: "TDC_FILE_SIZE_LIMIT",
		},
		cli.IntFlag{
			Name:  "concurrency",
			Value: 100,
		},
	}
	app.Before = func(c *cli.Context) error {
		var errs []string
		if len(c.String("output")) == 0 {
			errs = append(errs, "Flag --output is required.")
		}
		if len(c.StringSlice("input")) == 0 {
			errs = append(errs, "At least one --input flag is required.")
		}
		if len(errs) != 0 {
			return errors.New(strings.Join(errs, "\n"))
		}
		return nil
	}
	app.Action = func(c *cli.Context) {
		files := make(chan *inputFile, 10000)

		env, err := listToMap(os.Environ())
		if err != nil {
			log.Fatalf("[env] error parsing environment: %v", err)
		}

		log.Printf("%#v", env)

		data := make(map[string]string)
		prefix := c.String("prefix")

		for key, value := range env {
			if strings.HasPrefix(key, prefix) {
				key = strings.TrimPrefix(key, prefix)
				data[key] = value
			}
		}

		var wg sync.WaitGroup
		for i := 0; i < c.Int("concurrency"); i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for req := range files {
					if req.mkdir {
						log.Printf("[mkdir] %s", req.dest)
						if err := os.MkdirAll(req.dest, os.ModePerm); err != nil {
							log.Fatalf("[mkdir] error: %v", err)
						}
						continue
					}
					log.Printf("[template] %q => %q", req.file, req.dest)
					tmpl, err := template.ParseFiles(req.file)
					if err != nil {
						log.Fatalf("[template] read error: %v", err)
					}
					out, err := os.Create(req.dest)
					if err != nil {
						log.Fatalf("[file] create error: %v", err)
					}
					if err := tmpl.Execute(out, data); err != nil {
						log.Fatalf("[template] execute error: %v", err)
					}
					if err := out.Close(); err != nil {
						log.Fatalf("[file] close error: %v", err)
					}
				}
			}()
		}

		processed := make(map[string]bool)
		for _, input := range c.StringSlice("input") {
			filepath.Walk(input, func(path string, info os.FileInfo, err error) error {
				if processed[path] == true {
					return nil
				}
				processed[path] = true

				if info.IsDir() {
					dest := filepath.Join(c.String("output"), strings.TrimPrefix(path, input))
					files <- &inputFile{dest: dest, mkdir: true}
					return nil
				}

				for _, globPath := range c.StringSlice("just-copy") {
					if glob.Glob(globPath, path) {
						dest := filepath.Join(c.String("output"), strings.TrimPrefix(path, input))
						files <- &inputFile{file: path, dest: dest, justCopy: true}
						return nil
					}
				}
				if limit, ok := c.Generic("size-limit").(*cliutils.Megabytes); ok && uint64(info.Size()) > limit.Value {
					log.Printf("Too big: %s", path)
					return nil
				}

				dest := filepath.Join(c.String("output"), strings.TrimPrefix(path, input))
				files <- &inputFile{file: path, dest: dest}
				return nil
			})
		}

		close(files)

		wg.Wait()
		log.Println("done")
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

type inputFile struct {
	file, dest      string
	justCopy, mkdir bool
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
