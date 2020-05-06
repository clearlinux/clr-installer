// Copyright © 2020 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package boolset

import (
	"testing"

	"gopkg.in/yaml.v2"
)

func TestSetValue(t *testing.T) {
	truth := New()

	// Should default to false, like a boolean
	if truth.Value() {
		t.Fatalf("Should have defaulted to false")
	}

	if truth.Default() != false {
		t.Fatalf("Should have default set to false")
	}

	// It should not be set by default
	if truth.IsSet() {
		t.Fatalf("Should not be set when only declared")
	}

	truth.SetValue(true)

	if !truth.IsSet() {
		t.Fatalf("Should be set from the previous assignment")
	}

	if !truth.Value() {
		t.Fatalf("Should have been set to true")
	}
}

func TestSetDefaultValue(t *testing.T) {
	truth := NewTrue()

	if !truth.Value() {
		t.Fatalf("Should have been set to true")
	}

	if truth.Default() != true {
		t.Fatalf("Should have default set to true")
	}

	if truth.IsSet() {
		t.Fatalf("Should not be set from the SetDefault")
	}

	if !truth.IsDefault() {
		t.Fatalf("Should be set to Default from the previous assignment")
	}

	truth.SetValue(true)

	if !truth.IsSet() {
		t.Fatalf("Should be set from the SetValue")
	}

	if !truth.IsDefault() {
		t.Fatalf("Should be set to Default from the previous assignment")
	}

	truth.SetDefault(false)
	truth.SetValue(true)

	if !truth.IsSet() {
		t.Fatalf("Should be set from the SetValue")
	}

	if truth.IsDefault() {
		t.Fatalf("Should NOT be set to Default from the SetDefault()")
	}
}

func TestSetDefaultFalseValue(t *testing.T) {
	truth := &BoolSet{}

	if truth.Value() {
		t.Fatalf("Should have been set to false")
	}

	if truth.Default() != false {
		t.Fatalf("Should have default set to false")
	}

	if truth.IsSet() {
		t.Fatalf("Should not be set from the declaration")
	}

	if !truth.IsDefault() {
		t.Fatalf("Should be set to Default from the declaration")
	}

	truth.SetValue(false)

	if !truth.IsSet() {
		t.Fatalf("Should be set from the SetValue")
	}

	if !truth.IsDefault() {
		t.Fatalf("Should be set to Default from the previous assignment")
	}
}

func TestCheckSet(t *testing.T) {
	truth := NewTrue()

	if truth.IsSet() {
		t.Fatalf("Should not be set from the NewTrue")
	}

	truth.SetValue(true)

	if !truth.IsSet() {
		t.Fatalf("Should be set from the SetValue")
	}

	truth.ClearSet()
	if truth.Value() != true {
		t.Fatalf("Should be set to true after ClearSet")
	}
	if truth.IsSet() {
		t.Fatalf("Should NOT be set from the ClearSet")
	}

	truth.SetValue(false)

	if truth.IsDefault() {
		t.Fatalf("Should NOT be set to default from SetValue(false) and NewTrue()")
	}

	if !truth.IsSet() {
		t.Fatalf("Should be set from the SetValue")
	}

	truth.ClearSetDefault(false)
	if truth.Value() != false {
		t.Fatalf("Should be set to true after ClearSetDefault")
	}
	if truth.IsSet() {
		t.Fatalf("Should NOT be set from the ClearSet")
	}

	if !truth.IsDefault() {
		t.Fatalf("Should be set to default from SetValue(false) and ClearSetDefault(false)")
	}
}

type SystemInstall struct {
	PostArchive *BoolSet `yaml:"postArchive,omitempty,flow"`
	Hostname    string   `yaml:"hostname,omitempty,flow"`
	AutoUpdate  *BoolSet `yaml:"autoUpdate,flow"`
}

func TestMarshalTrue(t *testing.T) {
	// Log a sanitized YAML file with Telemetry
	var copyTruth SystemInstall
	bothTrue := &SystemInstall{Hostname: "both-true"}
	bothTrue.AutoUpdate = NewTrue()
	bothTrue.PostArchive = NewTrue()

	// Marshal current into bytes
	confBytes, bytesErr := yaml.Marshal(bothTrue)
	if bytesErr != nil {
		t.Fatalf("Failed to Marshal YAML: %v", bytesErr)
	}

	t.Logf("bytes:\n%+v\n", string(confBytes))

	// Unmarshal into a copy
	if yamlErr := yaml.Unmarshal(confBytes, &copyTruth); yamlErr != nil {
		t.Fatalf("Failed to Unmarshal YAML: %v", yamlErr)
	}

	if copyTruth.AutoUpdate == nil {
		t.Fatalf("No AutoUpdate, but should have unmarshal")
	}

	if bothTrue.AutoUpdate.Default() == copyTruth.AutoUpdate.Default() {
		t.Fatalf("Default should not have been passed via Marshal/Unmarshal")
	}

	if bothTrue.AutoUpdate.Value() != copyTruth.AutoUpdate.Value() {
		t.Fatalf("Value should have been passed via Marshal/Unmarshal: %v != %v",
			bothTrue.AutoUpdate.Value(), copyTruth.AutoUpdate.Value())
	}

	if copyTruth.PostArchive != nil {
		t.Fatalf("Found PostArchive, but should not have unmarshal")
	}
}

func TestMarshalFalse(t *testing.T) {
	// Log a sanitized YAML file with Telemetry
	var copyTruth SystemInstall
	bothFalse := &SystemInstall{Hostname: "both-false"}
	bothFalse.AutoUpdate = New()
	bothFalse.PostArchive = New()
	bothFalse.PostArchive.SetValue(false)

	// Marshal current into bytes
	confBytes, bytesErr := yaml.Marshal(bothFalse)
	if bytesErr != nil {
		t.Fatalf("Failed to Marshal YAML: %v", bytesErr)
	}

	t.Logf("bytes:\n%+v\n", string(confBytes))

	// Unmarshal into a copy
	if yamlErr := yaml.Unmarshal(confBytes, &copyTruth); yamlErr != nil {
		t.Fatalf("Failed to Unmarshal YAML: %v", yamlErr)
	}

	if copyTruth.AutoUpdate == nil {
		t.Fatalf("No AutoUpdate, but should have unmarshal")
	}

	if bothFalse.AutoUpdate.Default() != copyTruth.AutoUpdate.Default() {
		t.Fatalf("Default should have been passed via Marshal/Unmarshal")
	}

	if bothFalse.AutoUpdate.Value() != copyTruth.AutoUpdate.Value() {
		t.Fatalf("Value should have been passed via Marshal/Unmarshal: %v != %v",
			bothFalse.AutoUpdate.Value(), copyTruth.AutoUpdate.Value())
	}

	if copyTruth.PostArchive == nil {
		t.Fatalf("No PostArchive, but should have unmarshal")
	}

	if bothFalse.PostArchive.Default() != copyTruth.PostArchive.Default() {
		t.Fatalf("Default should have been passed via Marshal/Unmarshal")
	}

	if bothFalse.PostArchive.Value() != copyTruth.PostArchive.Value() {
		t.Fatalf("Value should have been passed via Marshal/Unmarshal: %v != %v",
			bothFalse.PostArchive.Value(), copyTruth.PostArchive.Value())
	}
}
