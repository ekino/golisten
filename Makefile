SHELL=/bin/bash
VERSION=0.1.2

.PHONY: build release deps golang-crosscompile

build: deps golang-crosscompile
	source golang-crosscompile/crosscompile.bash; \
	go-darwin-386 build -o release/golisten-darwin-i386; \
	go-darwin-amd64 build -o release/golisten-darwin-amd64; \
	go-freebsd-386 build -o release/golisten-freebsd-i386; \
	go-freebsd-amd64 build -o release/golisten-freebsd-amd64; \
	go-freebsd-arm build -o release/golisten-freebsd-arm; \
	go-linux-386 build -o release/golisten-linux-386; \
	go-linux-amd64 build -o release/golisten-linux-amd64; \
	go-linux-arm build -o release/golisten-linux-arm; \
	go-windows-386 build -o release/golisten-windows-386.exe
	go-windows-amd64 build -o release/golisten-windows-amd64.exe
	go-openbsd-386 build -o release/golisten-openbsd-386; \
	go-openbsd-amd64 build -o release/golisten-openbsd-amd64; \


release:
	github-release release --user ekino --repo golisten --tag v$(VERSION) --name "golisten v$(VERSION)" --pre-release
	github-release upload --user ekino --repo golisten --tag v$(VERSION) --name "golisten-darwin-i386" --file release/golisten-darwin-i386
	github-release upload --user ekino --repo golisten --tag v$(VERSION) --name "golisten-darwin-amd64" --file release/golisten-darwin-amd64
	github-release upload --user ekino --repo golisten --tag v$(VERSION) --name "golisten-freebsd-i386" --file release/golisten-freebsd-i386
	github-release upload --user ekino --repo golisten --tag v$(VERSION) --name "golisten-freebsd-amd64" --file release/golisten-freebsd-amd64
	github-release upload --user ekino --repo golisten --tag v$(VERSION) --name "golisten-freebsd-arm" --file release/golisten-freebsd-arm
	github-release upload --user ekino --repo golisten --tag v$(VERSION) --name "golisten-linux-386" --file release/golisten-linux-386
	github-release upload --user ekino --repo golisten --tag v$(VERSION) --name "golisten-linux-amd64" --file release/golisten-linux-amd64
	github-release upload --user ekino --repo golisten --tag v$(VERSION) --name "golisten-linux-arm" --file release/golisten-linux-arm
	github-release upload --user ekino --repo golisten --tag v$(VERSION) --name "golisten-windows-386.exe" --file "release/golisten-windows-386.exe"
	github-release upload --user ekino --repo golisten --tag v$(VERSION) --name "golisten-windows-amd64.exe" --file "release/golisten-windows-amd64.exe"
	github-release upload --user ekino --repo golisten --tag v$(VERSION) --name "golisten-openbsd-386" --file "release/golisten-openbsd-386"
	github-release upload --user ekino --repo golisten --tag v$(VERSION) --name "golisten-openbsd-amd64" --file "release/golisten-openbsd-amd64"

golang-crosscompile:
	rm -rf golang-crosscompile
	git clone https://github.com/davecheney/golang-crosscompile.git

deps:
	go get gopkg.in/fsnotify.v1
	go get github.com/aktau/github-release
