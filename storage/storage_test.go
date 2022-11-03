// Copyright © 2020 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package storage

import (
	"bytes"
	"io/ioutil"
	"os"
	"path"
	"sort"
	"testing"
	"text/template"
	"time"

	"github.com/clearlinux/clr-installer/progress"
	"github.com/clearlinux/clr-installer/utils"
)

func init() {
	utils.SetLocale("en_US.UTF-8")
}

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
	expected := []string{"btrfs", "ext2", "ext3", "ext4", "swap", "vfat", "xfs", "f2fs"}
	supported := []string{}
	tot := 0

	for key := range bdOps {
		supported = append(supported, key)
	}
	sort.Strings(supported)

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
		{"size", `"size": 1.1`},
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
		t.Fatalf("Partition 1, expected to be named: sda2 - had: %s", p1.Name)
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
		t.Fatalf("Partition 1, mpoint expected to be null, had: %s", p1.MountPoint)
	}
}

func TestNullRemovable(t *testing.T) {
	//nolint: lll // WONTFIX
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

func TestRAID(t *testing.T) {
	//nolint: lll // WONTFIX
	lsblkOutput := `{
   "blockdevices": [
      {"name":"sdb", "kname":"sdb", "path":"/dev/sdb", "maj:min":"8:16", "fsavail":null, "fssize":null, "fstype":null, "fsused":null, "fsuse%":null, "mountpoint":null, "label":null, "pttype":"gpt", "parttype":null, "partlabel":null, "ra":1024, "ro":false, "rm":false, "hotplug":false, "size":1000204886016, "state":"running", "owner":"root", "group":"disk", "mode":"brw-rw----", "alignment":0, "min-io":4096, "opt-io":0, "phy-sec":4096, "log-sec":512, "rota":false, "sched":"bfq", "rq-size":1024, "type":"disk", "disc-aln":0, "disc-gran":4096, "disc-max":2147450880, "disc-zero":false, "wsame":0, "wwn":"0x500a0751e1eda080", "rand":true, "pkname":null, "hctl":"7:0:0:0", "tran":"sata", "subsystems":"block:scsi:pci", "rev":"023 ", "vendor":"ATA     ", "zoned":"none",
         "children": [
            {"name":"sdb1", "kname":"sdb1", "path":"/dev/sdb1", "maj:min":"8:17", "fsavail":null, "fssize":null, "fstype":"linux_raid_member", "fsused":null, "fsuse%":null, "mountpoint":null, "label":"localhost-live:home", "pttype":"gpt", "parttype":"a19d880f-05fc-4d3b-a006-743f0f84911e", "partlabel":null, "partflags":null, "ra":1024, "ro":false, "rm":false, "hotplug":false, "size":1000203091968, "state":null, "owner":"root", "group":"disk", "mode":"brw-rw----", "alignment":0, "min-io":4096, "opt-io":0, "phy-sec":4096, "log-sec":512, "rota":false, "sched":"bfq", "rq-size":1024, "type":"part", "disc-aln":0, "disc-gran":4096, "disc-max":2147450880, "disc-zero":false, "wsame":0, "wwn":"0x500a0751e1eda080", "rand":true, "pkname":"sdb", "hctl":null, "tran":null, "subsystems":"block:scsi:pci", "rev":null, "vendor":null, "zoned":"none",
               "children": [
                  {"name":"md127", "kname":"md127", "path":"/dev/md127", "maj:min":"9:127", "fsavail":"4790297608192", "fssize":"4998202130432", "fstype":"xfs", "fsused":"207904522240", "fsuse%":"4%", "mountpoint":"/home", "label":"home", "pttype":null, "parttype":null, "partlabel":null, "partflags":null, "ra":5120, "ro":false, "rm":false, "hotplug":false, "size":5000339128320, "state":null, "owner":"root", "group":"disk", "mode":"brw-rw----", "alignment":0, "min-io":524288, "opt-io":2621440, "phy-sec":4096, "log-sec":512, "rota":false, "sched":null, "rq-size":128, "type":"raid5", "disc-aln":0, "disc-gran":4194304, "disc-max":2147450880, "disc-zero":false, "wsame":0, "wwn":null, "rand":false, "pkname":"sdb1", "hctl":null, "tran":null, "subsystems":"block", "rev":null, "vendor":null, "zoned":"none"}
               ]
            }
         ]
      },
      {"name":"sdc", "kname":"sdc", "path":"/dev/sdc", "maj:min":"8:32", "fsavail":null, "fssize":null, "fstype":null, "fsused":null, "fsuse%":null, "mountpoint":null, "label":null, "pttype":"gpt", "parttype":null, "partlabel":null, "partflags":null, "ra":1024, "ro":false, "rm":false, "hotplug":false, "size":1000204886016, "state":"running", "owner":"root", "group":"disk", "mode":"brw-rw----", "alignment":0, "min-io":4096, "opt-io":0, "phy-sec":4096, "log-sec":512, "rota":false, "sched":"bfq", "rq-size":1024, "type":"disk", "disc-aln":0, "disc-gran":4096, "disc-max":2147450880, "disc-zero":false, "wsame":0, "wwn":"0x500a0751e1f0f6eb", "rand":true, "pkname":null, "hctl":"8:0:0:0", "tran":"sata", "subsystems":"block:scsi:pci", "rev":"023 ", "vendor":"ATA     ", "zoned":"none",
         "children": [
            {"name":"sdc1", "kname":"sdc1", "path":"/dev/sdc1", "maj:min":"8:33", "fsavail":null, "fssize":null, "fstype":"linux_raid_member", "fsused":null, "fsuse%":null, "mountpoint":null, "label":"localhost-live:home", "pttype":"gpt", "parttype":"a19d880f-05fc-4d3b-a006-743f0f84911e", "partlabel":null, "partflags":null, "ra":1024, "ro":false, "rm":false, "hotplug":false, "size":1000203091968, "state":null, "owner":"root", "group":"disk", "mode":"brw-rw----", "alignment":0, "min-io":4096, "opt-io":0, "phy-sec":4096, "log-sec":512, "rota":false, "sched":"bfq", "rq-size":1024, "type":"part", "disc-aln":0, "disc-gran":4096, "disc-max":2147450880, "disc-zero":false, "wsame":0, "wwn":"0x500a0751e1f0f6eb", "rand":true, "pkname":"sdc", "hctl":null, "tran":null, "subsystems":"block:scsi:pci", "rev":null, "vendor":null, "zoned":"none",
               "children": [
                  {"name":"md127", "kname":"md127", "path":"/dev/md127", "maj:min":"9:127", "fsavail":"4790297608192", "fssize":"4998202130432", "fstype":"xfs", "fsused":"207904522240", "fsuse%":"4%", "mountpoint":"/home", "label":"home", "pttype":null, "parttype":null, "partlabel":null, "partflags":null, "ra":5120, "ro":false, "rm":false, "hotplug":false, "size":5000339128320, "state":null, "owner":"root", "group":"disk", "mode":"brw-rw----", "alignment":0, "min-io":524288, "opt-io":2621440, "phy-sec":4096, "log-sec":512, "rota":false, "sched":null, "rq-size":128, "type":"raid5", "disc-aln":0, "disc-gran":4194304, "disc-max":2147450880, "disc-zero":false, "wsame":0, "wwn":null, "rand":false, "pkname":"sdc1", "hctl":null, "tran":null, "subsystems":"block", "rev":null, "vendor":null, "zoned":"none"}
               ]
            }
         ]
      }
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

		part1 :=
			&BlockDevice{Name: bd.Name + "p1",
				FsType: "vfat", Size: 157286400,
				PartitionLabel: "CLR_BOOT",
				Type:           BlockDeviceTypePart,
				MountPoint:     "/boot",
				MakePartition:  true}

		part2 :=
			&BlockDevice{Name: bd.Name + "p2",
				FsType:         "swap",
				Size:           125829120,
				PartitionLabel: "CLR_SWAP",
				Type:           BlockDeviceTypePart,
				MountPoint:     "",
				MakePartition:  true}

		part3 :=
			&BlockDevice{Name: bd.Name + "p3",
				FsType:         "ext4",
				Size:           502267904,
				PartitionLabel: "CLR_ROOT_F",
				Type:           BlockDeviceTypePart,
				MountPoint:     "/",
				MakePartition:  true}

		part4 :=
			&BlockDevice{Name: bd.Name + "p4",
				FsType:         "ext4",
				Size:           502267904,
				PartitionLabel: "CLR_MNT_/home",
				Type:           BlockDeviceTypeCrypt,
				MountPoint:     "/home",
				MakePartition:  true}

		children = append(children, part1)
		children = append(children, part2)
		children = append(children, part3)
		children = append(children, part4)
		bd.Children = children

		//write the partition table (dryrun)
		var dryRun = &DryRunType{&[]string{}, &[]string{}}
		if err = bd.WritePartitionTable(true, false, dryRun); err != nil {
			t.Fatalf("Could not dryrun write partition table (%s): %s", file, err)
		}

		//write the partition table
		if err = bd.WritePartitionTable(true, false, nil); err != nil {
			t.Fatalf("Could not write partition table (%s): %s", file, err)
		}

		// prepare the blockdevice's partitions filesystem
		for _, ch := range bd.Children {
			if err = ch.updatePartitionInfo(); err != nil {
				t.Fatalf("Could not updatePartitionInfo partition (%s): %s", ch.Name, err)
			}

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

		found := FindAdvancedInstallTargets(bds)
		if len(found) == 0 {
			t.Fatalf("Should have found any advanced targets %+v", found)
		}

		if AdvancedPartitionsRequireEncryption(bds) {
			t.Fatalf("Advanced targets should not require encryption")
		}

		if scanErr := UpdateBlockDevices(bds); scanErr != nil {
			t.Fatalf("Could not UpdateBlockDevices: %s", scanErr)
		}

		if UmountAll() != nil {
			t.Fatalf("Could not unmount volumes")
		}
	}
}

func TestValidDiskSize(t *testing.T) {
	//nolint: lll // WONTFIX
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
	//nolint: lll // WONTFIX
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

func TestValidPassphrase(t *testing.T) {
	passphrases := []string{
		"P@ssW0rd",
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
		"Password",
		"drowssap",
		"1234567890123456789012345678901234567890" +
			"1234567890123456789012345678901234567890" +
			"12345678901234",
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
	//nolint: lll // WONTFIX
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
	//nolint: lll // WONTFIX
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

func TestInstallTargets(t *testing.T) {
	getPartAllFreeOutput := `
BYT;
/dev/sde:30752636928B:scsi:512:512:gpt:SanDisk Ultra USB 3.0:;
1:17408B:30752620031B:30752602624B:free;
`
	getPartSomeFreeOutput := `
BYT;
/dev/sdc:2000398934016B:scsi:512:4096:gpt:ATA ST2000DM001-1ER1:;
1:17408B:150000127B:149982720B:fat32:EFI:boot, esp;
2:150000128B:2198000127B:2048000000B:linux-swap(v1):linux-swap:;
3:2198000128B:1907729000447B:1905531000320B:ext4:/:;
1:1907729000448B:2000398917119B:92669916672B:free;
`
	getPartNotEnoughFreeOutput := `
BYT;
/dev/sda:240057409536B:scsi:512:512:gpt:ATA INTEL SSDSC2BW24:;
1:17408B:1048575B:1031168B:free;
1:1048576B:149946367B:148897792B:fat32:EFI:boot, esp;
2:149946368B:182452223B:32505856B:linux-swap(v1):linux-swap:;
3:182452224B:7799308287B:7616856064B:ext4:/:;
4:7799308288B:240056795135B:232257486848B::ext4:;
1:240056795136B:240057392639B:597504B:free;
`
	getPartNotEnoughFree2Output := `
BYT;
/dev/sdb:2000398934016B:scsi:512:4096:gpt:ATA ST2000DM001-1ER1:;
1:17408B:1048575B:1031168B:free;
1:1048576B:537919487B:536870912B:fat32::boot, esp;
2:537919488B:105395519487B:104857600000B:ext4:ubuntu1404:;
4:105395519488B:210253119487B:104857600000B:ext4:ubuntu1604:;
5:210253119488B:1966220509183B:1755967389696B:ext4::;
3:1966220509184B:2000398843903B:34178334720B:linux-swap(v1)::;
1:2000398843904B:2000398917119B:73216B:free;
`
	getPartNotEnoughFree3Output := `
/dev/sdd:7822376960B:scsi:512:512:gpt:JetFlash Transcend 8GB:;
1:17408B:1048575B:1031168B:free;
1:1048576B:149946367B:148897792B:fat32:EFI:boot, esp;
2:149946368B:182452223B:32505856B:linux-swap(v1):linux-swap:;
3:182452224B:7799308287B:7616856064B:ext4:/:;
1:7799308288B:7822360063B:23051776B:free;
`

	var start, end, twentyGig, fourGig uint64
	children := make([]*BlockDevice, 0)
	bd := &BlockDevice{Name: "sda", Children: children}

	twentyGig = 21474836480
	fourGig = 4294967296
	t.Logf("getPartAllFreeOutput: twentyGig: %d, fourGig: %d", twentyGig, fourGig)

	bd.setPartitionTable(bytes.NewBuffer([]byte(getPartAllFreeOutput)))
	start, end = bd.LargestContiguousFreeSpace(twentyGig)
	if start == 0 && end == 0 {
		t.Fatalf("Should have found %d free in getPartAllFreeOutput", twentyGig)
	}
	t.Logf("getPartAllFreeOutput: start: %d, end: %d", start, end)

	bd.setPartitionTable(bytes.NewBuffer([]byte(getPartSomeFreeOutput)))
	start, end = bd.LargestContiguousFreeSpace(twentyGig)
	if start == 0 && end == 0 {
		t.Fatalf("Should have found %d free in getPartSomeFreeOutput", twentyGig)
	}
	t.Logf("getPartSomeFreeOutput: start: %d, end: %d", start, end)

	bd.setPartitionTable(bytes.NewBuffer([]byte(getPartNotEnoughFreeOutput)))
	start, end = bd.LargestContiguousFreeSpace(fourGig)
	if start != 0 || end != 0 {
		t.Logf("getPartNotEnoughFreeOutput: start: %d, end: %d", start, end)
		t.Fatalf("Should NOT have found %d free in getPartNotEnoughFreeOutput", twentyGig)
	}
	t.Logf("getPartNotEnoughFreeOutput: start: %d, end: %d", start, end)

	bd.setPartitionTable(bytes.NewBuffer([]byte(getPartNotEnoughFree2Output)))
	start, end = bd.LargestContiguousFreeSpace(twentyGig)
	if start != 0 || end != 0 {
		t.Logf("getPartNotEnoughFree2Output: start: %d, end: %d", start, end)
		t.Fatalf("Should NOT have found %d free in getPartNotEnoughFree2Output", twentyGig)
	}
	t.Logf("getPartNotEnoughFree2Output: start: %d, end: %d", start, end)

	bd.setPartitionTable(bytes.NewBuffer([]byte(getPartNotEnoughFree3Output)))
	start, end = bd.LargestContiguousFreeSpace(twentyGig)
	if start != 0 || end != 0 {
		t.Logf("getPartNotEnoughFree3Output: start: %d, end: %d", start, end)
		t.Fatalf("Should NOT have found %d free in getPartNotEnoughFree3Output", twentyGig)
	}
	t.Logf("getPartNotEnoughFree3Output: start: %d, end: %d", start, end)
}

func TestAddPartititions(t *testing.T) {
	bd := &BlockDevice{Size: MinimumServerInstallSize}

	size := AddBootStandardPartition(bd)
	if size != bootSizeDefault {
		t.Fatalf("Boot partition should be %d, but was %d", bootSizeDefault, size)
	}

	rootSize := uint64(bd.Size - bootSizeDefault)
	AddRootStandardPartition(bd, rootSize)
}

// nolint: lll // WONTFIX
var lsblkOutput = `{
   "blockdevices": [
	  {"name": "sde", "path": "/dev/sde", "size": "2.0T", "type": "disk", "mountpoint": null,
         "children": [
			{"name": "sde1", "path": "/dev/sde1", "size": "512M",  "type": "part", "fstype": "vfat", "mountpoint": "/boot"},
            {"name": "sde2", "path": "/dev/sde2", "size": "256M",  "type": "part", "fstype": "swap", "mountpoint": null},
            {"name": "sde3", "path": "/dev/sde3", "size": "8G",    "type": "part", "fstype": "ext4", "mountpoint": "/"},
            {"name": "sde4", "path": "/dev/sde4", "size": "8G",    "type": "part", "fstype": "ext4", "mountpoint": "/home"}
         ]
      },
	  {"name": "sdf", "path": "/dev/sdf", "size": "2.0T", "type": "disk", "mountpoint": null,
         "children": [
			{"name": "sdf1", "path": "/dev/sdf1", "size": "512M",  "type": "part", "fstype": "vfat", "mountpoint": "/boot"},
            {"name": "sdf3", "path": "/dev/sdf3", "size": "8G",    "type": "part", "fstype": "ext4", "mountpoint": "/"},
            {"name": "sdf4", "path": "/dev/sdf4", "size": "8G",    "type": "part", "fstype": "ext4", "mountpoint": "/var"}
         ]
      },
	  {"name": "sdg", "path": "/dev/sdg", "size": "2.0T", "type": "disk", "mountpoint": null,
         "children": [
			{"name": "sdg1", "path": "/dev/sdg1", "size": "512M",  "type": "part", "fstype": "vfat", "partlabel": "CLR_BOOT", "mountpoint": "/boot"},
            {"name": "sdg2", "path": "/dev/sdg2", "size": "256M",  "type": "part", "fstype": "swap", "partlabel": "CLR_SWAP", "mountpoint": null},
            {"name": "sdg3", "path": "/dev/sdg3", "size": "8G",    "type": "part", "fstype": "ext4", "partlabel": "CLR_ROOT", "mountpoint": "/"},
            {"name": "sdg4", "path": "/dev/sdg4", "size": "8G",    "type": "part", "fstype": "ext4", "partlabel": "CLR_MNT_/home", "mountpoint": "/home"}
         ]
      },
	  {"name": "sdh", "path": "/dev/sdh", "size": "2.0T", "type": "disk", "mountpoint": null,
         "children": [
			{"name": "sdh1", "path": "/dev/sdh1", "size": "2.0T",  "type": "part", "fstype": "ext4",  "partlabel": "CLR_ROOT", "mountpoint": "/"}
         ]
      },
	  {"name": "sda", "path": "/dev/sda", "size": "2.0T", "type": "disk", "mountpoint": null,
         "children": [
			{"name": "sda1", "path": "/dev/sda1", "size": "512M",  "type": "part", "fstype": "vfat", "mountpoint": "/boot"},
            {"name": "sda2", "path": "/dev/sda2", "size": "256M",  "type": "part", "fstype": "swap", "mountpoint": null},
            {"name": "sda3", "path": "/dev/sda3", "size": "20G",   "type": "part", "fstype": "ext4", "mountpoint": "/"},
            {"name": "sda4", "path": "/dev/sda4", "size": "20G",   "type": "part", "fstype": "ext4", "mountpoint": "/home"}
         ]
      },
	  {"name": "sdb", "path": "/dev/sdb", "size": "2.0T", "type": "disk", "mountpoint": null,
         "children": [
			{"name": "sdb1", "path": "/dev/sdb1", "size": "512M",  "type": "part", "fstype": "vfat", "mountpoint": "/boot"},
            {"name": "sdb3", "path": "/dev/sdb3", "size": "20G",   "type": "part", "fstype": "ext4", "mountpoint": "/"},
            {"name": "sdb4", "path": "/dev/sdb4", "size": "20G",   "type": "part", "fstype": "ext4", "mountpoint": "/var"}
         ]
      },
	  {"name": "sdc", "path": "/dev/sdc", "size": "2.0T", "type": "disk", "mountpoint": null,
         "children": [
			{"name": "sdc1", "path": "/dev/sdc1", "size": "512M",  "type": "part", "fstype": "vfat", "partlabel": "CLR_BOOT", "mountpoint": "/boot"},
            {"name": "sdc2", "path": "/dev/sdc2", "size": "256M",  "type": "part", "fstype": "swap", "partlabel": "CLR_SWAP", "mountpoint": null},
            {"name": "sdc3", "path": "/dev/sdc3", "size": "20G",   "type": "part", "fstype": "ext4", "partlabel": "CLR_ROOT", "mountpoint": "/"},
            {"name": "sdc4", "path": "/dev/sdc4", "size": "20G",   "type": "part", "fstype": "ext4", "partlabel": "CLR_MNT_/home", "mountpoint": "/home"}
         ]
      },
	  {"name": "sdd", "path": "/dev/sdd", "size": "2.0T", "type": "disk", "mountpoint": null,
         "children": [
			{"name": "sdd1", "path": "/dev/sdd1", "size": "512M",  "type": "part", "fstype": "vfat", "partlabel": "CLR_BOOT", "mountpoint": "/boot"},
            {"name": "sdd2", "path": "/dev/sdd2", "size": "20G",   "type": "part", "fstype": "ext4", "partlabel": "CLR_ROOT", "mountpoint": "/"},
            {"name": "sdd3", "path": "/dev/sdd3", "size": "20G",   "type": "part", "fstype": "ext4", "partlabel": "CLR_MNT_/home", "mountpoint": "/home"}
         ]
      }
   ]
}`

func TestPartitionValidation(t *testing.T) {
	medias, err := parseBlockDevicesDescriptor([]byte(lsblkOutput))
	if err != nil {
		t.Fatalf("Could not parser block device descriptor: %s", err)
	}

	var targets []*BlockDevice
	var mediaOpts MediaOpts

	resetWith := func(name string) {
		mediaOpts.SwapFileSize = ""
		mediaOpts.SwapFileSet = false
		mediaOpts.LegacyBios = false
		mediaOpts.SkipValidationSize = false
		mediaOpts.SkipValidationAll = false
		targets = []*BlockDevice{}

		for _, bd := range medias {
			if bd.Name == name {
				t.Logf("Found media %s", name)
				clone := bd.Clone()
				targets = append(targets, clone)
			}
		}
	}

	resetWith("sde")
	results := ServerValidatePartitions(targets, mediaOpts)
	if len(results) > 0 {
		for _, err := range results {
			t.Fatalf("ServerValidatePartitions returned error %q", err)
		}
	}

	resetWith("sdf")
	results = ServerValidatePartitions(targets, mediaOpts)
	if len(results) > 0 {
		for _, err := range results {
			t.Fatalf("ServerValidatePartitions returned error %q", err)
		}
	}

	resetWith("sdg")
	results = ServerValidateAdvancedPartitions(targets, mediaOpts)
	if len(results) > 0 {
		for _, err := range results {
			t.Fatalf("ServerValidatePartitions returned error %q", err)
		}
	}

	resetWith("sde")
	mediaOpts.SwapFileSize = "4G"
	mediaOpts.SwapFileSet = true
	results = ServerValidatePartitions(targets, mediaOpts)
	if len(results) > 0 {
		for _, err := range results {
			t.Fatalf("ServerValidatePartitions returned error %q", err)
		}
	}

	resetWith("sde")
	mediaOpts.SwapFileSize = "4.1G"
	mediaOpts.SwapFileSet = true
	mediaOpts.SkipValidationSize = true
	results = ServerValidatePartitions(targets, mediaOpts)
	if len(results) > 0 {
		for _, err := range results {
			t.Fatalf("ServerValidatePartitions returned error %q", err)
		}
	}

	resetWith("sde")
	mediaOpts.SwapFileSize = "8.1G"
	mediaOpts.SwapFileSet = true
	mediaOpts.SkipValidationSize = true
	results = ServerValidatePartitions(targets, mediaOpts)
	if cnt := len(results); cnt != 1 {
		t.Fatalf("ServerValidatePartitions returned %d errors, but should be 1", cnt)
	}

	resetWith("sde")
	mediaOpts.SwapFileSize = "8.1G"
	mediaOpts.SwapFileSet = true
	results = ServerValidatePartitions(targets, mediaOpts)
	if cnt := len(results); cnt != 3 {
		t.Fatalf("ServerValidatePartitions returned %d errors, but should be 3", cnt)
	}

	resetWith("sda")
	results = DesktopValidatePartitions(targets, mediaOpts)
	if len(results) > 0 {
		for _, err := range results {
			t.Fatalf("DesktopValidatePartitions returned error %q", err)
		}
	}

	resetWith("sdf")
	results = DesktopValidatePartitions(targets, mediaOpts)
	if cnt := len(results); cnt != 1 {
		t.Fatalf("DesktopValidatePartitions returned %d errors, but should be 1", cnt)
		if len(results) > 0 {
			for _, err := range results {
				t.Fatalf("DesktopValidatePartitions returned error %q", err)
			}
		}
	}

	resetWith("sdg")
	results = DesktopValidateAdvancedPartitions(targets, mediaOpts)
	if cnt := len(results); cnt != 1 {
		t.Fatalf("DesktopValidatePartitions returned %d errors, but should be 1", cnt)
		if len(results) > 0 {
			for _, err := range results {
				t.Fatalf("DesktopValidatePartitions returned error %q", err)
			}
		}
	}

	resetWith("sda")
	mediaOpts.SwapFileSize = "4G"
	mediaOpts.SwapFileSet = true
	results = DesktopValidatePartitions(targets, mediaOpts)
	if len(results) > 0 {
		for _, err := range results {
			t.Fatalf("DesktopValidatePartitions returned error %q", err)
		}
	}

	resetWith("sda")
	mediaOpts.SwapFileSize = "4.1G"
	mediaOpts.SwapFileSet = true
	mediaOpts.SkipValidationSize = true
	results = DesktopValidatePartitions(targets, mediaOpts)
	if len(results) > 0 {
		for _, err := range results {
			t.Fatalf("DesktopValidatePartitions returned error %q", err)
		}
	}

	resetWith("sda")
	mediaOpts.SwapFileSize = "20.1G"
	mediaOpts.SwapFileSet = true
	mediaOpts.SkipValidationSize = true
	results = DesktopValidatePartitions(targets, mediaOpts)
	if cnt := len(results); cnt != 1 {
		t.Fatalf("DesktopValidatePartitions returned %d errors, but should be 1", cnt)
	}

	resetWith("sda")
	mediaOpts.SwapFileSize = "20.1G"
	mediaOpts.SwapFileSet = true
	results = DesktopValidatePartitions(targets, mediaOpts)
	if cnt := len(results); cnt != 3 {
		t.Fatalf("DesktopValidatePartitions returned %d errors, but should be 3", cnt)
	}
}

func TestLegacyPartitionValidation(t *testing.T) {
	medias, err := parseBlockDevicesDescriptor([]byte(lsblkOutput))
	if err != nil {
		t.Fatalf("Could not parser block device descriptor: %s", err)
	}

	var targets []*BlockDevice
	var mediaOpts MediaOpts

	resetWith := func(name string) {
		mediaOpts.SwapFileSize = ""
		mediaOpts.SwapFileSet = false
		mediaOpts.LegacyBios = true
		mediaOpts.SkipValidationSize = false
		mediaOpts.SkipValidationAll = false
		targets = []*BlockDevice{}

		for _, bd := range medias {
			if bd.Name == name {
				t.Logf("Found media %s", name)
				clone := bd.Clone()
				targets = append(targets, clone)
			}
		}
	}

	resetWith("sde")
	results := ServerValidatePartitions(targets, mediaOpts)
	if len(results) > 0 {
		for _, err := range results {
			t.Fatalf("ServerValidatePartitions returned error %q", err)
		}
	}

	resetWith("sdf")
	results = ServerValidatePartitions(targets, mediaOpts)
	if len(results) > 0 {
		for _, err := range results {
			t.Fatalf("ServerValidatePartitions returned error %q", err)
		}
	}

	resetWith("sdh")
	mediaOpts.LegacyBios = false
	t.Logf("mediaOpts: %+v", mediaOpts)
	results = ServerValidateAdvancedPartitions(targets, mediaOpts)
	if cnt := len(results); cnt != 1 {
		t.Fatalf("ServerValidatePartitions returned %d errors, but should be 1", cnt)
		if len(results) > 0 {
			for _, err := range results {
				t.Fatalf("ServerValidatePartitions returned error %q", err)
			}
		}
	}

	resetWith("sdh")
	mediaOpts.LegacyBios = false
	mediaOpts.SkipValidationAll = true
	t.Logf("mediaOpts: %+v", mediaOpts)
	results = ServerValidateAdvancedPartitions(targets, mediaOpts)
	if cnt := len(results); cnt != 1 {
		t.Fatalf("ServerValidatePartitions returned %d errors, but should be 1", cnt)
		if len(results) > 0 {
			for _, err := range results {
				t.Fatalf("ServerValidatePartitions returned error %q", err)
			}
		}
	}

	resetWith("sdh")
	mediaOpts.SkipValidationAll = true
	t.Logf("mediaOpts: %+v", mediaOpts)
	results = ServerValidateAdvancedPartitions(targets, mediaOpts)
	t.Logf("results: %+v", results)
	if cnt := len(results); cnt > 0 {
		t.Fatalf("ServerValidatePartitions returned %d errors, but should be 0", cnt)
		if len(results) > 0 {
			for _, err := range results {
				t.Fatalf("ServerValidatePartitions returned error %q", err)
			}
		}
	}
}

func TestAdvancedPartitionValidation(t *testing.T) {
	medias, err := parseBlockDevicesDescriptor([]byte(lsblkOutput))
	if err != nil {
		t.Fatalf("Could not parser block device descriptor: %s", err)
	}

	var targets []*BlockDevice
	var mediaOpts MediaOpts

	resetWith := func(name string) {
		mediaOpts.SwapFileSize = ""
		mediaOpts.SwapFileSet = false
		mediaOpts.LegacyBios = false
		mediaOpts.SkipValidationSize = false
		mediaOpts.SkipValidationAll = false
		targets = []*BlockDevice{}

		for _, bd := range medias {
			if bd.Name == name {
				t.Logf("Found media %s", name)
				clone := bd.Clone()
				targets = append(targets, clone)
			}
		}
	}

	resetWith("sda")
	mediaOpts.SwapFileSize = "20.1G"
	mediaOpts.SwapFileSet = true
	results := DesktopValidatePartitions(targets, mediaOpts)
	if cnt := len(results); cnt != 3 {
		t.Fatalf("DesktopValidatePartitions returned %d errors, but should be 3", cnt)
	}

	resetWith("sdc")
	t.Logf("targets: %v", targets)
	advTargets := FindAdvancedInstallTargets(targets)
	t.Logf("advTargets: %v", advTargets)
	if !HasAdvancedSwap(advTargets) {
		t.Fatalf("HasAdvancedSwap should be true for device %q", "sdc")
	}
	resetWith("sdd")
	advTargets = FindAdvancedInstallTargets(targets)
	if HasAdvancedSwap(advTargets) {
		t.Fatalf("HasAdvancedSwap should be false for device %q", "sdd")
	}
}

func TestHumanReadableSize(t *testing.T) {
	tests := []struct {
		size      uint64
		unit      string
		precision int
		result    string
		err       error
	}{
		{102, "", -1, "102", nil},
		{1024, "", -1, "1KB", nil},
		{1854, "", -1, "1.9KB", nil},
		{1000000, "KB", -1, "1000KB", nil},
		{1000000, "KB", -1, "1000KB", nil},
		{1000000, "", -1, "1MB", nil},
		{1854, "MB", -1, "0MB", nil},
		{1000000000, "MB", -1, "1000MB", nil},
		{1000000000, "", -1, "1GB", nil},
		{1500000000, "", -1, "1.5GB", nil},
		{1570000000, "", -1, "1.57GB", nil},
		{1570000000, "", 2, "1.57GB", nil},
		{1570000000, "", 1, "1.6GB", nil},
		{1510000000, "", 1, "1.5GB", nil},
		{1000000000000, "", -1, "1TB", nil},
		{1400000000000, "", -1, "1.4TB", nil},
		{1450000000000, "", -1, "1.45TB", nil},
		{1459000000000, "", -1, "1.459TB", nil},
		{1459000000000, "TB", -1, "1.459TB", nil},
		{1400500000000, "", 3, "1.401TB", nil},
		{1400500000000, "", 2, "1.4TB", nil},
		{1451000000000, "", 1, "1.5TB", nil},
		{1440000000000, "", 1, "1.4TB", nil},
		{1459000000000, "", 2, "1.46TB", nil},
		{1459000000000, "GB", 2, "1459GB", nil},
		{1459470000000, "GB", 2, "1459.47GB", nil},
		{1000000000000000, "", -1, "1PB", nil},
		{1000000000000000, "PB", -1, "1PB", nil},
		{1080000000000000, "", -1, "1.08PB", nil},
		{1080000000000000, "", 1, "1.1PB", nil},
		{1081000000000000, "TB", 1, "1081TB", nil},
		{1081000000000000, "GB", 1, "1081000GB", nil},
	}

	for i, curr := range tests {
		value, err := HumanReadableSizeXBWithUnitAndPrecision(curr.size, curr.unit, curr.precision)
		if err != curr.err {
			t.Fatalf("TestHumanReadableSize-All %d: error %v did not match %v", i, err, curr.err)
		}
		if value != curr.result {
			t.Fatalf("TestHumanReadableSize-All %d: conversion %q did not match %q", i, value, curr.result)
		}

		if curr.unit == "" {
			value, err := HumanReadableSizeXBWithPrecision(curr.size, curr.precision)
			if err != curr.err {
				t.Fatalf("TestHumanReadableSize-Precision %d: error %v did not match %v", i, err, curr.err)
			}
			if value != curr.result {
				t.Fatalf("TestHumanReadableSize-Precision %d: conversion %q did not match %q", i, value, curr.result)
			}
		}

		if curr.precision == -1 {
			value, err := HumanReadableSizeXBWithUnit(curr.size, curr.unit)
			if err != curr.err {
				t.Fatalf("TestHumanReadableSize-Unit %d: error %v did not match %v", i, err, curr.err)
			}
			if value != curr.result {
				t.Fatalf("TestHumanReadableSize-Unit %d: conversion %q did not match %q", i, value, curr.result)
			}
		}

		if curr.unit == "" && curr.precision == -1 {
			value, err := HumanReadableSizeXB(curr.size)
			if err != curr.err {
				t.Fatalf("TestHumanReadableSize %d: error %v did not match %v", i, err, curr.err)
			}
			if value != curr.result {
				t.Fatalf("TestHumanReadableSize %d: conversion %q did not match %q", i, value, curr.result)
			}
		}
	}
}

func TestHumanReadableSizeXiB(t *testing.T) {
	tests := []struct {
		size      uint64
		unit      string
		precision int
		result    string
		err       error
	}{
		{102, "", -1, "102", nil},
		{999, "B", -1, "999", nil},
		{1000, "KiB", -1, "1KiB", nil},
		{1024, "", -1, "1KiB", nil},
		{1854, "", -1, "1.8KiB", nil},
		{1000000, "KiB", -1, "976.6KiB", nil},
		{1024000, "KiB", -1, "1000KiB", nil},
		{1048576, "", -1, "1MiB", nil},
		{1854, "MiB", -1, "0MiB", nil},
		{1048576000, "MiB", -1, "1000MiB", nil},
		{1073741824, "", -1, "1GiB", nil},
		{1610612736, "", -1, "1.5GiB", nil},
		{1685774664, "", -1, "1.57GiB", nil},
		{1685774664, "", 2, "1.57GiB", nil},
		{1685774664, "", 1, "1.6GiB", nil},
		{1612612736, "", 1, "1.5GiB", nil},
		{1099511627776, "", -1, "1TiB", nil},
		{1539316278886, "", -1, "1.4TiB", nil},
		{1594291860275, "", -1, "1.45TiB", nil},
		{1604187464925, "", -1, "1.459TiB", nil},
		{1604187464925, "TiB", -1, "1.459TiB", nil},
		{1540415790514, "", 3, "1.401TiB", nil},
		{1540415790514, "", 2, "1.4TiB", nil},
		{1604187464925, "GiB", 2, "1494.02GiB", nil},
		{1125899906842624, "", -1, "1PiB", nil},
		{1125899906842624, "PiB", -1, "1PiB", nil},
		{1215971899390034, "", -1, "1.08PiB", nil},
		{1215971899390034, "", 1, "1.1PiB", nil},
	}

	for i, curr := range tests {
		value, err := HumanReadableSizeXiBWithUnitAndPrecision(curr.size, curr.unit, curr.precision)
		if err != curr.err {
			t.Fatalf("TestHumanReadableSizeXiB-All %d: error %v did not match %v", i, err, curr.err)
		}
		if value != curr.result {
			t.Fatalf("TestHumanReadableSizeXiB-All %d: conversion %q did not match %q", i, value, curr.result)
		}

		if curr.unit == "" {
			value, err := HumanReadableSizeXiBWithPrecision(curr.size, curr.precision)
			if err != curr.err {
				t.Fatalf("TestHumanReadableSizeXiB-Precision %d: error %v did not match %v", i, err, curr.err)
			}
			if value != curr.result {
				t.Fatalf("TestHumanReadableSizeXiB-Precision %d: conversion %q did not match %q", i, value, curr.result)
			}
		}

		if curr.precision == -1 {
			value, err := HumanReadableSizeXiBWithUnit(curr.size, curr.unit)
			if err != curr.err {
				t.Fatalf("TestHumanReadableSizeXiB-Unit %d: error %v did not match %v", i, err, curr.err)
			}
			if value != curr.result {
				t.Fatalf("TestHumanReadableSizeXiB-Unit %d: conversion %q did not match %q", i, value, curr.result)
			}
		}

		if curr.unit == "" && curr.precision == -1 {
			value, err := HumanReadableSizeXiB(curr.size)
			if err != curr.err {
				t.Fatalf("TestHumanReadableSizeXiB %d: error %v did not match %v", i, err, curr.err)
			}
			if value != curr.result {
				t.Fatalf("TestHumanReadableSizeXiB %d: conversion %q did not match %q", i, value, curr.result)
			}
		}
	}
}
