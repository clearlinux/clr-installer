// Copyright © 2018 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package errors

import (
	"fmt"
	"strings"
	"testing"
)

func testErrorf(t *testing.T) {
	err := Errorf("traceable error")

	e, ok := err.(TraceableError)
	if !ok {
		t.Fatal("Errorf() should return a TraceableError")
	}

	if e.Trace == "" {
		t.Fatal("Traceable error should contain trace info")
	}

	if !strings.Contains(e.Error(), e.Trace) && strings.Contains(e.Error(), e.What) {
		t.Fatal("Error() should return the content of Trace and What member")
	}
}

func TestErrorf(t *testing.T) {
	testErrorf(t)
}

func TestWrapp(t *testing.T) {
	err := Wrap(fmt.Errorf("wrapper error"))

	e, ok := err.(TraceableError)
	if !ok {
		t.Fatal("Wrap() should return a TraceableError")
	}

	if e.Trace == "" {
		t.Fatal("Traceable error should contain trace info")
	}

	if !strings.Contains(e.Error(), e.Trace) && strings.Contains(e.Error(), e.What) {
		t.Fatal("Error() should return the content of Trace and What member")
	}
}

func TestValidationError(t *testing.T) {
	msg := "Validation error"
	ve := ValidationErrorf(msg)

	if ve.Error() != msg {
		t.Fatal("Wrong validation error message")
	}

	if !IsValidationError(ve) {
		t.Fatal("IsValidationError() should report true")
	}

	te := Errorf("A traceable error")
	if IsValidationError(te) {
		t.Fatal("IsValidationError() should return false for a TraceableError")
	}
}
