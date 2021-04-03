package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"

	"github.com/jessevdk/go-flags"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var opts struct {
	ListenAddr  string `short:"l" long:"listen" default:":9654" description:"http listen address to serve metrics"`
	MetricsPath string `short:"p" long:"metrics-path" default:"/command" description:"http path at which to serve metrics"`
	ConfigFile  string `short:"c" long:"config-file" default:"./config.yml" description:"path to config yaml file"`
}

func main() {
	parser := flags.NewParser(&opts, 0)
	pos, err := parser.Parse()
	if err != nil {
		parser.WriteHelp(os.Stderr)
		os.Exit(1)
	}
	if len(pos) > 0 {
		fmt.Fprintln(os.Stderr, "positional arguments are not supported\n")
		parser.WriteHelp(os.Stderr)
		os.Exit(1)
	}

	var config Config
	err = LoadConfig(opts.ConfigFile, &config)
	if err != nil {
		log.Fatalf("config load failed: %s", err)
	}

	for i, script := range config.Scripts {
		err = config.Scripts[i].Validate()
		if err != nil {
			log.Fatalf("script '%s' failed to validate: %s", script.Name, err)
		}
	}

	gather := CommandGatherer{
		config: &config,
	}

	log.Printf("Starting %s, version %s", path.Base(os.Args[0]), Version)

	if config.Startup != nil {
		startup := config.Startup
		startup.Name = "startup" // to ensure validation doesn't fail when no name in config

		err := startup.Validate()
		if err != nil {
			log.Fatalf("startup command failed to validate: %s", err)
		}

		cmd := exec.Command(startup.Command[0], startup.Command[1:]...)

		out, err := cmd.StdoutPipe()
		if err != nil {
			log.Fatalf("failed to open stdout of startup command: %s\n", err)
		}
		cmd.Stderr = cmd.Stdout
		read := bufio.NewReader(out)
		go func() {
			for err == nil {
				line, _, err := read.ReadLine()
				if err != nil {
					if !errors.Is(err, io.EOF) {
						log.Printf("stdout/stderr read: %s\n", err)
					}
					return
				}
				log.Printf("startup: %s\n", line)
			}
		}()

		if startup.Script != "" {
			stdin, err := cmd.StdinPipe()
			if err != nil {
				log.Fatalf("failed to open stdin of startup command: %s\n", err)
			}

			go func() {
				defer stdin.Close()
				io.WriteString(stdin, startup.Script)
			}()
		}

		log.Printf("running startup command")
		err = cmd.Run()
		if err != nil {
			log.Fatalf("startup command failed: %s\n", err)
		}
	}

	router := http.NewServeMux()
	router.Handle("/metrics", promhttp.Handler())
	router.Handle(opts.MetricsPath, promhttp.HandlerFor(gather, promhttp.HandlerOpts{}))
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
		<head><title>Command Exporter</title></head>
		<body>
		<h1>Script Exporter</h1>
		<p><a href='/metrics'>Metrics</a></p>
		<p><a href='/command'>Command</a></p>
		<p>Version ` + Version + `</p
		</body>
		</html>`))
	})

	server := &http.Server{
		Addr:    opts.ListenAddr,
		Handler: router,
	}

	log.Printf("Listening on %s at %s\n", opts.ListenAddr, opts.MetricsPath)
	log.Fatalln(server.ListenAndServe())
}
