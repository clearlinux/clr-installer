// Copyright © 2018 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package language

import (
	"bytes"
	"strings"

	"github.com/clearlinux/clr-installer/cmd"

	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
)

// Language represents a system language, containing the locale code and lang tag representation
type Language struct {
	Code        string
	Tag         language.Tag
	userDefined bool
}

// IsUserDefined returns true if the configuration was interactively
// defined by the user
func (l *Language) IsUserDefined() bool {
	return l.userDefined
}

// String converts a Language to string, namely it returns the tag's name - or the language desc
func (l *Language) String() string {
	return display.English.Tags().Name(l.Tag)
}

// MarshalYAML marshals Language into YAML format
func (l *Language) MarshalYAML() (interface{}, error) {
	return l.Code, nil
}

// UnmarshalYAML unmarshals Language from YAML format
func (l *Language) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var code string

	if err := unmarshal(&code); err != nil {
		return err
	}

	l.Code = code
	l.userDefined = false
	return nil
}

// Equals compares tow Language instances
func (l *Language) Equals(comp *Language) bool {
	if comp == nil {
		return false
	}

	return l.Code == comp.Code
}

// Load uses localectl to load the currently available locales/Languages
func Load() ([]*Language, error) {
	result := []*Language{}

	w := bytes.NewBuffer(nil)
	err := cmd.Run(w, "localectl", "list-locales", "--no-pager")
	if err != nil {
		return nil, err
	}

	tks := strings.Split(w.String(), "\n")
	for _, curr := range tks {
		if curr == "" {
			continue
		}

		code := strings.Replace(curr, ".UTF-8", "", 1)

		lang := &Language{
			Code: curr,
			Tag:  language.MustParse(code),
		}

		result = append(result, lang)
	}

	return result, nil
}
