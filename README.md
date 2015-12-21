# tdc

```sh
$ go install github.com/crackcomm/tdc
$ tdc --help
NAME:
   tdc - Templates Directory Compiler. Compiles directory of Go style templates.

   It will read all files in directory in directory unless path specified in --just-copy flag.
   All environment variables with prefix (flag: --prefix) will be applied to templates.

USAGE:
   tdc [global options] command [command options] <templates_dir>

VERSION:
   0.0.1

COMMANDS:
   help, h	Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --output 						Output destination (at least one is required)
   --input [--input option --input option]		Input path (at least one is required)
   --just-copy [--just-copy option --just-copy option]	Doesnt compiles templates in specified path, just copies them; accepts glob-like wildcards
   --prefix "TDC_"					Values environment prefix
   --size-limit "1M"					template size limit [$TDC_FILE_SIZE_LIMIT]
   --concurrency "100"					
   --help, -h						show help
   --version, -v					print the version
```
