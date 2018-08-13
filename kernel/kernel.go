// Copyright © 2018 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package kernel

import (
	"encoding/json"
	"io/ioutil"

	"github.com/clearlinux/clr-installer/conf"
	"github.com/clearlinux/clr-installer/errors"
)

// Kernel describes a linux kernel to be installed
type Kernel struct {
	Bundle      string // Bundle is the bundle name containing this kernel
	Name        string // Name the bundle name for a given kernel
	Desc        string // Desc is the kernel description
	userDefined bool
}

// LoadKernelList loads the kernel definitions
func LoadKernelList() ([]*Kernel, error) {
	path, err := conf.LookupKernelListFile()
	if err != nil {
		return nil, err
	}

	root := struct {
		Kernels []*Kernel `json:"kernels"`
	}{}

	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, errors.Wrap(err)
	}

	if err = json.Unmarshal(data, &root); err != nil {
		return nil, errors.Wrap(err)
	}

	return root.Kernels, nil
}

// IsUserDefined returns true if the configuration was interactively
// defined by the user
func (k *Kernel) IsUserDefined() bool {
	return k.userDefined
}

// MarshalYAML marshals Kernel into YAML format
func (k *Kernel) MarshalYAML() (interface{}, error) {
	return k.Bundle, nil
}

// UnmarshalYAML unmarshals Kernel from YAML format
func (k *Kernel) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var bundle string

	if err := unmarshal(&bundle); err != nil {
		return err
	}

	k.Bundle = bundle
	k.userDefined = false
	return nil
}

// Equals compares tow Kernels instances
func (k *Kernel) Equals(comp *Kernel) bool {
	if comp == nil {
		return false
	}

	return k.Bundle == comp.Bundle
}
