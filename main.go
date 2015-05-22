// Copyright 2014-2015 Ekino. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"container/list"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/rjeczalik/notify"
)

const (
	FORMAT_GEM     = "gem-listen"
	FORMAT_GO_JSON = "go-json"
)

var (
	configuration = &Configuration{}
	exclude       *regexp.Regexp
	include       *regexp.Regexp
)

func debug(message string) {
	if configuration.Verbose == false {
		return
	}

	log.Print(message, "\n")
}

func info(message string) {
	log.Print(message, "\n")
}

type Operation struct {
	Path      string
	EventInfo notify.EventInfo
}

func (o Operation) Name() string {
	switch o.EventInfo.Event() {
	case notify.Create:
		return "CREATE"
	case notify.Remove:
		return "REMOVE"
	case notify.Rename:
		return "RENAME"
	default:
		return "WRITE"
	}
}

func (o Operation) GemName() string {
	switch o.EventInfo.Event() {
	case notify.Create:
		return "added"
	case notify.Remove:
		return "removed"
	case notify.Rename:
		return "removed"
	default:
		return "modified"
	}
}

type watcher struct {
	events chan notify.EventInfo
	ops    chan Operation
}

func newWatcher(path string, include, exclude *regexp.Regexp) (*watcher, error) {
	events := make(chan notify.EventInfo, 10)
	ops := make(chan Operation, 10)

	if err := notify.Watch(path+"/...", events, notify.All); err != nil {
		return nil, err
	}

	go func() {
		for ev := range events {
			if include.MatchString(ev.Path()) == false {
				debug(fmt.Sprintf("Skipping: does not match include path: %s", ev.Path()))
				continue
			}

			if exclude.MatchString(ev.Path()) == true {
				debug(fmt.Sprintf("Skipping: does match exclude path: %s", ev.Path()))
				continue
			}

			ops <- Operation{
				Path:      ev.Path(),
				EventInfo: ev,
			}
		}
	}()

	return &watcher{ops: ops, events: events}, nil
}

func (w *watcher) Watch() chan Operation {
	return w.ops
}

func (w *watcher) Close() {
	notify.Stop(w.events)
}

type Configuration struct {
	Path                string
	Server              string
	Command             string
	Exclude             string
	Include             string
	ServerFormat        string
	ParallelCommand     string
	ServerMaxConnection int
	Verbose             bool
	FileConfiguration   string
	PrintConfiguration  bool
}

func NewServer(conf *Configuration) *Server {
	return &Server{
		listeners:     list.New(),
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
	if s.listeners.Len()+1 > s.maxConnection {
		log.Printf("Drop client connection, too much connections\n")

		l.Write([]byte("Cannot register your client, too much connections\n"))
		l.Close()

		return
	}

	s.listeners.PushBack(l)
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
	flag.StringVar(&configuration.Path, "path", "", "The path to watch")
	flag.BoolVar(&configuration.Verbose, "verbose", false, "Display verbose information")
	flag.StringVar(&configuration.Server, "server", "", "Open a TCP server with local modification")
	flag.StringVar(&configuration.Command, "command", "", "The command to start, use {file} as placeholder for the file")
	flag.StringVar(&configuration.Exclude, "exclude", "", "Folder pattern to ignore")
	flag.StringVar(&configuration.Include, "include", "", "Folder pattern to include (all by default)")
	flag.IntVar(&configuration.ServerMaxConnection, "server-max-connection", 8, "The number of maximun connection, default=8")
	flag.StringVar(&configuration.ServerFormat, "server-format", "", fmt.Sprintf("Output format, default to: %s (also: %s compatible with gem listen)", FORMAT_GO_JSON, FORMAT_GEM))
	flag.StringVar(&configuration.ParallelCommand, "parallel-command", "", fmt.Sprintf("Run a command as a child process"))
	flag.StringVar(&configuration.FileConfiguration, "c", "", fmt.Sprintf("Configuration file to use"))
	flag.BoolVar(&configuration.PrintConfiguration, "p", false, fmt.Sprintf("Print the current configuration into stdout"))
}

func PrintConfiguration(conf *Configuration) {
	debug("")
	debug(fmt.Sprintf("> Configuration"))
	debug(fmt.Sprintf(">> Path: %s ", conf.Path))
	debug(fmt.Sprintf(">> Verbose: %b ", conf.Verbose))
	debug(fmt.Sprintf(">> Server: %s ", conf.Server))
	debug(fmt.Sprintf(">> Command: %s ", conf.Command))
	debug(fmt.Sprintf(">> Exclude: %s ", conf.Exclude))
	debug(fmt.Sprintf(">> Include: %s ", conf.Include))
	debug(fmt.Sprintf(">> ServerMaxConnection: %s ", conf.ServerMaxConnection))
	debug(fmt.Sprintf(">> ServerFormat: %s ", conf.ServerFormat))
	debug(fmt.Sprintf(">> ParallelCommand: %s ", conf.ParallelCommand))
	debug("")
	debug("")
}

// configure basic variable and check if the command can run properly
func configure() {
	var err error

	// parse command line argument
	flag.Parse()

	if configuration.FileConfiguration != "" {
		var fileConf = Configuration{}
		path, _ := filepath.Abs(configuration.FileConfiguration)

		if _, err := toml.DecodeFile(path, &fileConf); err != nil {
			info(fmt.Sprintf("Error while reading configuration file, %s", err))
		}

		if configuration.Command == "" {
			configuration.Command = fileConf.Command
		}

		if configuration.Path == "" {
			configuration.Path = fileConf.Path
		}

		if configuration.Server == "" {
			configuration.Server = fileConf.Server
		}

		if configuration.Exclude == "" {
			configuration.Exclude = fileConf.Exclude
		}

		if configuration.Include == "" {
			configuration.Include = fileConf.Include
		}

		if configuration.ServerMaxConnection == 0 {
			configuration.ServerMaxConnection = fileConf.ServerMaxConnection
		}

		if configuration.ServerFormat == "" {
			configuration.ServerFormat = fileConf.ServerFormat
		}

		if configuration.ParallelCommand == "" {
			configuration.ParallelCommand = fileConf.ParallelCommand
		}
	}

	// fix default value
	if configuration.Exclude == "" {
		configuration.Exclude = "((.*)/\\.git|\\.svn|node_modules|bower_components|/dist)"
	}

	if configuration.Include == "" {
		configuration.Include = "*"
	}

	if configuration.Path == "" {
		configuration.Path = "."
	}

	if configuration.ServerFormat == "" {
		configuration.ServerFormat = "127.0.0.1:4000"
	}

	if configuration.ServerFormat == "" {
		configuration.ServerFormat = FORMAT_GO_JSON
	}

	configuration.Path, _ = filepath.Abs(configuration.Path)

	if configuration.PrintConfiguration {
		encoder := toml.NewEncoder(os.Stdout)
		encoder.Encode(configuration)

		os.Exit(0)
	}

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

	PrintConfiguration(configuration)
}

func startParallelCommand() {
	//	var err error

	if configuration.ParallelCommand == "" {
		return
	}

	for {
		info(fmt.Sprintf("Running command: %s", configuration.ParallelCommand))

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

		info(fmt.Sprintf("Parallel command exited, start a new one in 2s"))

		time.Sleep(2 * time.Second)
	}
}

func formatMessage(op Operation, format string) (raw []byte, err error) {
	switch format {
	case FORMAT_GEM:
		buf := new(bytes.Buffer)

		// uint32(length) + json encoded array
		// https://github.com/guard/listen/blob/master/lib/listen/tcp/message.rb
		payload := []byte(fmt.Sprintf(
			`["file","%s","%s","%s",{}]`,
			op.GemName(), filepath.Dir(op.Path), filepath.Base(op.Path),
		))

		if err := binary.Write(buf, binary.BigEndian, uint32(len(payload))); err != nil {
			panic(err)
		}

		buf.Write(payload)
		return buf.Bytes(), nil

	case FORMAT_GO_JSON:
		return json.Marshal(struct{ Path, Type, Dir, Filename, Operation string }{
			Path:      op.Path,
			Type:      "file",
			Dir:       filepath.Dir(op.Path),
			Filename:  filepath.Base(op.Path),
			Operation: op.Name(),
		})
	}
	return nil, fmt.Errorf("Unrecognized format %s", format)
}

func main() {
	configure()

	info(fmt.Sprintf("Watching %s", configuration.Path))
	watcher, err := newWatcher(configuration.Path, include, exclude)
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
			case op := <-watcher.Watch():
				info(fmt.Sprintf("Operation: %s %s", op.Name(), op.Path))

				if configuration.Server != "" {
					raw, err := formatMessage(op, configuration.ServerFormat)
					if err != nil {
						panic(err)
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
			}
		}
	}()

	go startParallelCommand()

	if err != nil {
		log.Fatal(err)
	}

	<-done
}
