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
	"io"
	"time"
	"runtime"
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
	exclude *regexp.Regexp
	include *regexp.Regexp
)

func debug(message string) {
	if (configuration.Verbose == false) {
		return
	}

	log.Print(message, "\n")
}

func info(message string) {
	log.Print(message, "\n")
}

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
	Path                string
	Server              string
	Verbose             bool
	Command             string
	Exclude             string
	Include             string
	ServerMaxConnection int
	ServerFormat        string
	ParallelCommand     string
}

func NewServer(conf *Configuration) *Server {
	return &Server{
		listeners: list.New(),
		maxConnection: conf.ServerMaxConnection,
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
			info(fmt.Sprintf("Error writing to: %+v, removing connection from the stacks", err))
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

	cpt := 0

	err := filepath.Walk(path, func(path string, f os.FileInfo, err error) error {
		if err != nil {
			debug(fmt.Sprintf("Folder does not exist: ", err))
			panic(err)
		}

		if f.IsDir() {
			if include.Match([]byte(path)) == true && exclude.Match([]byte(path)) == false {

				debug(fmt.Sprintf("Add folder: %s", path))
				watcher.Add(path)

				cpt += 1
			}
		}

		return nil
	})

	info(fmt.Sprintf("%d folders added", cpt))

	return err
}

func StartServer(server *Server, conf *Configuration) {
	if conf.Server == "" {
		info("Server disabled")

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

	info(fmt.Sprintf("Listening on %s", conf.Server))

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

func GetCommand(command string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.Command("cmd", "/C %s", configuration.ParallelCommand)
	}

	return exec.Command("sh", "-c", configuration.ParallelCommand)
}

func init() {
	flag.StringVar(&configuration.Path, "path", ".", "The path to watch")
	flag.BoolVar(&configuration.Verbose, "verbose", false, "Display verbose information")
	flag.StringVar(&configuration.Server, "server", "", "Open a TCP server with local modification")
	flag.StringVar(&configuration.Command, "command", "", "The command to start, use {file} as placeholder for the file")
	flag.StringVar(&configuration.Exclude, "exclude", "((.*)/\\.git|\\.svn|node_modules|bower_components|/dist)", "Folder pattern to ignore")
	flag.StringVar(&configuration.Include, "include", "*", "Folder pattern to include (all by default)")
	flag.IntVar(&configuration.ServerMaxConnection, "server-max-connection", 8, "The number of maximun connection, default=8")
	flag.StringVar(&configuration.ServerFormat, "server-format", FORMAT_GO_JSON, fmt.Sprintf("Output format, default to: %s (also: %s compatible with gem listen)", FORMAT_GO_JSON, FORMAT_GEM ))
	flag.StringVar(&configuration.ParallelCommand, "parallel-command", "", fmt.Sprintf("Run a command as a child process"))
}

// configure basic variable and check if the command can run properly
func configure() {
	var err error

	// parse command line argument
	flag.Parse()

	// check if the command can run
	if configuration.Server == "" && configuration.Command == "" {
		log.Fatal("You need to set either a -server option or a -command option")
	}

	// parse Exclude and Include parameter
	exclude, err = regexp.Compile(fmt.Sprintf("^(.*)%s(.*)$", configuration.Exclude))

	if err != nil {
		panic(err)
	}

	include, err = regexp.Compile(fmt.Sprintf("^(.*)%s(.*)$", configuration.Include))

	if err != nil {
		panic(err)
	}

	debug("")
	debug(fmt.Sprintf("> Configuration"))
	debug(fmt.Sprintf(">> Path: %s ", configuration.Path))
	debug(fmt.Sprintf(">> Path: %b ", configuration.Verbose))
	debug(fmt.Sprintf(">> Server: %s ", configuration.Server))
	debug(fmt.Sprintf(">> Command: %s ", configuration.Command))
	debug(fmt.Sprintf(">> Exclude: %s ", configuration.Exclude))
	debug(fmt.Sprintf(">> Include: %s ", configuration.Include))
	debug(fmt.Sprintf(">> ServerMaxConnection: %s ", configuration.ServerMaxConnection))
	debug(fmt.Sprintf(">> ServerFormat: %s ", configuration.ServerFormat))
	debug(fmt.Sprintf(">> ParallelCommand: %s ", configuration.ParallelCommand))
	debug("")
	debug("")
}

func getWatcher() *fsnotify.Watcher {
	// start the watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}

	debug(fmt.Sprintf("Scanning folders",))

	AddFolder(watcher, configuration)

	debug(fmt.Sprintf("End scanning."))

	return watcher
}

func startParallelCommand() {
//	var err error

	if configuration.ParallelCommand == "" {
		return;
	}

	for {
		debug(fmt.Sprintf("Running command: %s", configuration.ParallelCommand))

		c := GetCommand(configuration.ParallelCommand)

		errReader, err := c.StderrPipe()
		outReader, err := c.StdoutPipe()

		err = c.Start()

		if err != nil {
			panic(err)
		}

		go io.Copy(os.Stdout, errReader)
		go io.Copy(os.Stdout, outReader)

		c.Wait()

		debug(fmt.Sprintf("Parallel command exited, start a new one in 5s"))

		time.Sleep(5 * time.Second)
	}

}

func main() {
	var err error


	info("golisten is a development tools and not suitable for production usage")
	info("   more information can be found at https://github.com/ekino/golisten")
	info("                                                Thomas Rabaix @ Ekino")
	info("")

	configure()

	// start the watcher
	watcher := getWatcher()

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

				debug(fmt.Sprintf("Operation: %s", op.Name()))

				if include.Match([]byte(path)) == false  {
					debug(fmt.Sprintf("Skipping: does not match include path: %s", op.Path))

					continue
				}

				if exclude.Match([]byte(path)) == true {
					debug(fmt.Sprintf("Skipping: does match exclude path: %s", op.Path))

					continue
				}

				debug(fmt.Sprintf("Event path: %s", op.Path))

				if configuration.Server != "" {
					// format compatible with gem listen
					// d["file","added","/Users/rande/Projects/go/gonode/src/github.com/rande/gonodeexplorer","test",{}]

					var raw []byte
					if configuration.ServerFormat == FORMAT_GEM {
						raw = []byte(fmt.Sprintf("d[\"%s\",\"%s\",\"%s\",\"%s\",{}]", op.Type, op.GemName(), op.Dir, op.Filename ))
					} else {
						raw, _ = json.Marshal(op)
					}

					debug(fmt.Sprintf("Raw message: %s", raw))
					server.SendMessage(raw)
				}

				if configuration.Command != "" {
					if running {
						debug(fmt.Sprintf("SKIPPING: Command already running", configuration.Command))
					} else {
						go func() {
							running = true

							debug(fmt.Sprintf("Running command: %s", configuration.Command))

							c := GetCommand(configuration.Command)
							output, err := c.CombinedOutput()
							if err != nil {
								info(fmt.Sprintf("Fail to run the command: %s", err))
							}

							info(fmt.Sprintf("Output command\n%s", output))

							running = false
						}()
					}
				}

				info(fmt.Sprintf("Operation: %s => %s", op.Name(), op.Filename))
			case err := <-watcher.Errors:
				info(fmt.Sprint("error: %s", err))
			}
		}
	}()

	go startParallelCommand()

	if err != nil {
		log.Fatal(err)
	}

	<-done
}
