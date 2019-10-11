// Copyright © 2019 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package utils

import (
	"testing"
)

func init() {
	SetLocale("en_US.UTF-8")
}

func TestExamndVariables(t *testing.T) {
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
