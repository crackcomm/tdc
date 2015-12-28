
save-godeps:
	godep save github.com/crackcomm/tdc

tdc-build:
	mkdir -p dist
	CGO_ENABLED=0 GOOS=linux go build -ldflags "-s" -a -installsuffix cgo -o ./dist/tdc ./main.go

install:
	go install github.com/crackcomm/tdc

docs: install
	sh -c 'TDC_HELP=`tdc --help` \
		tdc --input README.md.tmpl --output README.md --verbose'

dist: tdc-build

clean:
	rm -rf dist
