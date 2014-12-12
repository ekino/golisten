// Copyright 2014-2015 Ekino. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package main

import (
	"gopkg.in/fsnotify.v1"
	"log"
	"os"
	"path/filepath"
	"flag"
	"net"
	"container/list"
	"encoding/json"
	"os/exec"
	"regexp"
	"fmt"
)

const (
	FORMAT_GEM = "gem-listen"
	FORMAT_GO_JSON = "go-json"
)

var (
	configuration = &Configuration{
		Path: ".",
		Verbose: true,
	}
)

type Operation struct {
	Path      string
	Type      string
	Dir       string
	Filename  string
	Op        fsnotify.Op
	Operation string
}

func (o Operation) Name() string {
	if o.Op&fsnotify.Create == fsnotify.Create {
		return "CREATE"
	}
	if o.Op&fsnotify.Remove == fsnotify.Remove {
		return "REMOVE"
	}
	if o.Op&fsnotify.Write == fsnotify.Write {
		return "WRITE"
	}
	if o.Op&fsnotify.Rename == fsnotify.Rename {
		return "RENAME"
	}
	if o.Op&fsnotify.Chmod == fsnotify.Chmod {
		return "CHMOD"
	}

	return "WRITE"
}

func (o Operation) GemName() string {
	if o.Op&fsnotify.Create == fsnotify.Create {
		return "added"
	}
	if o.Op&fsnotify.Remove == fsnotify.Remove {
		return "removed"
	}
	if o.Op&fsnotify.Write == fsnotify.Write {
		return "modified"
	}
	if o.Op&fsnotify.Rename == fsnotify.Rename {
		return "removed"
	}
	if o.Op&fsnotify.Chmod == fsnotify.Chmod {
		return "modified"
	}

	return "modified"
}

type Configuration struct {
	Path          string
	Server        string
	Verbose       bool
	Command       string
	Exclude       string
	Include       string
	MaxConnection int
	ServerFormat  string
}

func NewServer(conf *Configuration) *Server {
	return &Server{
		listeners: list.New(),
		maxConnection: conf.MaxConnection,
	}
}

type Server struct {
	listeners     *list.List
	maxConnection int
}

func (s *Server) SendMessage(message []byte) {
	for e := s.listeners.Front(); e != nil; e = e.Next() {
		conn := e.Value.(net.Conn)
		_, err := conn.Write(message)

		if err != nil {
			log.Printf("Error write: %+v\n", err)
			s.listeners.Remove(e)
			conn.Close()

			continue
		}

		conn.Write([]byte("\n"))
	}
}

func (s *Server) AddListener(l net.Conn) {

	if s.listeners.Len() + 1 > s.maxConnection {
		log.Printf("Drop client connection, too much connections\n")

		l.Write([]byte("Cannot register your client, too much connections\n"))
		l.Close()

		return
	}

	s.listeners.PushBack(l)
}

// Add folder recursively in the watcher scope
// The configuration object is used to get the root path and exclude/include information
func AddFolder(watcher *fsnotify.Watcher, conf *Configuration) error {
	path, _ := filepath.Abs(conf.Path)

	exclude, err := regexp.Compile(fmt.Sprintf("^(.*)%s(.*)$", conf.Exclude))

	if err != nil {
		panic(err)
	}

	include, err := regexp.Compile(fmt.Sprintf("^(.*)%s(.*)$", conf.Include))

	if err != nil {
		panic(err)
	}

	cpt := 0

	err = filepath.Walk(path, func(path string, f os.FileInfo, err error) error {

		if err != nil {
			log.Printf("Folder does not exist: ", err)
			panic(err)
		}

		if f.IsDir() {

			if include.Match([]byte(path)) == true && exclude.Match([]byte(path)) == false {

				log.Printf("Add directory: %s\n", path)

				watcher.Add(path)

				cpt += 1
			}

		}

		return nil
	})

	log.Printf("%d folders added\n", cpt)

	return err
}

func StartServer(server *Server, conf *Configuration) {
	if conf.Server == "" {
		log.Println("Server disabled")

		return
	}

	// Listen for incoming connections.
	l, err := net.Listen("tcp", conf.Server)

	if err != nil {
		log.Println("Error listening:", err.Error())
		os.Exit(1)
	}

	// Close the listener when the application closes.
	defer l.Close()

	log.Println("Listening on " + conf.Server)

	for {
		// Listen for an incoming connection.
		conn, err := l.Accept()
		if err != nil {
			log.Println("Error accepting: ", err.Error())
			os.Exit(1)
		}

		server.AddListener(conn)
	}
}

func init() {
	flag.StringVar(&configuration.Path, "path", ".", "The path to watch")
	flag.BoolVar(&configuration.Verbose, "verbose", true, "Display verbose information")
	flag.StringVar(&configuration.Server, "server", "", "Open a TCP server with local modification")
	flag.StringVar(&configuration.Command, "command", "", "The command to start, use {file} as placeholder for the file")
	flag.StringVar(&configuration.Exclude, "exclude", "(\\.svn|\\.git|node_modules|bower_components|.idea)", "Folder pattern to ignore")
	flag.StringVar(&configuration.Include, "include", "*", "Folder pattern to include (all by default)")
	flag.IntVar(&configuration.MaxConnection, "max-connection", 8, "The number of maximun connection, default=8")
	flag.StringVar(&configuration.ServerFormat, "server-format", FORMAT_GO_JSON, fmt.Sprintf("Output format, default to: %s (also: %s compatible with gem listen)", FORMAT_GO_JSON, FORMAT_GEM ))
}


func main() {

	flag.Parse()

	if configuration.Server == "" && configuration.Command == "" {
		log.Fatal("You need to set either a -server option or a -command option")
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	done := make(chan bool)

	server := NewServer(configuration)

	go StartServer(server, configuration)

	running := false

	go func() {

		for {
			select {
			case event := <-watcher.Events:
				path, _ := filepath.Abs(event.Name)

				// move this into a NewOperation function
				op := Operation{
					Type: "file",
					Path: path,
					Dir: filepath.Dir(path),
					Filename: filepath.Base(path),
					Op: event.Op,
				}

				op.Operation = op.Name()

				if configuration.Server != "" {
					// format compatible with gem listen
					// d["file","added","/Users/rande/Projects/go/gonode/src/github.com/rande/gonodeexplorer","test",{}]

					var raw []byte
					if configuration.ServerFormat == FORMAT_GEM {
						raw = []byte(fmt.Sprintf("d[\"%s\",\"%s\",\"%s\",\"%s\",{}])", op.Type, op.GemName(), op.Dir, op.Filename ))
					} else {
						raw, _ = json.Marshal(op)
					}

					server.SendMessage(raw)
				}

				if configuration.Command != "" {
					if running {
						log.Printf("SKIPPING: Command already running\n", configuration.Command)
					} else {
						go func() {
							running = true
							log.Printf("Running command: %s\n", configuration.Command)

							output, err := exec.Command("sh", "-c", configuration.Command).CombinedOutput()
							if err != nil {
								log.Printf("Fail to run the command: %s\n", err)
							}

							log.Printf("Output command\n%s\n", output)

							running = false
						}()
					}
				}

				log.Printf("Operation: %+v \n", op)
			case err := <-watcher.Errors:
				log.Println("error:", err)
			}
		}
	}()

	log.Printf("Scanning folders: %s\n", configuration)

	AddFolder(watcher, configuration)

	log.Printf("End scanning.")

	if err != nil {
		log.Fatal(err)
	}

	<-done
}
