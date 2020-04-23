// Copyright © 2020 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package user

import (
	"fmt"
	"regexp"

	"github.com/clearlinux/clr-installer/cmd"
	"github.com/clearlinux/clr-installer/utils"
)

// Validator abstracts user validation efforts
type Validator struct {
	Login    string
	UserName string
	Password string
}

const (
	// MaxUsernameLength is the longest possible username
	MaxUsernameLength = 64
	// MaxLoginLength is the longest possible login
	MaxLoginLength = 31
	// MinPasswordLength is the shortest possible password
	MinPasswordLength = 8
	// MaxPasswordLength is the shortest possible password
	MaxPasswordLength = 255

	// UsernameCharRequirementMessage is basic username requirements
	UsernameCharRequirementMessage = "Username must contain only numbers, letters, commas, - or _"

	// UsernameMaxRequirementMessage is the basic username requirements
	UsernameMaxRequirementMessage = "UserName maximum length is %d"

	// LoginNonEmptyRequirementMessage is basic login requirements
	LoginNonEmptyRequirementMessage = "Login is required"

	// LoginMaxRequirementMessage is the basic login requirements
	LoginMaxRequirementMessage = "Login maximum length is %d"

	// LoginRegexRequirementMessage is the basic password requirements
	LoginRegexRequirementMessage = "Login must contain only numbers, letters, -, . or _"

	// PasswordMinRequirementMessage is the basic password requirements
	PasswordMinRequirementMessage = "Password must be at least %d characters long"

	// PasswordMaxRequirementMessage is the basic password requirements
	PasswordMaxRequirementMessage = "Password may be at most %d characters long"
)

var (
	usernameExp = regexp.MustCompile("^([a-zA-Z]+[0-9a-zA-Z-_ ,'.]*|)$")
	loginExp    = regexp.MustCompile("^[a-zA-Z]+[0-9a-zA-Z-_.]*$")
)

// NewValidator creates/allocates a new user validation
func NewValidator(login string,
	username string, password string) *Validator {
	return &Validator{
		Login:    login,
		UserName: username,
		Password: password,
	}
}

// loginEmptyCheck checks if to see if login is non-empty
func (uservalidator *Validator) loginEmptyCheck() error {
	if uservalidator.Login == "" {
		return fmt.Errorf(utils.Locale.Get(LoginNonEmptyRequirementMessage))
	}

	return nil
}

// loginMaxLengthCheck checks if login is less than or equal to MaxLoginLength
func (uservalidator *Validator) loginMaxLengthCheck() error {
	if len(uservalidator.Login) > MaxLoginLength {
		return fmt.Errorf(utils.Locale.Get(LoginMaxRequirementMessage, MaxLoginLength))
	}

	return nil
}

// loginRegexCheck checks if login meets regular expression loginExp
func (uservalidator *Validator) loginRegexCheck() error {
	if !loginExp.MatchString(uservalidator.Login) {
		return fmt.Errorf(utils.Locale.Get(LoginRegexRequirementMessage))
	}

	return nil
}

// usernameRegexCheck checks if username meets regular expression usernameExp
func (uservalidator *Validator) usernameRegexCheck() error {
	if !usernameExp.MatchString(uservalidator.UserName) {
		return fmt.Errorf(utils.Locale.Get(UsernameCharRequirementMessage))
	}

	return nil
}

// usernameMaxLengthCheck checks if the username is less than or equal to MaxUsernameLength
func (uservalidator *Validator) usernameMaxLengthCheck() error {
	if len(uservalidator.UserName) > MaxUsernameLength {
		return fmt.Errorf(utils.Locale.Get(UsernameMaxRequirementMessage, MaxUsernameLength))
	}

	return nil
}

// passwordMaximumLengthCheck checks if password has a maximum length of MaxPasswordLength
func (uservalidator *Validator) passwordMaximumLengthCheck() error {
	if len(uservalidator.Password) > MaxPasswordLength {
		return fmt.Errorf(utils.Locale.Get(PasswordMaxRequirementMessage, MaxPasswordLength))
	}

	return nil
}

// passwordMinimumLengthCheck checks if password has a minimum length of MinPasswordLength
func (uservalidator *Validator) passwordMinimumLengthCheck() error {
	if len(uservalidator.Password) < MinPasswordLength {
		return fmt.Errorf(utils.Locale.Get(PasswordMinRequirementMessage, MinPasswordLength))
	}

	return nil
}

// passwordCracklibCheck runs the cracklib-check executable piping password to stdin of cmd
// and writing stdoutput to byte buffer
func (uservalidator *Validator) passwordCracklibCheck() error {
	if status, errstring := cmd.CracklibCheck(uservalidator.Password, "Password"); !status {
		return fmt.Errorf(errstring)
	}

	return nil
}
