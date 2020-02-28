// Copyright © 2020 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package cmd

import (
	"bytes"
	"strings"

	"github.com/clearlinux/clr-installer/log"
	"github.com/clearlinux/clr-installer/utils"
)

// Path to the cracklib-check exe
const crackLibPath = "/usr/bin/cracklib-check"

// CracklibCheck runs the cracklib-check executable piping password to stdin of cmd
// and writing stdoutput to byte buffer
// stringtype is used to inform kind of information we are checking: password or passphrase
func CracklibCheck(texttoinpsect string, stringtype string) (bool, string) {

	defaultprefix := "Password"
	if stringtype != "" {
		defaultprefix = stringtype
	}

	cmdArgs := []string{crackLibPath}
	var out bytes.Buffer

	if err := PipeRunAndPipeOut(texttoinpsect, &out, cmdArgs...); err != nil {
		log.Error("Error running cracklib-check, %q", err)
		log.Error("Cracklib-check check will be skipped")
		return true, ""
	}

	stringout := string(out.Bytes())
	if index := strings.Index(stringout, ":"); index > -1 {
		parsedString := strings.Trim(stringout[index+1:], " \n")
		if strings.ToUpper(parsedString) != "OK" {
			parsedString = strings.Replace(parsedString, "it", defaultprefix, 1)
			return false, utils.Locale.Get(parsedString)
		}
	}

	return true, ""
}
