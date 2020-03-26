// Copyright © 2020 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package cmd

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/clearlinux/clr-installer/log"
	"github.com/clearlinux/clr-installer/proxy"
)

// Output interface allows implementors to process the output from a
// command according to their specific case
type Output interface {
	Process(printPrefix, line string)
}

type runLogger struct{}

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

// PipeRunAndPipeOut is similar to PipeRunAndLog but runs a command by feeding
// a string to stdin of Cmd and output is written to a byte buffer instead of a log
func PipeRunAndPipeOut(in string, out *bytes.Buffer, args ...string) error {
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
	}, out, nil, args...)
}

func run(sw func(cmd *exec.Cmd) error, writer io.Writer, env map[string]string, args ...string) error {
	var exe string
	var cmdArgs []string

	log.Debug("%s", strings.Join(args, " "))

	exe = args[0]
	cmdArgs = args[1:]

	cmd := exec.Command(exe, cmdArgs...)

	// Add any proxy environment variables
	for _, pvar := range proxy.GetProxyValues() {
		cmd.Env = append(cmd.Env, pvar)
	}
	log.Debug("cmd.Env: %+v", cmd.Env)

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
		curr := fmt.Sprintf("%s=%s", k, v)
		cmd.Args = append(cmd.Args, curr)
		cmd.Env = append(cmd.Env, curr)
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

// RunAndProcessOutput executes a command and process the output from
// Stdout and Stderr according to the implementor
// args are the actual command and its arguments
func RunAndProcessOutput(printPrefix string, output Output, args ...string) error {

	var exe string
	var cmdArgs []string

	log.Debug(strings.Join(args, " "))

	exe = args[0]
	cmdArgs = args[1:]

	cmd := exec.Command(exe, cmdArgs...)

	// Add any proxy environment variables
	for _, pvar := range proxy.GetProxyValues() {
		cmd.Env = append(cmd.Env, pvar)
	}
	log.Debug("cmd.Env: %+v", cmd.Env)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Error("Could not connect a pipe to Stdout")
		return err
	}

	// run the command but don't wait for it to finish
	if err := cmd.Start(); err != nil {
		log.Error("Failed to start command execution: %s", exe)
		return err
	}

	// start scanning stdout for messages
	scannerOut := bufio.NewScanner(stdout)
	for scannerOut.Scan() {

		// specific processing implementation
		output.Process(printPrefix, scannerOut.Text())

	}

	if err := scannerOut.Err(); err != nil {
		log.Error("An error occurred while reading stdout")
		return err
	}

	// wait for the command to finish running
	if err := cmd.Wait(); err != nil {
		log.Error("An error occurred executing command: \"%s\". Error: %s", strings.Join(args, " "), err)
		return err
	}

	return nil
}
