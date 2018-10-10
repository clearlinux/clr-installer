// Copyright © 2018 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package cmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/clearlinux/clr-installer/log"
)

type runLogger struct{}

var (
	httpsProxy string
)

// SetHTTPSProxy defines the HTTPS_PROXY env var value for all the cmd executions
func SetHTTPSProxy(addr string) {
	httpsProxy = addr
}

func (rl runLogger) Write(p []byte) (n int, err error) {
	for _, curr := range strings.Split(string(p), "\n") {
		if curr == "" {
			continue
		}

		log.Debug(curr)
	}
	return len(p), nil
}

// RunAndLog executes a command (similar to Run) but takes care of writing
// the output to default logger
func RunAndLog(args ...string) error {
	return Run(runLogger{}, args...)
}

// RunAndLogWithEnv does the same as RunAndLog but it changes the execution's environment
// variables adding the provided ones by the env argument
func RunAndLogWithEnv(env map[string]string, args ...string) error {
	return run(nil, runLogger{}, env, args...)
}

// PipeRunAndLog is similar to RunAndLog runs a command and writes the output
// to default logger and also writes in to the process stdin
func PipeRunAndLog(in string, args ...string) error {
	return run(func(cmd *exec.Cmd) error {
		stdin, err := cmd.StdinPipe()
		if err != nil {
			return err
		}

		go func() {
			defer func() {
				_ = stdin.Close()
			}()

			_, _ = io.WriteString(stdin, in)
		}()

		return nil
	}, runLogger{}, nil, args...)
}

func run(sw func(cmd *exec.Cmd) error, writer io.Writer, env map[string]string, args ...string) error {
	var exe string
	var cmdArgs []string

	log.Debug("%s", strings.Join(args, " "))

	exe = args[0]
	cmdArgs = args[1:]

	cmd := exec.Command(exe, cmdArgs...)

	if httpsProxy != "" {
		cmd.Env = append(os.Environ(), fmt.Sprintf("https_proxy=%s", httpsProxy))
	}

	if sw != nil {
		if err := sw(cmd); err != nil {
			return err
		}
	}

	cmd.Stdout = writer
	cmd.Stderr = writer
	if cmd.Stdin == nil {
		cmd.Stdin = os.Stdin
	}

	for k, v := range env {
		cmd.Args = append(cmd.Args, fmt.Sprintf("%s=%s", k, v))
	}

	err := cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

// Run executes a command and uses writer to write both stdout and stderr
// args are the actual command and its arguments
func Run(writer io.Writer, args ...string) error {
	return run(nil, writer, nil, args...)
}
