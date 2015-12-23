# tdc

[![Circle CI](https://img.shields.io/circleci/project/crackcomm/tdc.svg)](https://circleci.com/gh/crackcomm/tdc)

```sh
$ go install github.com/crackcomm/tdc
$ tdc --help
NAME:
   tdc - Templates Directory Compiler. Compiles directory of Go style templates.

   It will read all files in directory in directory unless path specified in --just-copy flag.
   All environment variables with prefix (flag: --prefix) will be applied to templates.

USAGE:
   tdc [global options] command [command options] <templates_dir>

COMMANDS:
   help, h	Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --output 						output destination (at least one is required)
   --input [--input option --input option]		input path (at least one is required)
   --just-copy [--just-copy option --just-copy option]	wildcard path for files to just copy
   --prefix "TDC_"					environment keys prefix
   --size-limit "1M"					template size limit [$TDC_FILE_SIZE_LIMIT]
   --concurrency "100"					
   -v							verbose
   --help, -h						show help
```
