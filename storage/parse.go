// Copyright © 2020 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package storage

import (
	"bytes"
	"encoding/json"
	"io"
	"strconv"

	"github.com/clearlinux/clr-installer/errors"
)

// Version used for reading and writing YAML
type blockDeviceYAMLMarshal struct {
	Name            string         `yaml:"name,omitempty"`
	Model           string         `yaml:"model,omitempty"`
	MajorMinor      string         `yaml:"majMin,omitempty"`
	FsType          string         `yaml:"fstype,omitempty"`
	UUID            string         `yaml:"uuid,omitempty"`
	Serial          string         `yaml:"serial,omitempty"`
	MountPoint      string         `yaml:"mountpoint,omitempty"`
	Label           string         `yaml:"label,omitempty"`
	Size            string         `yaml:"size,omitempty"`
	ReadOnly        string         `yaml:"ro,omitempty"`
	RemovableDevice string         `yaml:"rm,omitempty"`
	Type            string         `yaml:"type,omitempty"`
	State           string         `yaml:"state,omitempty"`
	Children        []*BlockDevice `yaml:"children,omitempty"`
	Options         string         `yaml:"options,omitempty"`
}

// UnmarshalJSON decodes a BlockDevice, targeted to integrate with json
// decoding framework
// nolint: gocyclo
func (bd *BlockDevice) UnmarshalJSON(b []byte) error {

	dec := json.NewDecoder(bytes.NewReader(b))

	for {
		t, err := dec.Token()
		if err == io.EOF {
			break
		}

		str, valid := t.(string)
		if !valid {
			continue
		}

		switch str {
		case "name":
			var name string

			if name, err = getNextStrToken(dec, "name"); err != nil {
				return err
			}

			bd.Name = name
		case "model":
			var model string

			if model, err = getNextStrToken(dec, "model"); err != nil {
				return err
			}

			bd.Model = model
		case "maj:min":
			var majMin string

			if majMin, err = getNextStrToken(dec, "maj:min"); err != nil {
				return err
			}

			bd.MajorMinor = majMin
		case "size":
			var size uint64

			if size, err = getNextByteToken(dec, "size"); err != nil {
				return err
			}

			bd.Size = size
		case "pttype":
			var pttype string

			if pttype, err = getNextStrToken(dec, "pttype"); err != nil {
				return err
			}

			bd.PtType = pttype
		case "fstype":
			var fstype string

			if fstype, err = getNextStrToken(dec, "fstype"); err != nil {
				return err
			}

			bd.FsType = fstype
		case "uuid":
			var uuid string

			if uuid, err = getNextStrToken(dec, "uuid"); err != nil {
				return err
			}

			bd.UUID = uuid
		case "serial":
			var serial string

			if serial, err = getNextStrToken(dec, "serial"); err != nil {
				return err
			}

			bd.Serial = serial
		case "type":
			var tp string

			tp, err = getNextStrToken(dec, "type")
			if err != nil {
				return err
			}

			bd.Type, err = parseBlockDeviceType(tp)
			if err != nil {
				return err
			}
		case "state":
			var state string

			state, err = getNextStrToken(dec, "state")
			if err != nil {
				return err
			}

			bd.State, err = parseBlockDeviceState(state)
			if err != nil {
				return err
			}
		case "mountpoint":
			var mpoint string

			if mpoint, err = getNextStrToken(dec, "mountpoint"); err != nil {
				return err
			}

			bd.MountPoint = mpoint
		case "label":
			var label string

			if label, err = getNextStrToken(dec, "label"); err != nil {
				return err
			}

			bd.Label = label
		case "partlabel":
			var label string

			if label, err = getNextStrToken(dec, "partlabel"); err != nil {
				return err
			}

			bd.PartitionLabel = label
		case "ro":
			if bd.ReadOnly, err = getNextBoolToken(dec, "ro"); err != nil {
				return err
			}
		case "rm":
			if bd.RemovableDevice, err = getNextBoolToken(dec, "rm"); err != nil {
				return err
			}
		case "children":
			bd.Children = []*BlockDevice{}
			if err := dec.Decode(&bd.Children); err != nil {
				return errors.Errorf("Invalid \"children\" token: %s", err)
			}
		}
	}

	return nil
}

func getNextStrToken(dec *json.Decoder, name string) (string, error) {
	t, _ := dec.Token()
	if t == nil {
		return "", nil
	}

	str, valid := t.(string)
	if !valid {
		return "", errors.Errorf("\"%s\" token should have a string value", name)
	}

	return str, nil
}

func getNextByteToken(dec *json.Decoder, name string) (uint64, error) {
	var byteSize uint64
	var err error

	dec.UseNumber()
	token, _ := dec.Token()
	if token == nil {
		return 0, nil
	}

	switch t := token.(type) {
	case json.Number:
		// Is it an unsigned int value (lsblk >= 2.33)
		var n int64

		n, err = t.Int64()
		if err != nil {
			return 0, err
		}

		byteSize = uint64(n)

	case string:
		// Is it a string value (lsblk < 2.33)

		str, sValid := token.(string)
		if !sValid {
			return 0, errors.Errorf("\"%s\" token is neither an uint64 nor a string value", name)
		}

		byteSize, err = ParseVolumeSize(str)
		if err != nil {
			return 0, err
		}
	}

	return byteSize, nil
}

func getNextBoolToken(dec *json.Decoder, name string) (bool, error) {
	t, _ := dec.Token()
	if t == nil {
		return false, nil
	}

	// Is it a boolean value (lsblk >= 2.33)
	b, bValid := t.(bool)
	if bValid {
		return b, nil
	}

	// Is it a string value (lsblk < 2.33)
	str, sValid := t.(string)
	if !sValid {
		return false, errors.Errorf("\"%s\" token is neither a boolean nor a string value", name)
	}

	if str == "0" {
		return false, nil
	} else if str == "1" {
		return true, nil
	} else if str == "" {
		return false, nil
	}

	return false, errors.Errorf("Unknown ro value: %s", str)
}

// MarshalYAML is the yaml Marshaller implementation
func (bd *BlockDevice) MarshalYAML() (interface{}, error) {

	var bdm blockDeviceYAMLMarshal

	bdm.Name = bd.Name
	bdm.Model = bd.Model
	bdm.MajorMinor = bd.MajorMinor
	bdm.FsType = bd.FsType
	bdm.UUID = bd.UUID
	bdm.Serial = bd.Serial
	bdm.MountPoint = bd.MountPoint
	bdm.Label = bd.Label
	bdm.Size = strconv.FormatUint(bd.Size, 10)
	bdm.ReadOnly = strconv.FormatBool(bd.ReadOnly)
	bdm.RemovableDevice = strconv.FormatBool(bd.RemovableDevice)
	bdm.Type = bd.Type.String()
	bdm.State = bd.State.String()
	bdm.Children = bd.Children
	bdm.Options = bd.Options

	return bdm, nil
}

// UnmarshalYAML is the yaml Unmarshaller implementation
func (bd *BlockDevice) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var unmarshBlockDevice blockDeviceYAMLMarshal

	if err := unmarshal(&unmarshBlockDevice); err != nil {
		return err
	}

	// Copy the unmarshaled data
	bd.Name = unmarshBlockDevice.Name
	bd.Model = unmarshBlockDevice.Model
	bd.MajorMinor = unmarshBlockDevice.MajorMinor
	bd.FsType = unmarshBlockDevice.FsType
	bd.UUID = unmarshBlockDevice.UUID
	bd.Serial = unmarshBlockDevice.Serial
	bd.MountPoint = unmarshBlockDevice.MountPoint
	bd.Label = unmarshBlockDevice.Label
	bd.Children = unmarshBlockDevice.Children
	bd.Options = unmarshBlockDevice.Options
	// Convert String to Uint64
	if unmarshBlockDevice.Size != "" {
		uSize, err := ParseVolumeSize(unmarshBlockDevice.Size)
		if err != nil {
			return err
		}
		bd.Size = uSize
	}

	// Map the BlockDeviceType
	if unmarshBlockDevice.Type != "" {
		iType, err := parseBlockDeviceType(unmarshBlockDevice.Type)
		if err != nil {
			return errors.Errorf("Device: %s: %v", unmarshBlockDevice.Name, err)
		}
		if iType < 0 || iType > BlockDeviceTypeUnknown {
		}
		bd.Type = iType
		if iType != BlockDeviceTypeDisk {
			bd.MakePartition = true
			bd.FormatPartition = true
		}
	}

	// Map the BlockDeviceState
	if unmarshBlockDevice.State != "" {
		iState, err := parseBlockDeviceState(unmarshBlockDevice.State)
		if err != nil {
			return errors.Errorf("Device: %s: %v", unmarshBlockDevice.Name, err)
		}
		bd.State = iState
	}

	// Map the ReanOnly bool
	if unmarshBlockDevice.ReadOnly != "" {
		bReadOnly, err := strconv.ParseBool(unmarshBlockDevice.ReadOnly)
		if err != nil {
			return err
		}
		bd.ReadOnly = bReadOnly
	}

	// Map the RemovableDevice bool
	if unmarshBlockDevice.RemovableDevice != "" {
		bRemovableDevice, err := strconv.ParseBool(unmarshBlockDevice.RemovableDevice)
		if err != nil {
			return err
		}
		bd.RemovableDevice = bRemovableDevice
	}

	return nil
}
