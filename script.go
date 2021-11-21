package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os/exec"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type Script struct {
	Name    string
	Raw     bool
	Timeout time.Duration
	Command []string
	Script  string
}

func (s *Script) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type scriptT struct {
		Name    string
		Raw     bool
		Timeout time.Duration
		Command []string `yaml:"-"`
		Script  string
	}

	var t scriptT
	err := unmarshal(&t)
	if err != nil {
		return err
	}

	*s = Script(t)

	type scriptCmd struct {
		Command interface{}
	}
	var c scriptCmd
	err = unmarshal(&c)
	if err != nil {
		return err
	}

	switch sc := c.Command.(type) {
	case string:
		s.Command = strings.Split(sc, " ")
	case []string:
		s.Command = sc
	case []interface{}:
		s.Command = make([]string, len(sc))
		for i, ss := range sc {
			if str, ok := ss.(string); ok {
				s.Command[i] = str
			} else {
				return fmt.Errorf("cannot convert %T %v to string", ss, ss)
			}
		}
	}

	return nil
}

func (s *Script) Validate() error {
	if s.Name == "" {
		return errors.New("no name provided")
	}
	if s.Timeout == 0 {
		s.Timeout = time.Second * 10
	}
	if len(s.Command) < 1 {
		if s.Script == "" {
			return errors.New("no script or command provided")
		}
	}
	return nil
}

func (s *Script) Run(ctx context.Context) (func() (int, error), io.ReadCloser, error) {
	ctx, cancel := context.WithTimeout(ctx, s.Timeout)

	argv := s.Command
	if len(argv) < 1 {
		argv = []string{"/bin/sh", "-e"}
	}

	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	stdout, err := cmd.StdoutPipe()

	// Pipe script to stdin
	if s.Script != "" {
		stdin, inErr := cmd.StdinPipe()
		if err != nil && inErr != nil {
			err = prometheus.MultiError{err, inErr}
		} else {
			err = inErr
		}

		go func() {
			io.WriteString(stdin, s.Script)
			stdin.Close()
		}()
	}

	// Forcefully close stdout to make cmd.Wait() return
	go func() {
		<-ctx.Done()
		stdout.Close()
	}()

	wait := func() (int, error) {
		defer cancel()

		exitCode := 0
		err := cmd.Wait()
		if err != nil {
			var exitErr *exec.ExitError

			exitCode = -1
			if errors.Is(err, context.DeadlineExceeded) ||
				errors.Is(ctx.Err(), context.DeadlineExceeded) {
				log.Printf("command '%s' timed out\n", s.Name)
			} else if errors.As(err, &exitErr) {
				err = nil
				exitCode = exitErr.ExitCode()

				stderr := string(exitErr.Stderr)

				log.Printf("command '%s' exited %d\n", s.Name, exitCode)
				if strings.TrimSpace(stderr) != "" {
					log.Printf("stderr:\n%s", stderr)
				}
			}
		}
		return exitCode, err
	}

	return wait, stdout, cmd.Start()
}
