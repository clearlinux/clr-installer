// Copyright © 2019 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package utils

import (
	"github.com/gotk3/gotk3/gtk"
)

// SetBox sets up a new gtk Box
func SetBox(orient gtk.Orientation, spacing int, borderWidth uint) (*gtk.Box, error) {
	box, err := gtk.BoxNew(orient, spacing)
	if err != nil {
		return nil, err
	}
	box.SetBorderWidth(borderWidth)
	return box, nil
}

// GetBufferFromEntry gets the buffer from a gtk Entry
func GetBufferFromEntry(entry *gtk.Entry) (*gtk.EntryBuffer, error) {
	buffer, err := entry.GetBuffer()
	if err != nil {
		return nil, err
	}
	return buffer, nil
}

// GetTextFromEntry reads the text from an Entry buffer
func GetTextFromEntry(entry *gtk.Entry) (string, error) {
	buffer, err := GetBufferFromEntry(entry)
	if err != nil {
		return "", err
	}
	text, err := buffer.GetText()
	if err != nil {
		return "", err
	}
	return text, nil
}

// SetTextInEntry writes the text to an Entry buffer
func SetTextInEntry(entry *gtk.Entry, text string) error {
	buffer, err := GetBufferFromEntry(entry)
	if err != nil {
		return err
	}
	buffer.SetText(text)
	return nil
}
