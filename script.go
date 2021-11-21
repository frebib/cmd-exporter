package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os/exec"
	"strings"
	"time"
)

type Result struct {
	ExitCode int
	Stdout   io.ReadSeeker
	Stderr   io.ReadSeeker
}

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

	// Timeout in seconds, not nanoseconds
	s.Timeout = s.Timeout * time.Second

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

func (s *Script) Exec(ctx context.Context) (*Result, error) {
	ctx, cancel := context.WithTimeout(ctx, s.Timeout)
	defer cancel()

	argv := s.Command
	if len(argv) < 1 {
		argv = []string{"/bin/sh", "-e"}
	}

	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)

	if s.Script != "" {
		stdin, err := cmd.StdinPipe()
		if err != nil {
			log.Fatal(err)
		}

		go func() {
			defer stdin.Close()
			io.WriteString(stdin, s.Script)
		}()
	}

	result := &Result{
		ExitCode: 0,
		Stdout:   nil,
		Stderr:   nil,
	}

	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError

		if errors.Is(err, context.DeadlineExceeded) ||
			errors.Is(ctx.Err(), context.DeadlineExceeded) {
			log.Printf("command '%s' timed out\n", s.Name)
			result.ExitCode = -1

		} else if errors.As(err, &exitErr) {
			result.ExitCode = exitErr.ExitCode()
			result.Stderr = bytes.NewReader(exitErr.Stderr)

			log.Printf("command '%s' exited %d\n", s.Name, result.ExitCode)
			if strings.TrimSpace(string(exitErr.Stderr)) != "" {
				log.Printf("stderr:\n%s", string(exitErr.Stderr))
			}

		} else {
			return nil, err
		}
	}

	result.Stdout = bytes.NewReader(output)

	return result, nil
}
