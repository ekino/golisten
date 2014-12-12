SHELL=/bin/bash

build: deps
	go build

release: deps golang-crosscompile
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

golang-crosscompile:
	git clone https://github.com/davecheney/golang-crosscompile.git

deps:
	go get gopkg.in/fsnotify.v1
