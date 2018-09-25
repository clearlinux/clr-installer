// Copyright © 2018 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package storage

import (
	"bytes"
	"testing"
	"text/template"
)

func TestGetConfiguredStatus(t *testing.T) {
	children := make([]*BlockDevice, 0)
	bd := &BlockDevice{Name: "sda", Children: children}
	expected := ConfiguredNone

	df := bd.GetConfiguredStatus()
	if df != expected {
		t.Fatalf("GetConfiguredStatus() returned returned: %d, expected: %d",
			df, expected)
	}

	part1 := &BlockDevice{FsType: "vfat", MountPoint: "/boot"}
	part2 := &BlockDevice{FsType: "swap", MountPoint: ""}
	part3 := &BlockDevice{FsType: "ext4", MountPoint: "/"}

	children = append(children, part1)
	bd = nil
	bd = &BlockDevice{Name: "sda", Children: children}
	expected = ConfiguredPartial

	df = bd.GetConfiguredStatus()
	if df != expected {
		t.Fatalf("GetConfiguredStatus() returned returned: %d, expected: %d",
			df, expected)
	}

	children = append(children, part2)
	bd = nil
	bd = &BlockDevice{Name: "sda", Children: children}
	expected = ConfiguredPartial

	df = bd.GetConfiguredStatus()
	if df != expected {
		t.Fatalf("GetConfiguredStatus() returned returned: %d, expected: %d",
			df, expected)
	}

	children = append(children, part3)
	bd = nil
	bd = &BlockDevice{Name: "sda", Children: children}
	expected = ConfiguredFull

	df = bd.GetConfiguredStatus()
	if df != expected {
		t.Fatalf("GetConfiguredStatus() returned returned: %d, expected: %d",
			df, expected)
	}
}

func TestGetDeviceFile(t *testing.T) {
	bd := &BlockDevice{Name: "sda"}
	expected := "/dev/sda"

	df := bd.GetDeviceFile()
	if df != expected {
		t.Fatalf("GetDeviceFile() returned wrong device file, returned: %s, expected: %s",
			df, expected)
	}
}

func TestSupportedFileSystem(t *testing.T) {
	expected := []string{"btrfs", "ext2", "ext3", "ext4", "swap", "vfat", "xfs"}
	supported := SupportedFileSystems()
	tot := 0

	if len(expected) != len(supported) {
		t.Fatal("supported file system list don't match the expected")
	}

	for _, value := range supported {
		for _, curr := range expected {
			if curr == value {
				tot = tot + 1
			}
		}
	}

	if tot != len(expected) {
		t.Fatal("supported file system list don't match the expected")
	}
}

func TestFailListBlockDevices(t *testing.T) {
	lsblkBinary = "lsblkX"

	_, err := ListBlockDevices(nil)
	if err == nil {
		t.Fatalf("Should have failed to list block devices")
	}

	lsblkBinary = "lsblk"
}

func TestEmptyBlockDevicesDescriptor(t *testing.T) {
	_, err := parseBlockDevicesDescriptor([]byte(""))
	if err == nil {
		t.Fatalf("Should have failed to parse invalid descriptor")
	}
}

func TestInvalidValues(t *testing.T) {
	templateStr := `{
    "blockdevices": [
        {
           {{.Value}}
        }
    ]
}`

	tests := []struct {
		name  string
		Value string
	}{
		{"children", `"children": "invalid"`},
		{"fstype", `"fstype": []`},
		{"maj:min", `"maj:min": []`},
		{"mountpoint", `"mountpoint": []`},
		{"removable", `"rm": "3"`},
		{"removable", `"rm": []`},
		{"ro", `"ro": "3"`},
		{"ro", `"ro": []`},
		{"size", `"size": "str"`},
		{"size", `"size": []`},
		{"type", `"type": "invalid"`},
		{"type", `"type": []`},
		{"uuid", `"uuid": []`},
	}

	tmpl, err := template.New("").Parse(templateStr)
	if err != nil {
		t.Fatalf("Failed to parse template: %s", err)
	}

	for _, curr := range tests {
		w := bytes.NewBuffer(nil)

		err = tmpl.Execute(w, curr)
		if err != nil {
			t.Fatalf("Failed to execute template: %s", err)
		}

		_, err := parseBlockDevicesDescriptor(w.Bytes())
		if err == nil {
			t.Fatalf("Should have failed to parse invalid %s value", curr.name)
		}
	}
}

func TestSizeUnits(t *testing.T) {
	templateStr := `{
    "blockdevices": [
        {
           {{.Value}}
        }
    ]
}`

	tests := []struct {
		size  uint64
		Value string
	}{
		{1024, `"size": "1k"`},
		{1331, `"size": "1.3k"`},
		{1536, `"size": "1.5k"`},
		{1048576, `"size": "1m"`},
		{1363149, `"size": "1.3m"`},
		{1572864, `"size": "1.5m"`},
		{1073741824, `"size": "1g"`},
		{1395864371, `"size": "1.3g"`},
		{1610612736, `"size": "1.5g"`},
		{1099511627776, `"size": "1t"`},
		{1429365116109, `"size": "1.3t"`},
		{1649267441664, `"size": "1.5t"`},
		{1125899906842624, `"size": "1p"`},
		{1463669878895411, `"size": "1.3p"`},
		{1688849860263936, `"size": "1.5p"`},
	}

	tmpl, err := template.New("").Parse(templateStr)
	if err != nil {
		t.Fatalf("Failed to parse template: %s", err)
	}

	for _, curr := range tests {
		w := bytes.NewBuffer(nil)

		err = tmpl.Execute(w, curr)
		if err != nil {
			t.Fatalf("Failed to execute template: %s", err)
		}

		bd, _ := parseBlockDevicesDescriptor(w.Bytes())
		if bd[0].Size != curr.size {
			t.Fatalf("Parsed size: %d doesn't match the expected size: %d",
				bd[0].Size, curr.size)
		}
	}
}

func TestListBlockDevices(t *testing.T) {
	bd, err := ListBlockDevices(nil)
	if err != nil {
		t.Fatalf("Should have listed block devices: %s", err)
	}

	if len(bd) == 0 {
		t.Fatalf("At least one block device should be listed")
	}
}

func TestInvalidBlockDevicesDescriptor(t *testing.T) {
	lsblkOutput := `{
    "blockdevices": [
        {
            "name": [],
            "maj:min": [],
            "rm": [],
            "size": [],
            "ro": [],
            "type": [],
            "mountpoint": []
        }
    ]
}`

	_, err := parseBlockDevicesDescriptor([]byte(lsblkOutput))
	if err == nil {
		t.Fatalf("Should have failed to parse invalid descriptor")
	}
}

func TestParseBlockDevicesDescriptor(t *testing.T) {
	lsblkOutput := `{
    "blockdevices": [
        {
            "name": "sda",
            "maj:min": "8:0",
            "rm": "1",
            "size": "8053063680",
            "ro": "0",
            "type": "disk",
            "mountpoint": null,
            "children": [
                {
                    "name": "sda1",
                    "maj:min": "8:1",
                    "rm": "1",
                    "size": "934281216",
                    "ro": "0",
                    "type": "part",
                    "mountpoint": null
                },
                {
                    "name": "sda2",
                    "maj:min": "8:2",
                    "rm": "1",
                    "size": "524288000",
                    "ro": "0",
                    "type": "part",
                    "mountpoint": null
                }
            ]
        }
    ]
}`

	bd, err := parseBlockDevicesDescriptor([]byte(lsblkOutput))
	if err != nil {
		t.Fatalf("Could not parser block device descriptor: %s", err)
	}

	if len(bd) != 1 {
		t.Fatal("Wrong number of block devices, expected 2")
	}

	bd0 := bd[0]
	if bd0.Name != "sda" {
		t.Fatalf("Block device 0, expected to be named: sda - had: %s", bd0.Name)
	}

	if bd0.MajorMinor != "8:0" {
		t.Fatalf("Block device 0, expected maj:min to be named: 8:0 - had: %s",
			bd0.MajorMinor)
	}

	if bd0.RemovableDevice != true {
		t.Fatalf("Block device 0, expected removable flag: false - had: true")
	}

	if bd0.Size != 8053063680 {
		t.Fatalf("Block device 0, expected size: 8053063680 - had: %d", bd0.Size)
	}

	if bd0.ReadOnly != false {
		t.Fatalf("Block device 0, expected read-only flag: false, had: true")
	}

	if bd0.Type != BlockDeviceTypeDisk {
		t.Fatalf("Block device 0, expected to be block device type: disk, had: part")
	}

	if bd0.MountPoint != "" {
		t.Fatalf("Block device 0, mpoint expected to be null, had: %s", bd0.MountPoint)
	}

	if len(bd0.Children) != 2 {
		t.Fatal("Block device 0, should have 2 children partitions")
	}

	p0 := bd0.Children[0]
	if p0.Name != "sda1" {
		t.Fatalf("Partition 0, expected to be named: sda1 - had: %s", p0.Name)
	}

	if p0.MajorMinor != "8:1" {
		t.Fatalf("Partition 0, expected maj:min to be named: 8:1 - had: %s",
			p0.MajorMinor)
	}

	if p0.RemovableDevice != true {
		t.Fatalf("Partition 0, expected removable flag: true - had: false")
	}

	if p0.Size != 934281216 {
		t.Fatalf("Partition 0, expected size: 934281216 - had: %d", p0.Size)
	}

	if p0.ReadOnly != false {
		t.Fatalf("Partition 0, expected read-only flag: false, had: true")
	}

	if p0.Type != BlockDeviceTypePart {
		t.Fatalf("Partition 0, expected to be block device type: part, had: disk")
	}

	if p0.MountPoint != "" {
		t.Fatalf("Partition 0, mpoint expected to be null, had: %s", p0.MountPoint)
	}

	p1 := bd0.Children[1]
	if p1.Name != "sda2" {
		t.Fatalf("Partition 1, expected to be named: sda2 - had: %s", p0.Name)
	}

	if p1.MajorMinor != "8:2" {
		t.Fatalf("Partition 1, expected maj:min to be named: 8:1 - had: %s",
			p1.MajorMinor)
	}

	if p1.RemovableDevice != true {
		t.Fatalf("Partition 1, expected removable flag: true - had: false")
	}

	if p1.Size != 524288000 {
		t.Fatalf("Partition 1, expected size: 524288000 - had: %d", p1.Size)
	}

	if p1.ReadOnly != false {
		t.Fatalf("Partition 1, expected read-only flag: false, had: true")
	}

	if p1.Type != BlockDeviceTypePart {
		t.Fatalf("Partition 1, expected to be block device type: part, had: disk")
	}

	if p1.MountPoint != "" {
		t.Fatalf("Partition 0, mpoint expected to be null, had: %s", p1.MountPoint)
	}
}

func TestNullRemovable(t *testing.T) {
	lsblkOutput := `{
   "blockdevices": [
      {"name": "sda", "maj:min": "8:0", "rm": "0", "size": "223.6G", "ro": "0", "type": "disk", "mountpoint": null,
         "children": [
            {"name": "sda1", "maj:min": "8:1", "rm": "0", "size": "223.6G", "ro": "0", "type": "part", "mountpoint": null}
         ]
      },
      {"name": "sdb", "maj:min": "8:16", "rm": "0", "size": "1.8T", "ro": "0", "type": "disk", "mountpoint": null,
         "children": [
            {"name": "sdb1", "maj:min": "8:17", "rm": "0", "size": "512M", "ro": "0", "type": "part", "mountpoint": null},
            {"name": "sdb2", "maj:min": "8:18", "rm": "0", "size": "97.7G", "ro": "0", "type": "part", "mountpoint": null},
            {"name": "sdb3", "maj:min": "8:19", "rm": "0", "size": "31.9G", "ro": "0", "type": "part", "mountpoint": null},
            {"name": "sdb4", "maj:min": "8:20", "rm": "0", "size": "97.7G", "ro": "0", "type": "part", "mountpoint": null},
            {"name": "sdb5", "maj:min": "8:21", "rm": "0", "size": "1.6T", "ro": "0", "type": "part", "mountpoint": null}
         ]
      },
      {"name": "sdc", "maj:min": "8:32", "rm": "0", "size": "1.8T", "ro": "0", "type": "disk", "mountpoint": null,
         "children": [
            {"name": "sdc1", "maj:min": "8:33", "rm": null, "size": "1G", "ro": "0", "type": "part", "mountpoint": null},
            {"name": "sdc2", "maj:min": "8:34", "rm": "0", "size": "1.8T", "ro": "0", "type": "part", "mountpoint": null}
         ]
      },
      {"name": "sr0", "maj:min": "11:0", "rm": "1", "size": "1024M", "ro": "0", "type": "rom", "mountpoint": null}
   ]
}`

	_, err := parseBlockDevicesDescriptor([]byte(lsblkOutput))
	if err != nil {
		t.Fatalf("Could not parser block device descriptor: %s", err)
	}
}
