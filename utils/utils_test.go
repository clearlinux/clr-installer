// Copyright © 2019 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package utils

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

// String for test information
const testString = "Lorem ipsum dolor sit amet, consectetur adipiscing elit"

func init() {
	SetLocale("en_US.UTF-8")
}

func TestExpandVariables(t *testing.T) {
	vars := make(map[string]string)

	vars["chrootDir"] = "/tmp/mydir"
	vars["ISCHOOT"] = "1"
	vars["HOME"] = "/root"

	text := "[[ ${ISCHOOT} -eq 0 ]] && chroot ${chrootDir} ...."
	correctResult := "[[ 1 -eq 0 ]] && chroot /tmp/mydir ...."

	expandResult := ExpandVariables(vars, text)

	if expandResult != correctResult {
		t.Fatalf("Expansion of two variables failed: %q != %q", expandResult, correctResult)
	}

	text = "[[ ${ISCHOOT} -eq 0 -o $ISCHOOT -eq 0 ]] && chroot $chrootDir ...."
	correctResult = "[[ 1 -eq 0 -o 1 -eq 0 ]] && chroot /tmp/mydir ...."

	expandResult = ExpandVariables(vars, text)

	if expandResult != correctResult {
		t.Fatalf("Expansion of with and without braces variables failed: %q != %q", expandResult, correctResult)
	}

	text = "$home ${Home} $HoME ...."
	correctResult = "$home ${Home} $HoME ...."
	incorrectResult := "/root /root /root ...."

	expandResult = ExpandVariables(vars, text)

	if expandResult != correctResult {
		t.Fatalf("Expansion should not have matched -- case sensitive: %q != %q", expandResult, correctResult)
	}

	if expandResult == incorrectResult {
		t.Fatalf("Expansion should have failed -- case sensitive: %q == %q", expandResult, incorrectResult)
	}
}

func TestCopyFile(t *testing.T) {
	// Create temp file, which we will copy
	fileSrc, err := ioutil.TempFile("", "test_copy_file")
	if err != nil {
		t.Errorf("Create temp file: %v", err)
	}

	// It doesn’t matter if there is an error or not
	defer func() {
		fileSrc.Close()
		os.Remove(fileSrc.Name())
	}()

	// Writing test information to file
	_, err = fileSrc.Write([]byte(testString))
	if err != nil {
		t.Errorf("Write text into temp file: %v", err)
	}

	pathDest := filepath.Join(
		filepath.Dir(fileSrc.Name()),
		"test_copy_file",
	)

	compare := func() error {
		return compareFiles(fileSrc.Name(), pathDest)
	}

	// In any case, delete the file, even if it has not been created
	defer os.Remove(pathDest)

	type args struct {
		src  string
		dest string
	}

	tests := []struct {
		name       string
		args       args
		wantErr    bool
		checkAfter func() error
	}{
		{name: "Copy without error", args: args{fileSrc.Name(), pathDest}, wantErr: false, checkAfter: compare},
		{name: "Copy with error", args: args{"", ""}, wantErr: true, checkAfter: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := CopyFile(tt.args.src, tt.args.dest); (err != nil) != tt.wantErr {
				t.Errorf("CopyFile() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.checkAfter != nil {
				err := tt.checkAfter()
				if err != nil {
					t.Error(err)
				}
			}
		})
	}
}

func compareFiles(pathSrc, pathDest string) error {
	fileSrc, err := os.Open(pathSrc)
	if err != nil {
		return fmt.Errorf("Open src file %v", err)
	}

	fileDest, err := os.Open(pathDest)
	if err != nil {
		return fmt.Errorf("Open dest file %v", err)
	}

	statDest, err := fileDest.Stat()
	if err != nil {
		return fmt.Errorf("Get stat dest %v", err)
	}

	statSrc, err := fileSrc.Stat()
	if err != nil {
		return fmt.Errorf("Get stat src %v", err)
	}

	if statDest.Mode() != statSrc.Mode() {
		return errors.New("Mode files not equal")
	}

	destData, err := ioutil.ReadAll(fileDest)
	if err != nil {
		return fmt.Errorf("Read all file desst %v", err)
	}

	if string(destData) != testString {
		return errors.New("Data files not equal")
	}

	return nil
}

func TestVersion(t *testing.T) {
	versionString := VersionUintString(0)
	if !IsLatestVersion(versionString) {
		t.Fatalf("Version 0 should always be latest")
	}

	if num, err := VersionStringUint(versionString); err != nil {
		t.Fatalf("Parse Error: Version latest should always be 0")
	} else if num != 0 {
		t.Fatalf("Version latest should always be 0")
	} else {
		t.Logf("Found version %d for '%s'", num, versionString)
	}

	versionString = ""
	if num, err := VersionStringUint(versionString); err != nil {
		t.Fatalf("Parse Error: Version '' should always be 0")
	} else if num != 0 {
		t.Fatalf("Version '' should always be 0")
	} else {
		t.Logf("Found version %d for '%s'", num, versionString)
	}

	versionString = "0"
	if num, err := VersionStringUint(versionString); err != nil {
		t.Fatalf("Parse Error: Version '0' should always be 0")
	} else if num != 0 {
		t.Fatalf("Version '0' should always be 0")
	} else {
		t.Logf("Found version %d for '%s'", num, versionString)
	}

	versionString = "10"
	if num, err := VersionStringUint(versionString); err != nil {
		t.Fatalf("Parse Error: Version '10' should always be 10")
	} else if num != 10 {
		t.Fatalf("Version '10' should always be 10")
	} else {
		t.Logf("Found version %d for '%s'", num, versionString)
	}
}
