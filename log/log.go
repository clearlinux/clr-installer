// Copyright © 2018 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package log

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/clearlinux/clr-installer/conf"
	"github.com/clearlinux/clr-installer/errors"
)

const (
	// LogLevelError specified the log level as: ERROR
	LogLevelError = 1

	// LogLevelWarning specified the log level as: WARNING
	LogLevelWarning = 2

	// LogLevelInfo specified the log level as: INFO
	LogLevelInfo = 3

	// LogLevelDebug specified the log level as: DEBUG
	LogLevelDebug = 4

	// LogLevelVerbose specified the log level as: VERBOSE
	// This is the same as Debug, but without the repeat filtering
	LogLevelVerbose = 5

	// configFilePreInstalPrefix is the prefix to create a configuration// file name
	configFilePreInstalPrefix = "pre-install-"
)

var (
	level      = LogLevelInfo
	levelMap   = map[int]string{}
	filehandle *os.File

	logFileName string
	preConfName string

	lineLast  string
	lineCount int
)

func init() {
	levelMap[LogLevelError] = "LogLevelError"
	levelMap[LogLevelWarning] = "LogLevelWarning"
	levelMap[LogLevelInfo] = "LogLevelInfo"
	levelMap[LogLevelDebug] = "LogLevelDebug"
	levelMap[LogLevelVerbose] = "LogLevelVerbose"
}

// SetLogLevel sets the default log level to l
func SetLogLevel(l int) {
	if l < LogLevelError {
		level = LogLevelError
		logTag("WRN", "Log Level '%d' too low, forcing to %s (%d)", l, levelMap[level], level)
	} else if l > LogLevelVerbose {
		level = LogLevelVerbose
		logTag("WRN", "Log Level '%d' too high, forcing to %s (%d)", l, levelMap[level], level)
	} else {
		level = l
		Debug("Log Level set to %s (%d)", levelMap[level], l)
	}
}

// SetOutputFilename ... sets the default log output to filename instead of stdout/stderr
func SetOutputFilename(logFile string) (*os.File, error) {
	logFileName = logFile
	var err error
	filehandle, err = os.OpenFile(logFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return nil, err
	}

	log.SetOutput(filehandle)

	preConfName = filepath.Join(filepath.Dir(logFileName), configFilePreInstalPrefix+conf.ConfigFile)

	return filehandle, nil
}

// RequestCrashInfo prints information for the user on how to properly report the
// crash of the installer and how to gather more information
func RequestCrashInfo() {
	fmt.Println("Please report this crash using GitHub Issues:")
	fmt.Println("\thttps://github.com/clearlinux/clr-installer/issues")

	fmt.Println("")

	fmt.Println("Include the following as attachments to enable diagnosis:")
	fmt.Printf("\t%s\n", preConfName)
	fmt.Printf("\t%s\n", logFileName)

	fmt.Println("")

	fmt.Println("If the problem persists, enabling additional logging will be helpful in")
	fmt.Println("diagnosing the issue. At the OS Boot screen, hit 'e' to edit the kernel")
	fmt.Println("command line. Add the following to the end of the line:")
	fmt.Println("\tclri.loglevel=4")

	fmt.Println("")
}

// GetPreConfFile ... returns the filename of where to store the pre-configuration file
func GetPreConfFile() string {
	return preConfName
}

// ArchiveLogFile copies the contents of the log to the given filename
func ArchiveLogFile(archiveFile string) error {
	if filehandle == nil {
		return errors.Errorf("Log output should be set, see log.SetOutputFilename()")
	}

	a, err := os.OpenFile(archiveFile, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return err
	}

	defer func() {
		_ = a.Close()

		// Jump back to the end of the log file
		_, _ = filehandle.Seek(0, 2)

	}()

	_ = filehandle.Sync()

	// Jump to the beginning of the file
	_, err = filehandle.Seek(0, 0)
	if err != nil {
		Error("Failed to seek log file (%v)", err)
	}

	var bytesCopied int64
	bytesCopied, err = io.Copy(a, filehandle)
	if err != nil {
		Error("Failed to archive log file (%v) %q", err, archiveFile)
	}
	Debug("Archived %d bytes to file %q", bytesCopied, archiveFile)
	_ = a.Sync()

	return err
}

// LevelStr converts level to its text equivalent, if level is invalid
// an error is returned
func LevelStr(level int) (string, error) {
	for k, v := range levelMap {
		if k == level {
			return v, nil
		}
	}

	return "", fmt.Errorf("Invalid log level: %d", level)
}

func logTag(tag string, format string, a ...interface{}) {
	f := fmt.Sprintf("[%s] %s\n", tag, format)
	output := fmt.Sprintf(f, a...)

	if level >= LogLevelVerbose {
		log.Printf(output)
		return
	}

	if output != lineLast {
		// output the previous repeated line
		if lineCount > 0 {
			plural := ""
			if lineCount > 1 {
				plural = "s"
			}

			repeat := fmt.Sprintf("[%s] [Previous line repeated %d time%s]\n", tag, lineCount, plural)
			log.Printf(repeat)
		}

		log.Printf(output)

		lineLast = output
		lineCount = 0
	} else { // Repeated line
		lineCount++
	}
}

// Debug prints a debug log entry with DBG tag
func Debug(format string, a ...interface{}) {
	if level < LogLevelDebug {
		return
	}

	logTag("DBG", format, a...)
}

// Error prints an error log entry with ERR tag
func Error(format string, a ...interface{}) {
	logTag("ERR", format, a...)
}

// ErrorError prints an error log entry with ERR tag, it takes an
// error instead of format and args, if a TraceableError is provided
// then we also include the trace information in the error message
func ErrorError(err error) {
	msg := err.Error()

	if e, ok := err.(errors.TraceableError); ok {
		msg = fmt.Sprintf("%s %s", e.Trace, e.What)
	}

	logTag("ERR", msg)
}

// Info prints an info log entry with INF tag
func Info(format string, a ...interface{}) {
	if level < LogLevelInfo {
		return
	}

	logTag("INF", format, a...)
}

// Warning prints an warning log entry with WRN tag
func Warning(format string, a ...interface{}) {
	if level < LogLevelWarning {
		return
	}

	logTag("WRN", format, a...)
}
