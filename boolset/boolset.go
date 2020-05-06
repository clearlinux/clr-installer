// Copyright © 2020 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package boolset

// BoolSet used to track if both a boolean's value
// and if it was explicitly set.
// Useful for items which default to True
type BoolSet struct {
	value    bool
	defValue bool
	set      bool
}

// New creates a new instance of BoolSet
func New() *BoolSet {
	return &BoolSet{}
}

// New creates a new instance of BoolSet
// with the value and default set to 'true'
func NewTrue() *BoolSet {
	return &BoolSet{value: true, defValue: true}
}

// Value returns the state Value
func (bs *BoolSet) Value() bool {
	return bs.value
}

// SetValue sets the Value passed and sets the Set value state
func (bs *BoolSet) SetValue(value bool) {
	bs.value = value
	bs.set = true
}

// Value returns the default state Value
func (bs *BoolSet) Default() bool {
	return bs.defValue
}

// SetDefault sets the Default Value passed
func (bs *BoolSet) SetDefault(value bool) {
	bs.defValue = value
}

// ClearSet sets the Value passed and clears the Set value state
func (bs *BoolSet) ClearSet() {
	bs.set = false
}

// ClearSetDefault sets the Default Value passed and clears the Set value state
func (bs *BoolSet) ClearSetDefault(value bool) {
	bs.defValue = value
	bs.set = false
}

// IsSet returns if we have explicitly set a value
func (bs *BoolSet) IsSet() bool {
	return bs.set
}

// IsDefault returns if Value is set to the current Default
func (bs *BoolSet) IsDefault() bool {
	return bs.value == bs.defValue
}

// MarshalYAML is the yaml Marshaller implementation
func (bs *BoolSet) MarshalYAML() (interface{}, error) {
	return bs.Value(), nil
}

// UnmarshalYAML is the yaml Unmarshaller for BoolSet
func (bs *BoolSet) UnmarshalYAML(unmarshal func(interface{}) error) error {
	if bs == nil {
		bs = New()
	}

	var boolValue bool

	if err := unmarshal(&boolValue); err != nil {
		return err
	}

	// Copy the unmarshaled data
	bs.SetValue(boolValue)

	return nil
}

// IsZero is the yaml check for zero value
func (bs *BoolSet) IsZero() bool {
	if bs.IsSet() {
		return false
	}
	return bs.IsDefault()
}
