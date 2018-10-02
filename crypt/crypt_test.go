// Copyright © 2018 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package crypt

import (
	"testing"
)

func TestCrypt(t *testing.T) {
	str := "a string to be hashed"

	hashed, err := Crypt(str)
	if err != nil {
		t.Fatalf("Should not fail to hash the string")
	}

	hashed2, err := Crypt(str)
	if err != nil {
		t.Fatalf("Should not fail to hash the string")
	}

	if hashed2 == hashed {
		t.Fatalf("The hashes should not be the same")
	}
}
