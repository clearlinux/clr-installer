// Copyright © 2019 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package language

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/clearlinux/clr-installer/cmd"
	"github.com/clearlinux/clr-installer/log"

	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
)

// Language represents a system language, containing the locale code and lang tag representation
type Language struct {
	Code        string
	Tag         language.Tag
	userDefined bool
}

const (
	// DefaultLanguage is the default language string
	DefaultLanguage = "en_US.UTF-8"

	// RequiredBundle the bundle needed to set language other than the default
	RequiredBundle = "glibc-locale"
)

// validLanguages stores the list of localized languages
var localizedLanguages = map[string]bool{ // TODO: Remove this when localization is applied for all languages
	"en_US.UTF-8": false,
	"zh_CN.UTF-8": false,
	"es_MX.UTF-8": false,
}

// validLanguages stores the list of all valid, known languages
var validLanguages []*Language

// displayLanguage is The default language to display all language value
var displayLanguage *display.Dictionary

func init() {
	displayLanguage = display.English
}

// SetSelectionLanguage changes the default display language
func SetSelectionLanguage(code string) {
	// The language package can show the language choices
	// in different languages, but it generic per language
	// aka no country specific values
	parts := strings.Split(code, "_")
	shortCode := strings.ToLower(parts[0])
	switch shortCode {
	case "de":
		displayLanguage = display.German
	case "en":
		displayLanguage = display.English
	case "es":
		displayLanguage = display.Spanish
	case "fr":
		displayLanguage = display.French
	case "ja":
		displayLanguage = display.Japanese
	case "pt":
		displayLanguage = display.Portuguese
	case "zh":
		displayLanguage = display.Chinese
	default:
		displayLanguage = display.English
	}
}

// IsUserDefined returns true if the configuration was interactively
// defined by the user
func (l *Language) IsUserDefined() bool {
	return l.userDefined
}

// GetConfValues converts a Language to display string which can
// be displayed as the configured value
func (l *Language) GetConfValues() (string, string) {
	saveDefault := displayLanguage
	SetSelectionLanguage(l.Code)

	en := displayLanguage.Tags()
	name := en.Name(l.Tag)

	displayLanguage = saveDefault

	return name, l.Code
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
	if tag, err := language.Parse(code); err != nil {
		l.Tag = tag
	}
	return nil
}

// Equals compares tow Language instances
func (l *Language) Equals(comp *Language) bool {
	if comp == nil {
		return false
	}

	return l.Code == comp.Code
}

func getLangName(tagName string) *Language {
	langDict := displayLanguage.Languages()
	var lang *Language

	if tag, err := language.Parse(tagName); err != nil {
		saveTag := tag

		simple := strings.Replace(tagName, ".UTF-8", "", -1)
		// In case the MustParse fails, recover and fallback
		defer func() {
			if r := recover(); r != nil {
				tag = saveTag
			}
		}()
		tag = language.MustParse(simple)

		langName := langDict.Name(tag)
		tagString := tag.String()

		if tagString != "" && tagString != "und" && langName != "" {
			lang = &Language{
				Code: tagName,
				Tag:  tag,
			}
		} else {
			log.Debug("Unable to use language locale '%s'", tagName)
		}
	} else {
		log.Debug("Unable to parse language locale '%s'", tagName)
	}

	return lang
}

// Load uses localectl to load the currently available locales/Languages
func Load() ([]*Language, error) {
	if validLanguages != nil {
		return validLanguages, nil
	}
	validLanguages = []*Language{}

	uniqLang := make(map[string]*Language)

	w := bytes.NewBuffer(nil)
	err := cmd.Run(w, "locale", "-a")
	if err != nil {
		return nil, err
	}

	tks := strings.Split(w.String(), "\n")
	for _, curr := range tks {
		if curr == "" {
			continue
		}
		if lang := getLangName(curr); lang != nil {
			uniqLang[curr] = lang
		}
	}

	// Create a sorted order list of keys
	sortedKeys := make([]string, 0, len(uniqLang))
	for k := range uniqLang {
		if _, found := localizedLanguages[k]; found { // TODO: Remove this when localization is applied for all languages
			sortedKeys = append(sortedKeys, k)
		}
	}
	sort.Strings(sortedKeys)

	for _, value := range sortedKeys {
		validLanguages = append(validLanguages, uniqLang[value])
	}

	return validLanguages, nil
}

// IsValidLanguage verifies if the given language is valid
func IsValidLanguage(l *Language) bool {
	var result = false
	allLanguages, err := Load()
	if err != nil {
		return result
	}

	for _, curr := range allLanguages {
		if curr.Equals(l) {
			result = true
		}
	}

	return result
}

// SetTargetLanguage creates a locale locale.conf on the target
func SetTargetLanguage(rootDir string, language string) error {

	targetLocaleFile := filepath.Join(rootDir, "/etc/locale.conf")

	filehandle, err := os.OpenFile(targetLocaleFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("Could not create locale file")
	}

	defer func() {
		_ = filehandle.Close()
	}()

	if _, err := filehandle.Write([]byte("LANG=" + language + "\n")); err != nil {
		return fmt.Errorf("Could not write keyboard file")
	}

	return nil
}
