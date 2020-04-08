// Copyright © 2020 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package user

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"github.com/clearlinux/clr-installer/utils"
)

func init() {
	utils.SetLocale("en_US.UTF-8")
}

const (
	offset = 10
)

// generateRandom String is a helper function to generate random string with alphabets.
// The startwithchars are extra argument if provided will
// prepend those chars to your random string.
// Total length of random string = arg n + len(startwithchars)
func generateRandomString(n int, startwithchars string) string {
	characterset := strings.Split("abcdefghijklmnopqrstuvwxyz", "")
	var random strings.Builder
	random.WriteString(startwithchars)
	for n > 0 {
		random.WriteString(characterset[rand.Intn(25)])
		n--
	}

	return random.String()

}

// Mock passwords
var mockpasswords = []struct {
	login               string
	username            string
	password            string
	expectedreturnError string
	expectedreturn      bool
}{
	// bad passwords
	//Fails Minimum Length
	{"testlogin", "testusername", "a", "Password must be at least 8 characters long", false},

	//Fails Maximum Length
	{"testlogin", "testusername",
		generateRandomString(MaxPasswordLength, "9!"),
		"Password may be at most 255 characters long", false},

	//Fails cracklib dictionary word
	{"login", "username", "Remember3!", "Password is based on a dictionary word", false},

	//Fails cracklib reverse dictionary word
	{"testlogin", "testusername", "Rebmemer8!",
		"Password is based on a (reversed) dictionary word", false},

	//Fails cracklib simplistic/systematic
	{"testlogin", "testusername", "Abcdefl1234567!",
		"Password is too simplistic/systematic", false},

	//Fails cracklib enough DIFFERENT characters
	{"testlogin", "testusername", "Aaaaaaaaaaaa8!",
		"Password does not contain enough DIFFERENT characters", false},

	// good passwords
	{"testlogin", "testusername", "Mfgatcsc5!", "", true},
	{"testlogin", "testusername", "84A562548463!", "", true},
	{"", "testusername", "Chha$24411", "", true},
}

// Mock Usernames
var mockusernames = []struct {
	login               string
	username            string
	password            string
	expectedreturnError string
	expectedreturn      bool
}{
	//bad usernames
	{"", generateRandomString(MaxUsernameLength-offset, "9"), "",
		"Username must contain only numbers, letters, commas, - or _", false},
	{"", generateRandomString(MaxUsernameLength-offset, "_"), "",
		"Username must contain only numbers, letters, commas, - or _", false},
	{"", generateRandomString(MaxUsernameLength+1, ""), "",
		"UserName maximum length is 64", false},
	// good usernames
	{"", generateRandomString(MaxUsernameLength-offset, "a-_ ,'."), "", "", true},
	{"", generateRandomString(MaxUsernameLength, ""), "", "", true},
}

// Mock Logins
var mocklogins = []struct {
	login               string
	username            string
	password            string
	expectedreturnError string
	expectedreturn      bool
}{
	//bad logins
	{"", "", "", "Login is required", false},
	{generateRandomString(MaxLoginLength-offset, "9!"), "", "",
		"Login must contain only numbers, letters, -, . or _", false},
	{generateRandomString(MaxLoginLength+1, ""), "", "", "Login maximum length is 31", false},
	//good logins
	{generateRandomString(MaxLoginLength-offset, "a9-_."), "", "", "", true},
	{generateRandomString(MaxLoginLength, ""), "", "", "", true},
}

func TestPasswordValidation(t *testing.T) {

	// Created a helper function to mask some long userpasswords
	testsuffixfunc := func(testfeed string) string {
		if len(testfeed) > 25 {
			return "long_random_password"
		}
		return fmt.Sprintf("password_equal_%s", testfeed)
	}

	for _, curruser := range mockpasswords {
		t.Run(testsuffixfunc(curruser.password), func(t *testing.T) {
			observedreturn, observedstring := IsValidPassword(curruser.password)
			if curruser.expectedreturnError != observedstring || curruser.expectedreturn != observedreturn {
				t.Errorf("Test assertion failed: got (%v,%s), want (%v,%s)",
					observedreturn, observedstring, curruser.expectedreturn, curruser.expectedreturnError)
			}
		})
	}
}

func TestUsernameValidation(t *testing.T) {

	// Created a helper function to mask some long userpasswords
	testsuffixfunc := func(testfeed string) string {
		if len(testfeed) > 25 {
			return "long_random_username"
		}
		return fmt.Sprintf("username_equal_%s", testfeed)
	}

	for _, curruser := range mockusernames {
		t.Run(testsuffixfunc(curruser.username), func(t *testing.T) {
			observedreturn, observedstring := IsValidUsername(curruser.username)
			if curruser.expectedreturnError != observedstring || curruser.expectedreturn != observedreturn {
				t.Errorf("Test assertion failed: got (%v,%s), want (%v,%s)",
					observedreturn, observedstring, curruser.expectedreturn, curruser.expectedreturnError)
			}
		})
	}
}

func TestLoginValidation(t *testing.T) {

	// Created a helper function to mask some long userpasswords
	testsuffixfunc := func(testfeed string) string {
		if len(testfeed) > 31 {
			return "long_random_login"
		}
		return fmt.Sprintf("login_equal_%s", testfeed)
	}

	for _, curruser := range mocklogins {
		t.Run(testsuffixfunc(curruser.login), func(t *testing.T) {
			observedreturn, observedstring := IsValidLogin(curruser.login)
			if curruser.expectedreturnError != observedstring || curruser.expectedreturn != observedreturn {
				t.Errorf("Test assertion failed: got (%v,%s), want (%v,%s)",
					observedreturn, observedstring, curruser.expectedreturn, curruser.expectedreturnError)
			}
		})
	}
}
