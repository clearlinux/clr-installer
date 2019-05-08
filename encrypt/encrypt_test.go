// Copyright © 2019 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package encrypt

import (
	"testing"

	"github.com/GehirnInc/crypt"
	// package requires import the hash method to blank
	_ "github.com/GehirnInc/crypt/sha512_crypt"
)

func TestCrypt(t *testing.T) {
	crypt := crypt.SHA512.New()

	str := "a string to be hashed"

	hashed, hErr := Crypt(str)
	if hErr != nil {
		t.Fatalf("Should not fail to hash the string")
	}

	hashed2, h2Err := Crypt(str)
	if h2Err != nil {
		t.Fatalf("Should not fail to hash2 the string")
	}

	if hashed2 == hashed {
		t.Fatalf("The hashes should not be the same")
	}

	err := crypt.Verify(hashed, []byte(str))
	if err != nil {
		t.Fatalf("Failed to verify hashed")
	}

	err = crypt.Verify(hashed2, []byte(str))
	if err != nil {
		t.Fatalf("Failed to verify hashed2")
	}
}

func TestFailCrypt(t *testing.T) {
	crypt := crypt.SHA512.New()

	str := "a string to be hashed"
	str2 := "string to be hashed"

	hashed, hErr := Crypt(str)
	if hErr != nil {
		t.Fatalf("Should not fail to hash the string")
	}

	hashed2, h2Err := Crypt(str2)
	if h2Err != nil {
		t.Fatalf("Should not fail to hash2 the string")
	}

	err := crypt.Verify(hashed, []byte(str2))
	if err == nil {
		t.Fatalf("Should have Failed to verify hashed")
	}

	err = crypt.Verify(hashed2, []byte(str))
	if err == nil {
		t.Fatalf("Should have Failed to verify hashed2")
	}
}
