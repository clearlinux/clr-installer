// Copyright © 2020 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package cmd

import (
	"fmt"
	"os/exec"
	"testing"
)

func TestCracklibCheckExecutable(t *testing.T) {
	if _, err := exec.LookPath(crackLibPath); err != nil {
		fmt.Println("cracklib-check exe could not be found")
		t.Fail()
	}
}
