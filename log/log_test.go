// Copyright © 2019 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package log

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/clearlinux/clr-installer/errors"
	"github.com/clearlinux/clr-installer/utils"
)

func setLog(t *testing.T) *os.File {
	var handle *os.File

	tmpfile, err := ioutil.TempFile("", "writeLog")
	if err != nil {
		t.Fatalf("could not make tempfile: %v", err)
	}
	_ = tmpfile.Close()

	if handle, err = SetOutputFilename(tmpfile.Name()); err != nil {
		t.Fatal("Could not set Log file")
	}

	return handle
}

func readLog(t *testing.T) *bytes.Buffer {
	tmpfile, err := ioutil.TempFile("", "readLog")
	if err != nil {
		t.Fatalf("could not make tempfile: %v", err)
	}
	_ = tmpfile.Close()
	defer func() { _ = os.Remove(tmpfile.Name()) }() // clean up

	_ = ArchiveLogFile(tmpfile.Name())

	var contents []byte
	contents, err = ioutil.ReadFile(tmpfile.Name())
	if err != nil {
		t.Fatalf("could not read tempfile: %v %q", err, tmpfile.Name())
	} else {
		return bytes.NewBuffer(contents)
	}

	return nil
}

func TestTag(t *testing.T) {
	tests := []struct {
		msg string
		tag string
		fc  func(fmt string, args ...interface{})
	}{
		{"debug tag test", "[DBG]", Debug},
		{"info tag test", "[INF]", Info},
		{"warning tag test", "[WRN]", Warning},
		{"error tag test", "[ERR]", Error},
	}

	fh := setLog(t)
	defer func() {
		_ = fh.Close()
		_ = os.Remove(fh.Name())
	}()

	SetLogLevel(LogLevelDebug)

	for _, curr := range tests {
		curr.fc(curr.msg)

		str := readLog(t).String()
		if str == "" {
			t.Fatal("No log written to output")
		}

		if !strings.Contains(str, curr.tag) {
			t.Fatalf("Log generated an entry without the expected tag: %s - entry: %s",
				curr.tag, str)
		}
	}
}

func TestRepeat(t *testing.T) {
	fh := setLog(t)
	defer func() {
		_ = fh.Close()
		_ = os.Remove(fh.Name())
	}()

	SetLogLevel(LogLevelDebug)

	msg := "This is a log message"
	Debug(msg)
	Debug(msg)
	Debug(msg)
	Debug(msg)
	Debug("Different")

	str := readLog(t).String()
	if str == "" {
		t.Fatal("No log written to output")
	}

	if !strings.Contains(str, "repeated") {
		t.Fatalf("Log generated an entries without the expected repeated message")
	}
}

func TestErrorError(t *testing.T) {
	fh := setLog(t)
	defer func() {
		_ = fh.Close()
		_ = os.Remove(fh.Name())
	}()

	ErrorError(fmt.Errorf("testing log with error"))

	str := readLog(t).String()
	if str == "" {
		t.Fatal("No log written to output")
	}
}

func TestLogLevel(t *testing.T) {
	tests := []struct {
		mutedLevel int
		msg        string
		fc         func(fmt string, args ...interface{})
	}{
		{LogLevelInfo, "Debug() log with LogLevelInfo", Debug},
		{LogLevelWarning, "Info() with LogLevelWarning", Info},
		{LogLevelError, "Warning() with LogLevelError", Warning},
	}

	fh := setLog(t)
	defer func() {
		_ = fh.Close()
		_ = os.Remove(fh.Name())
	}()

	for _, curr := range tests {
		SetLogLevel(curr.mutedLevel)
		curr.fc(curr.msg)

		if readLog(t).String() != "" {
			t.Fatalf("Shouldn't produce any log with level: %d", curr.mutedLevel)
		}
	}
}

func TestGetLogFileNameStr(t *testing.T) {

	if GetLogFileName() == "" {
		t.Fatalf("GetLogFileName returned an empty string")
	}
}

func TestLogLevelStr(t *testing.T) {
	tests := []struct {
		level int
		str   string
	}{
		{LogLevelVerbose, "LogLevelVerbose"},
		{LogLevelDebug, "LogLevelDebug"},
		{LogLevelInfo, "LogLevelInfo"},
		{LogLevelWarning, "LogLevelWarning"},
		{LogLevelError, "LogLevelError"},
	}

	for _, curr := range tests {
		str, err := LevelStr(curr.level)
		if err != nil {
			t.Fatalf(fmt.Sprintf("%s", err))
		}

		if str != curr.str {
			t.Fatalf("Expected string %s, but got: %s", curr.str, str)
		}
	}
}

func TestInvalidLogLevelStr(t *testing.T) {
	_, err := LevelStr(-1)
	if err == nil {
		t.Fatal("Should have failed to format an invalid log level")
	}
}

func TestInvalidLogLevel(t *testing.T) {
	// Exercise the code paths for invalid, but it is not a fatal error
	SetLogLevel(-1)
	SetLogLevel(999)
}

func TestLogTraceableError(t *testing.T) {
	fh := setLog(t)
	defer func() {
		_ = fh.Close()
		_ = os.Remove(fh.Name())
	}()

	ErrorError(errors.Errorf("Traceable error"))

	if !strings.Contains(readLog(t).String(), "log_test.go") {
		t.Fatal("Traceable should contain the source name")
	}
}

func TestFailSeek(t *testing.T) {
	err := ArchiveLogFile("archivefile")
	if err == nil {
		t.Fatal("Should have failed, unseekable file")
	}
}

func TestNoFileHandle(t *testing.T) {
	prevHandle := filehandle
	filehandle = nil
	err := ArchiveLogFile("archivefile")
	if err == nil {
		t.Fatal("Should have failed, no output set")
	}

	filehandle = prevHandle
}

func TestFailedToArchiveUnwritableFile(t *testing.T) {
	dir, err := ioutil.TempDir("", "clr-installer-utest")
	if err != nil {
		t.Fatal(err)
	}

	rootDir := filepath.Join(dir, "root")
	if err = utils.MkdirAll(rootDir, 0755); err != nil {
		t.Fatal(err)
	}

	err = os.Chmod(rootDir, 0000)
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		err = os.Chmod(rootDir, 0700)
		if err != nil {
			t.Fatal(err)
		}
		_ = os.RemoveAll(dir)
	}()

	if err = ArchiveLogFile(filepath.Join(rootDir, "test.log")); err == nil {
		t.Fatal("Should have failed, no permission to archive file")
	}
}

func TestFailedToSetOutput(t *testing.T) {
	if utils.IsRoot() {
		t.Skip("Not running as 'root', skipping test")
	}

	dir, err := ioutil.TempDir("", "clr-installer-utest")
	if err != nil {
		t.Fatal(err)
	}

	rootDir := filepath.Join(dir, "root")
	if err = utils.MkdirAll(rootDir, 0755); err != nil {
		t.Fatal(err)
	}

	err = os.Chmod(rootDir, 0000)
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		err = os.Chmod(rootDir, 0700)
		if err != nil {
			t.Fatal(err)
		}
		_ = os.RemoveAll(dir)
	}()

	_, err = SetOutputFilename(filepath.Join(rootDir, "test.log"))
	if err == nil {
		t.Fatal("Should have failed to open log file")
	}
}

func TestGetPreConfFile(t *testing.T) {
	if GetPreConfFile() != preConfName {
		t.Fatal("log.GetPreConfFile() should always match log.preConfName")
	}
}

func TestRequestCrashInfo(t *testing.T) {
	RequestCrashInfo()
}
