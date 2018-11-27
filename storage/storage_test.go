// Copyright © 2018 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package storage

import (
	"bytes"
	"io/ioutil"
	"os"
	"path"
	"testing"
	"text/template"
	"time"

	"github.com/clearlinux/clr-installer/progress"
	"github.com/clearlinux/clr-installer/utils"
)

// Need to implement an empty progress interface for testing
// FakeInstall implements the progress interface: progress.Client
type FakeInstall struct {
	prgDesc string
}

// Step is the progress step implementation for progress.Client interface
func (mi *FakeInstall) Step() { return }

// LoopWaitDuration is part of the progress.Client implementation and returns the
// duration each loop progress step should wait
func (mi *FakeInstall) LoopWaitDuration() time.Duration {
	return 1 * time.Millisecond
}

// Desc is part of the implementation for ProgresIface and is used to adjust the progress bar
// label content
func (mi *FakeInstall) Desc(desc string) {
	mi.prgDesc = desc
}

// Partial is part of the progress.Client implementation and sets the progress bar based
// on actual progression
func (mi *FakeInstall) Partial(total int, step int) { return }

// Success is part of the progress.Client implementation and represents the
// successful progress completion of a task
func (mi *FakeInstall) Success() { return }

// Failure is part of the progress.Client implementation and represents the
// unsuccessful progress completion of a task
func (mi *FakeInstall) Failure() { return }

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
	if !utils.IsRoot() {
		t.Log("Not running as 'root', not using Loopback device")
	} else {
		bd, err := ListBlockDevices(nil)
		if err != nil {
			t.Fatalf("Should have listed block devices: %s", err)
		}

		if len(bd) == 0 {
			t.Fatalf("At least one block device should be listed")
		}
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

func TestWritePartition(t *testing.T) {
	tmpFile, err := ioutil.TempFile("", "test-image-")
	if err != nil {
		t.Fatal("Could not create a temp file")
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()
	imageFile := tmpFile.Name()
	if err = tmpFile.Close(); err != nil {
		t.Fatal(err)
	}
	t.Logf("Image file is :%s", imageFile)

	children := make([]*BlockDevice, 0)
	bd := &BlockDevice{Name: "", Size: 1288490188, Type: BlockDeviceTypeLoop, Children: children}

	if err = MakeImage(bd, imageFile); err != nil {
		t.Fatalf("Could not make image file: %s", err)
	}

	if !utils.IsRoot() {
		t.Log("Not running as 'root', not using Loopback device")
	} else {
		detachMe := []string{}
		fakeImpl := &FakeInstall{}
		progress.Set(fakeImpl)

		file, err := SetupLoopDevice(imageFile)
		if err != nil {
			t.Fatalf("Could not setup loop device for image file %s: %s", file, err)
		}

		detachMe = append(detachMe, file)

		retry := 5
		// wait the loop device to be prepared and available with 5 retry attempts
		for {
			var ok bool

			if ok, err = utils.FileExists(file); err != nil {
				for _, file := range detachMe {
					DetachLoopDevice(file)
				}
				t.Fatalf("Could not check for file exists (%s): %s", file, err)
			}

			if ok || retry == 0 {
				break
			}

			retry--
			time.Sleep(time.Second * 1)
		}

		// defer detaching used loop devices
		defer func() {
			for _, file := range detachMe {
				DetachLoopDevice(file)
			}
		}()
		bd.Name = path.Base(file)
		part1 := &BlockDevice{Name: bd.Name + "p1", FsType: "vfat", Size: 157286400, Type: BlockDeviceTypePart, MountPoint: "/boot"}
		part2 := &BlockDevice{Name: bd.Name + "p2", FsType: "swap", Size: 125829120, Type: BlockDeviceTypePart, MountPoint: ""}
		part3 := &BlockDevice{Name: bd.Name + "p3", FsType: "ext4", Size: 502267904, Type: BlockDeviceTypePart, MountPoint: "/"}
		part4 := &BlockDevice{Name: bd.Name + "p4", FsType: "ext4", Size: 502267904, Type: BlockDeviceTypeCrypt, MountPoint: "/home"}

		children = append(children, part1)
		children = append(children, part2)
		children = append(children, part3)
		children = append(children, part4)
		bd.Children = children

		//write the partition table
		if err = bd.WritePartitionTable(); err != nil {
			t.Fatalf("Could not write partition table (%s): %s", file, err)
		}

		// prepare the blockdevice's partitions filesystem
		for _, ch := range bd.Children {
			if ch.Type == BlockDeviceTypeCrypt {
				if ch.FsType != "swap" {
					t.Logf("Mapping %s partition to an encrypted partition", ch.Name)
					if err = ch.MapEncrypted("P@ssW0rd"); err != nil {
						t.Fatalf("Could not Map Encrypted  partition (%s): %s", ch.Name, err)
					}
				}
			}
			if err = ch.MakeFs(); err != nil {
				t.Fatalf("Could not MakeFs partition (%s): %s", ch.Name, err)
			}
		}
		bds := []*BlockDevice{bd}
		if scanErr := UpdateBlockDevices(bds); scanErr != nil {
			t.Fatalf("Could not UpdateBlockDevices: %s", scanErr)
		}

		if UmountAll() != nil {
			t.Fatalf("Could not unmount volumes")
		}
	}
}

func TestValidDiskSize(t *testing.T) {
	lsblkOutput := `{
   "blockdevices": [
      {"name": "sda", "maj:min": "8:0", "rm": "0", "size": "223.6G", "ro": "0", "type": "disk", "mountpoint": null,
         "children": [
            {"name": "sda1", "maj:min": "8:1", "rm": "0", "size": "223.6G", "ro": "0", "type": "part", "mountpoint": null}
         ]
      },
      {"name": "sdb", "maj:min": "8:16", "rm": "0", "size": "2.0T", "ro": "0", "type": "disk", "mountpoint": null,
         "children": [
            {"name": "sdb1", "maj:min": "8:17", "rm": "0", "size": "512M", "ro": "0", "type": "part", "mountpoint": null},
            {"name": "sdb2", "maj:min": "8:18", "rm": "0", "size": "97.7G", "ro": "0", "type": "part", "mountpoint": null},
            {"name": "sdb3", "maj:min": "8:19", "rm": "0", "size": "31.9G", "ro": "0", "type": "part", "mountpoint": null},
            {"name": "sdb4", "maj:min": "8:20", "rm": "0", "size": "97.7G", "ro": "0", "type": "part", "mountpoint": null},
            {"name": "sdb5", "maj:min": "8:21", "rm": "0", "size": "1.6T", "ro": "0", "type": "part", "mountpoint": null}
         ]
      },
      {"name": "sdc", "maj:min": "8:32", "rm": "0", "size": "2.8T", "ro": "0", "type": "disk", "mountpoint": null,
         "children": [
            {"name": "sdc1", "maj:min": "8:33", "rm": null, "size": "1G", "ro": "0", "type": "part", "mountpoint": null},
            {"name": "sdc2", "maj:min": "8:34", "rm": "0", "size": "1.8T", "ro": "0", "type": "part", "mountpoint": null}
         ]
      },
      {"name": "sde", "maj:min": "8:128", "rm": "0", "size": "2.0T", "rw": "0", "type": "disk", "mountpoint": null,
         "children": [
            {"name": "sde1", "maj:min": "8:129", "rm": "0", "size": "512M", "rw": "0", "type": "part", "mountpoint": "/boot"},
            {"name": "sde2", "maj:min": "8:130", "rm": "0", "size": "97.7G", "rw": "0", "type": "part", "mountpoint": null},
            {"name": "sde3", "maj:min": "8:131", "rm": "0", "size": "31.9G", "rw": "0", "type": "crypt", "mountpoint": "/"},
            {"name": "sde4", "maj:min": "8:132", "rm": "0", "size": "97.7G", "rw": "0", "type": "crypt", "mountpoint": "/home"},
            {"name": "sde5", "maj:min": "8:133", "rm": "0", "size": "0.6T", "rw": "0", "type": "crypt", "mountpoint": "/secure"},
            {"name": "sde6", "maj:min": "8:134", "rm": "0", "size": "1.0T", "rw": "0", "type": "part", "mountpoint": "/db"}
         ]
      },
      {"name": "sr0", "maj:min": "11:0", "rm": "1", "size": "1024M", "ro": "0", "type": "rom", "mountpoint": null}
   ]
}`

	bds, err := parseBlockDevicesDescriptor([]byte(lsblkOutput))
	if err != nil {
		t.Fatalf("Could not parser block device descriptor: %s", err)
	}

	for _, bd := range bds {
		size, err := bd.DiskSize()
		if err != nil {
			t.Fatalf("Invalid Disk Size: %s", err)
		}
		t.Logf("Disk %s is Size %d", bd.Name, size)

		if bd.Name == "sde" {
			for _, ch := range bd.Children {
				isStandard := ch.isStandardMount()
				if ch.Name == "sde2" || ch.Name == "sde5" || ch.Name == "sde6" {
					if isStandard {
						t.Fatalf("Partition %s should NOT be standard [%s]", ch.Name, ch.MountPoint)
					}
				} else {
					if !isStandard {
						t.Fatalf("Partition %s should be standard [%s]", ch.Name, ch.MountPoint)
					}
				}
			}
		}
	}
}

func TestInvalidDiskSize(t *testing.T) {
	lsblkOutput := `{
   "blockdevices": [
      {"name": "sdb", "maj:min": "8:16", "rm": "0", "size": "1.8T", "ro": "0", "type": "disk", "mountpoint": null,
         "children": [
            {"name": "sdb1", "maj:min": "8:17", "rm": "0", "size": "512M", "ro": "0", "type": "part", "mountpoint": null},
            {"name": "sdb2", "maj:min": "8:18", "rm": "0", "size": "97.7G", "ro": "0", "type": "part", "mountpoint": null},
            {"name": "sdb3", "maj:min": "8:19", "rm": "0", "size": "31.9G", "ro": "0", "type": "part", "mountpoint": null},
            {"name": "sdb4", "maj:min": "8:20", "rm": "0", "size": "97.7G", "ro": "0", "type": "part", "mountpoint": null},
            {"name": "sdb5", "maj:min": "8:21", "rm": "0", "size": "1.6T", "ro": "0", "type": "part", "mountpoint": null}
         ]
      }
   ]
}`

	bds, err := parseBlockDevicesDescriptor([]byte(lsblkOutput))
	if err != nil {
		t.Fatalf("Could not parser block device descriptor: %s", err)
	}

	for _, bd := range bds {
		size, err := bd.DiskSize()
		if err == nil {
			t.Fatalf("Disk %s Size should be invalid", bd.Name)
		}
		t.Logf("Disk %s is Size %d", bd.Name, size)
	}
}

func TestValidLabels(t *testing.T) {
	labelInfo := []struct {
		fstype string
		label  string
	}{
		{"ext2", "a"},
		{"ext2", "root"},
		{"ext2", "1234567890123456"},
		{"ext3", "a"},
		{"ext3", "Root"},
		{"ext3", "1234567890123456"},
		{"ext4", "a"},
		{"ext4", "ROOT"},
		{"ext4", "1234567890123456"},
		{"swap", "SWAP"},
		{"swap", "123456789012345"},
		{"xfs", "home"},
		{"xfs", "123456789012"},
		{"btrfs", "home"},
		{"btrfs", "12345678901234567890123456789012345678901234567890" +
			"12345678901234567890123456789012345678901234567890" +
			"12345678901234567890123456789012345678901234567890" +
			"12345678901234567890123456789012345678901234567890" +
			"12345678901234567890123456789012345678901234567890" +
			"12345"},
		{"vfat", "BOOT"},
		{"vfat", "12345678901"},
		{"unknown", "BOOT"},
		{"unknown", "12345678901"},
	}

	for _, curr := range labelInfo {
		if result := IsValidLabel(curr.label, curr.fstype); result != "" {
			t.Fatalf("Label %q should be valid for fstype %q: %s", curr.label, curr.fstype, result)
		}
	}
}

func TestInvalidLabels(t *testing.T) {
	labelInfo := []struct {
		fstype string
		label  string
	}{
		{"ext2", "!"},
		{"ext2", "12345678901234567"},
		{"ext3", "@"},
		{"ext3", "12345678901234567890"},
		{"ext4", "$"},
		{"ext4", "1234567890123456789012345"},
		{"swap", "	"},
		{"swap", "1234567890123456"},
		{"xfs", "*"},
		{"xfs", "1234567890123"},
		{"btrfs", "("},
		{"btrfs", ")"},
		{"btrfs", "="},
		{"btrfs", "12345678901234567890123456789012345678901234567890" +
			"12345678901234567890123456789012345678901234567890" +
			"12345678901234567890123456789012345678901234567890" +
			"12345678901234567890123456789012345678901234567890" +
			"12345678901234567890123456789012345678901234567890" +
			"123456"},
		{"vfat", "#"},
		{"vfat", "123456789012"},
		{"unknown", "~"},
		{"unknown", "123456789012"},
	}

	for _, curr := range labelInfo {
		if result := IsValidLabel(curr.label, curr.fstype); result == "" {
			t.Fatalf("Label %q should be INVALID for fstype %q", curr.label, curr.fstype)
		}
	}
}

func TestValidPassphrase(t *testing.T) {
	passphrases := []string{
		"password",
		"P@ssW0rd",
		"1234567890123456789012345678901234567890" +
			"1234567890123456789012345678901234567890" +
			"12345678901234",
		"~!@#$%^&*()_+=][",
	}

	for _, curr := range passphrases {
		if valid, result := IsValidPassphrase(curr); !valid {
			t.Fatalf("Passphrase %q should be valid: %s ", curr, result)
		}
	}
}

func TestInvalidPassphrase(t *testing.T) {
	passphrases := []string{
		"",
		"@ssW0rd",
		"								",
		"1234567890123456789012345678901234567890" +
			"1234567890123456789012345678901234567890" +
			"123456789012345",
		"~!)_+][",
	}

	for _, curr := range passphrases {
		if valid, _ := IsValidPassphrase(curr); valid {
			t.Fatalf("Passphrase %q should be INVALID", curr)
		}
	}
}

func TestValidMakeFsCommand(t *testing.T) {
	lsblkOutput := `{
   "blockdevices": [
      {"name": "sde", "maj:min": "8:128", "rm": "0", "size": "2.0T", "rw": "0", "type": "disk", "mountpoint": null,
         "children": [
            {"name": "sde1", "maj:min": "8:129", "rm": "0", "fstype": "vfat", "label": "boot", "size": "512M", "rw": "0", "type": "part", "mountpoint": "/boot"},
            {"name": "sde2", "maj:min": "8:130", "rm": "0", "fstype": "swap", "label": "swap", "size": "128M", "rw": "0", "type": "part", "mountpoint": null},
            {"name": "sde3", "maj:min": "8:131", "rm": "0", "fstype": "ext4", "label": "root", "size": "6G", "rw": "0", "type": "crypt", "mountpoint": "/"},
            {"name": "sde4", "maj:min": "8:132", "rm": "0", "fstype": "ext4", "label": "home", "size": "1G", "rw": "0", "type": "crypt", "mountpoint": "/home"},
            {"name": "sde5", "maj:min": "8:133", "rm": "0", "fstype": "xfs", "label": "secure", "size": "1.6T", "rw": "0", "type": "crypt", "mountpoint": "/secure"}
         ]
      }
   ]
}`

	bds, err := parseBlockDevicesDescriptor([]byte(lsblkOutput))
	extraCmds := []string{}

	if err != nil {
		t.Fatalf("Could not parser block device descriptor: %s", err)
	}

	for _, bd := range bds {
		if bd.FsTypeNotSwap() {
			if cmd, err := commonMakeFsCommand(bd, extraCmds); err != nil {
				t.Fatalf("Could not discover the mkfs command: %s", err)
			} else {
				t.Logf("Disk %s uses %s", bd.Name, cmd)
			}
		} else {
			if cmd, err := swapMakeFsCommand(bd, extraCmds); err != nil {
				t.Fatalf("Could not discover the swap command: %s", err)
			} else {
				t.Logf("Disk %s uses %s", bd.Name, cmd)
			}
		}
	}
}

func TestWriteConfigFiles(t *testing.T) {
	lsblkOutput := `{
   "blockdevices": [
      {"name": "sde", "maj:min": "8:128", "rm": "0", "size": "2.0T", "rw": "0", "type": "disk", "mountpoint": null,
         "children": [
            {"name": "sde1", "maj:min": "8:129", "rm": "0", "fstype": "vfat", "label": "boot", "size": "512M", "rw": "0", "type": "part", "mountpoint": "/boot"},
            {"name": "sde2", "maj:min": "8:130", "rm": "0", "fstype": "swap", "label": "swap", "size": "128M", "rw": "0", "type": "crypt", "mountpoint": null},
            {"name": "sde3", "maj:min": "8:131", "rm": "0", "fstype": "ext4", "label": "root", "size": "6G", "rw": "0", "type": "crypt", "mountpoint": "/"},
            {"name": "sde4", "maj:min": "8:132", "rm": "0", "fstype": "ext4", "label": "share", "size": "1G", "rw": "0", "type": "part", "mountpoint": "/share"},
            {"name": "sde5", "maj:min": "8:133", "rm": "0", "fstype": "xfs", "label": "secure", "size": "1.6T", "rw": "0", "type": "crypt", "mountpoint": "/secure"}
         ]
      }
   ]
}`

	bds, bdsErr := parseBlockDevicesDescriptor([]byte(lsblkOutput))

	if bdsErr != nil {
		t.Fatalf("Could not parser block device descriptor: %s", bdsErr)
	}

	rootDir, err := ioutil.TempDir("", "clr-installer-storage-test")
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		_ = os.RemoveAll(rootDir)
	}()

	if err := GenerateTabFiles(rootDir, bds); err != nil {
		t.Fatalf("Failed to create directories to write config file: %v\n", err)
	}
}
