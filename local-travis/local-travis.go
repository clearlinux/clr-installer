// Copyright © 2018 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strings"

	"github.com/clearlinux/clr-installer/cmd"

	flag "github.com/spf13/pflag"

	"gopkg.in/yaml.v2"
)

type travisConfig struct {
	Env           []string `yaml:"env,omitempty"`
	BeforeInstall []string `yaml:"before_install,omitempty"`
	BeforeScript  []string `yaml:"before_script,omitempty"`
	Script        []string `yaml:"script,omitempty"`
	AfterScript   []string `yaml:"after_script,omitempty"`
	envVars       [][]string
}

var (
	cmdExp        = regexp.MustCompile(`\$\((?P<cmd>.*)\)`)
	knownCommands = map[string]string{}
)

func (tc travisConfig) getEnvVar() [][]string {
	res := [][]string{}

	if tc.envVars != nil {
		return tc.envVars
	}

	for _, curr := range tc.Env {
		tks := strings.Split(curr, "=")
		if len(tks) <= 1 {
			continue
		}

		res = append(res, []string{tks[0], tks[1]})
	}

	tc.envVars = res
	return res
}

func parseKnownCommands(line string) error {
	match := cmdExp.FindStringSubmatch(line)

	if len(match) > 0 {
		for i := range cmdExp.SubexpNames() {
			if i == 0 {
				continue
			}

			cmdRepl := fmt.Sprintf("$(%s)", match[1])
			if _, ok := knownCommands[cmdRepl]; ok {
				continue
			}

			w := bytes.NewBuffer(nil)
			if err := cmd.Run(w, strings.Split(match[1], " ")...); err != nil {
				return err
			}

			knownCommands[cmdRepl] = strings.Replace(w.String(), "\n", "", -1)
		}
	}

	return nil
}

func replaceCommandAndVars(cfg travisConfig, line string) (string, error) {
	if err := parseKnownCommands(line); err != nil {
		return "", err
	}

	repStr := map[string]string{}

	// uses the line parsed expanded commands
	for k, v := range knownCommands {
		repStr[k] = v
	}

	// uses the travis declared env var
	for _, evar := range cfg.getEnvVar() {
		vars := []string{fmt.Sprintf("$%s", evar[0]), fmt.Sprintf("${%s}", evar[0])}

		for _, curr := range vars {
			repStr[strings.ToLower(curr)] = evar[1]
			repStr[strings.ToUpper(curr)] = evar[1]
		}
	}

	for k, v := range repStr {
		if !strings.Contains(line, k) {
			continue
		}

		line = strings.Replace(line, k, v, 1)
	}

	return line, nil
}

func splitArgs(cfg travisConfig, line string) ([]string, error) {
	var token string
	quoted := false
	res := []string{}

	line, err := replaceCommandAndVars(cfg, line)
	if err != nil {
		return nil, err
	}

	ll := len(line)

	addToken := func(tk string) error {
		if tk == "" {
			return nil
		}

		res = append(res, tk)
		return nil
	}

	for idx, ch := range line {
		if ch == ' ' && !quoted {
			if err := addToken(token); err != nil {
				return nil, err
			}

			token = ""
			continue
		}

		if ch == '"' {
			if !quoted {
				quoted = true
				continue
			} else {
				if err := addToken(token); err != nil {
					return nil, err
				}
				quoted = false
				token = ""
				continue
			}
		}

		token = fmt.Sprintf("%s%c", token, ch)

		if idx+1 == ll {
			if err := addToken(token); err != nil {
				return nil, err
			}
		}
	}

	return res, nil
}

func addEnvVarsToDocker(args []string) []string {
	setVars := map[string]string{}
	proxyVars := []string{"http_proxy", "https_proxy", "no_proxy", "UPDATE_COVERAGE"}

	for _, pvar := range proxyVars {
		value := os.Getenv(pvar)

		if value == "" {
			value = os.Getenv(strings.ToUpper(pvar))
		}

		if value != "" {
			setVars[pvar] = value
		}
	}

	if len(setVars) == 0 {
		return args
	}

	res := []string{args[0], args[1]}

	for k, v := range setVars {
		res = append(res, []string{"-e", fmt.Sprintf("%s=%s", k, v)}...)
	}

	res = append(res, args[2:]...)
	return res
}

func dockerCommandSupportsEnv(command string) bool {
	for _, curr := range []string{"run", "exec"} {
		if command == curr {
			return true
		}
	}

	return false
}

func runBatch(cfg travisConfig, batch []string) error {
	for _, line := range batch {
		args, err := splitArgs(cfg, line)
		if err != nil {
			return err
		}

		if args[0] == "travis_retry" {
			fmt.Printf("ignoring: [%s]\n", args[0])
			args = args[1:]
		}

		// travis_wait 30 sleep infinity &
		// Drop all args through &
		if args[0] == "travis_wait" {
			drop := []string{}
			for args[0] != "&" {
				drop = append(drop, args[0])
				args = args[1:]
			}

			drop = append(drop, args[0])
			args = args[1:]
			fmt.Printf("ignoring: [%s]\n", strings.Join(drop, " "))
		}

		fmt.Printf("running: [%s]\n", strings.Join(args, " "))

		if args[0] == "docker" && dockerCommandSupportsEnv(args[1]) {
			args = addEnvVarsToDocker(args)
		}

		if err := cmd.Run(os.Stdout, args...); err != nil {
			return err
		}
	}

	return nil
}

func main() {
	var cfg travisConfig
	var panicError error
	var travisFile string

	flag.StringVar(&travisFile, "config", "", "The travis configuration file")
	flag.ErrHelp = fmt.Errorf("Local Travis Execution")

	flag.Parse()

	if travisFile == "" {
		travisFile = os.Getenv("TRAVIS_CONF")

		if travisFile == "" {
			fmt.Println("ERROR: Could not find the travis config file, var $TRAVIS_CONF is not set")
			os.Exit(1)
		}
	}

	content, err := ioutil.ReadFile(travisFile)
	if err != nil {
		panic(err)
	}

	if yamlErr := yaml.UnmarshalStrict(content, &cfg); yamlErr != nil {
		panic(yamlErr)
	}

	cmds := [][]string{cfg.BeforeInstall, cfg.BeforeScript, cfg.Script}

	for _, curr := range cmds {
		if err := runBatch(cfg, curr); err != nil {
			panicError = err
			break
		}
	}

	if err := runBatch(cfg, cfg.AfterScript); err != nil {
		panic(err)
	}

	if panicError != nil {
		panic(panicError)
	}
}
