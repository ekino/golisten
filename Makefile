SHELL=/bin/bash
VERSION=0.1.1

.PHONY: build release deps golang-crosscompile

build: deps golang-crosscompile
	source golang-crosscompile/crosscompile.bash; \
	go-darwin-386 build -o release/golisten-Darwin-i386; \
	go-darwin-amd64 build -o release/golisten-Darwin-x86_64; \
	go-linux-386 build -o release/golisten-Linux-i386; \
	go-linux-386 build -o release/golisten-Linux-i686; \
	go-linux-amd64 build -o release/golisten-Linux-x86_64; \
	go-linux-arm build -o release/golisten-Linux-armv6l; \
	go-linux-arm build -o release/golisten-Linux-armv7l; \
	go-freebsd-386 build -o release/golisten-FreeBSD-i386; \
	go-freebsd-amd64 build -o release/golisten-FreeBSD-amd64; \
	go-windows-386 build -o release/golisten.exe

release:
	github-release release --user ekino --repo golisten --tag v$(VERSION) --name "golisten v$(VERSION)" --pre-release
	github-release upload --user ekino --repo golisten --tag v$(VERSION) --name "golisten-darwin-i386" --file release/golisten-Darwin-i386
	github-release upload --user ekino --repo golisten --tag v$(VERSION) --name "golisten-darwin-x86_64" --file release/golisten-Darwin-x86_64
	github-release upload --user ekino --repo golisten --tag v$(VERSION) --name "golisten-freebsd-amd64" --file release/golisten-FreeBSD-amd64
	github-release upload --user ekino --repo golisten --tag v$(VERSION) --name "golisten-freebsd-i386" --file release/golisten-FreeBSD-i386
	github-release upload --user ekino --repo golisten --tag v$(VERSION) --name "golisten-linux-armv6l" --file release/golisten-Linux-armv6l
	github-release upload --user ekino --repo golisten --tag v$(VERSION) --name "golisten-linux-armv7l" --file release/golisten-Linux-armv7l
	github-release upload --user ekino --repo golisten --tag v$(VERSION) --name "golisten-linux-i386" --file release/golisten-Linux-i386
	github-release upload --user ekino --repo golisten --tag v$(VERSION) --name "golisten-linux-i686" --file release/golisten-Linux-i686
	github-release upload --user ekino --repo golisten --tag v$(VERSION) --name "golisten-linux-x86_64" --file release/golisten-Linux-x86_64

golang-crosscompile:
	rm -rf golang-crosscompile
	git clone https://github.com/davecheney/golang-crosscompile.git

deps:
	go get gopkg.in/fsnotify.v1
	go get github.com/aktau/github-release
