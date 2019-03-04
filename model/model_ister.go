// Copyright © 2019 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package model

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/clearlinux/clr-installer/errors"
	"github.com/clearlinux/clr-installer/kernel"
	"github.com/clearlinux/clr-installer/keyboard"
	"github.com/clearlinux/clr-installer/language"
	"github.com/clearlinux/clr-installer/log"
	"github.com/clearlinux/clr-installer/network"
	"github.com/clearlinux/clr-installer/storage"
	"github.com/clearlinux/clr-installer/telemetry"
	"github.com/clearlinux/clr-installer/user"
)

// IsterConfig represents the install configuration used in the "ister" app
type IsterConfig struct {
	DestinationType      string                 `json:"DestinationType"`
	PartitionLayouts     []*PartitionLayout     `json:"PartitionLayout"`
	FilesystemTypes      []*FilesystemType      `json:"FilesystemTypes"`
	PartitionMountPoints []*PartitionMountPoint `json:"PartitionMountPoints"`
	Version              json.Number            `json:"Version"`
	Bundles              []string               `json:"Bundles"`
	Users                []*User                `json:"Users,omitempty,flow"`
	Hostname             string                 `json:"Hostname,omitempty,flow"`
	StaticIP             *Network               `json:"Static_IP,omitempty,flow"`
	PostNonChroot        []string               `json:"PostNonChroot,omitempty,flow"`
	PostNonChrootShell   []string               `json:"PostNonChrootShell,omitempty,flow"`
	LegacyBios           bool                   `json:"LegacyBios,omitempty,flow"`
	HTTPSProxy           string                 `json:"HTTPSProxy,omitempty,flow"`
	HTTPProxy            string                 `json:"HTTPProxy,omitempty,flow"`
	MirrorURL            string                 `json:"MirrorURL,omitempty,flow"`
	Cmdline              string                 `json:"cmdline,omitempty,flow"`
	VersionURL           string                 `json:"VersionURL,omitempty,flow"`
}

// PartitionLayout describes the partition type
type PartitionLayout struct {
	Disk      string `json:"disk"`
	Partition uint64 `json:"partition"`
	Size      string `json:"size"`
	Type      string `json:"type"`
}

// FilesystemType describes the filesystem type
type FilesystemType struct {
	Disk      string `json:"disk"`
	Partition uint64 `json:"partition"`
	Type      string `json:"type"`
	Options   string `json:"options"`
}

// PartitionMountPoint describes the mount point
type PartitionMountPoint struct {
	Disk      string `json:"disk"`
	Partition uint64 `json:"partition"`
	Mount     string `json:"mount"`
}

// User describes the user  details
type User struct {
	Username string      `json:"username"`
	Key      string      `json:"key"`
	UID      json.Number `json:"uid"`
	Sudo     bool        `json:"sudo"`
	Password string      `json:"password"`
}

// Network describes the network interface  details
type Network struct {
	Iface   string `json:"iface"`
	Address string `json:"address"`
	Gateway string `json:"gateway"`
	DNS     string `json:"dns"`
}

// JSONtoYAMLConfig converts the "ister"JSON config to the corresponding YAML config fields
// and writes it out to a YAML config file.
func JSONtoYAMLConfig(cf string) (string, error) {
	fp, err := os.Open(cf)
	if err != nil {
		return cf, errors.Wrap(err)
	}
	log.Debug("Successfully opened config file: %s", cf)
	defer func() {
		_ = fp.Close()
	}()

	b, err := ioutil.ReadAll(fp)
	if err != nil {
		return cf, errors.Wrap(err)
	}

	ic := IsterConfig{}
	err = json.Unmarshal(b, &ic)
	if err != nil {
		return cf, errors.Wrap(err)
	}

	si := SystemInstall{}

	var disks = make(map[string](map[uint64]storage.BlockDevice)) // Key: Disk name, Value: Map of Partitions

	// For each partition, set the Size
	for _, curr := range ic.PartitionLayouts {

		partitions, ok := disks[curr.Disk]
		if !ok {
			sa := StorageAlias{}
			bd := storage.BlockDevice{}
			switch ic.DestinationType {
			case "virtual":
				bd.Name = curr.Disk
				bd.Type = storage.BlockDeviceTypeLoop
				sa.Name = strings.TrimSuffix(curr.Disk, filepath.Ext(curr.Disk)) // remove any extensions from alias name
				sa.File = curr.Disk
			case "physical":
				bd.Name = curr.Disk
				bd.Type = storage.BlockDeviceTypeDisk
				sa.Name = strings.TrimSuffix(curr.Disk, filepath.Ext(curr.Disk)) // remove any extensions from alias name
				sa.File = "/dev/" + curr.Disk
			default:
				return cf, errors.Errorf("invalid DestinationType in config file %s", cf)
			}
			si.StorageAlias = append(si.StorageAlias, &sa)
			si.AddTargetMedia(&bd)

			var partitions = make(map[uint64]storage.BlockDevice)
			partitions[curr.Partition], err = setStorageValues(curr.Disk, curr.Partition, curr.Size)
			if err != nil {
				return cf, errors.Wrap(err)
			}
			disks[curr.Disk] = partitions
		} else {
			_, ok := partitions[curr.Partition]
			if ok {
				return cf, fmt.Errorf("partition %d already defined for disk %s in config file %s", curr.Partition, curr.Disk, cf)
			}
			partitions[curr.Partition], err = setStorageValues(curr.Disk, curr.Partition, curr.Size)
			if err != nil {
				return cf, errors.Wrap(err)
			}
			disks[curr.Disk] = partitions
		}
	}

	// For each partition, set the FsType and Options
	for _, curr := range ic.FilesystemTypes {
		partitions, ok := disks[curr.Disk]
		if !ok {
			return cf, errors.Errorf("disk %s not defined in config file %s", curr.Disk, cf)
		}

		part, ok := partitions[curr.Partition]
		if !ok {
			return cf, errors.Errorf("partition %d not defined for disk %s in config file %s", curr.Partition, curr.Disk, cf)
		}
		part.FsType = curr.Type
		part.Options = curr.Options

		partitions[curr.Partition] = part
		disks[curr.Disk] = partitions
	}

	// For each partition, set the MountPoint
	for _, curr := range ic.PartitionMountPoints {
		partitions, ok := disks[curr.Disk]
		if !ok {
			return cf, errors.Errorf("disk %s not defined in config file %s", curr.Disk, cf)
		}

		part, ok := partitions[curr.Partition]
		if !ok {
			return cf, errors.Errorf("partition %d not defined for partitions %s in config file %s", curr.Partition, curr.Disk, cf)
		}
		part.MountPoint = curr.Mount

		partitions[curr.Partition] = part
		disks[curr.Disk] = partitions
	}

	// For each disk, add the partitions as children elements
	for _, curr := range si.TargetMedias {
		partitions := disks[curr.Name]
		var children = make([]*storage.BlockDevice, len(partitions), len(partitions))
		i := 0
		for _, part := range partitions {
			e := storage.BlockDevice{}
			e = part
			children[i] = &e
			i++
		}
		curr.Children = children                                                        // Set Children
		curr.Name = "${" + strings.TrimSuffix(curr.Name, filepath.Ext(curr.Name)) + "}" // Update Name
		si.AddTargetMedia(curr)                                                         // Set TargetMedia
	}

	// Set Version. Any non numeric version (e.g. "latest") is set to 0
	v, err := ic.Version.Int64()
	if err == nil {
		si.Version = uint(v)
	} else {
		si.Version = 0
	}

	// Remove the first occurrence of a kernel bundle and set it as the Kernel
	for _, curr := range ic.Bundles {
		if strings.HasPrefix(curr, "kernel-") {
			if si.Kernel == nil {
				si.Kernel = &kernel.Kernel{Bundle: curr}
				continue
			}
		}
		si.Bundles = append(si.Bundles, curr) // Set Bundles
	}

	// Set Users
	for _, curr := range ic.Users {
		u := user.User{}
		u.Login = string(curr.UID)
		u.UserName = curr.Username
		u.Admin = curr.Sudo
		u.SSHKeys = append(u.SSHKeys, curr.Key)
		si.Users = append(si.Users, &u)
	}

	// Set KernelArguments
	if ic.Cmdline != "" {
		s := strings.Split(ic.Cmdline, " ")
		ka := kernel.Arguments{}
		ka.Add = s
		si.KernelArguments = &ka
	}

	// Set Hostname
	si.Hostname = ic.Hostname

	// Set NetworkInterfaces
	if ic.StaticIP != nil {
		nw := network.Interface{}
		nw.Name = ic.StaticIP.Iface
		nw.Gateway = ic.StaticIP.Gateway
		nw.DHCP = false //  always static IP

		s := strings.Split(ic.StaticIP.Address, "/")
		addr := network.Addr{}
		addr.IP, addr.NetMask = s[0], s[1]
		nw.Addrs = append(nw.Addrs, &addr)

		si.NetworkInterfaces = append(si.NetworkInterfaces, &nw)
	}

	// Set PostInstall with PostNonChroot
	for _, curr := range ic.PostNonChroot {
		pi := InstallHook{Chroot: false, Cmd: curr}
		si.PostInstall = append(si.PostInstall, &pi)
	}
	// Set PostInstall with PostNonChrootShell  prefixed with "/bin/bash -c "
	for _, curr := range ic.PostNonChrootShell {
		pi := InstallHook{Chroot: false, Cmd: "/bin/bash -c " + curr}
		si.PostInstall = append(si.PostInstall, &pi)
	}

	si.LegacyBios = ic.LegacyBios // Set LegacyBios
	si.SwupdMirror = ic.MirrorURL // Set SwupdMirror

	si.HTTPSProxy = ic.HTTPSProxy // Set HTTPSProxy
	if si.HTTPSProxy == "" {
		si.HTTPSProxy = ic.HTTPProxy
		fmt.Println("WARNING: Mapping HTTPProxy in json to HTTPSProxy in yaml")
		log.Warning("Mapping HTTPProxy in json to HTTPSProxy in yaml")
	} else {
		fmt.Println("WARNING: Skipping HTTPProxy mapping")
		log.Warning("Skipping HTTPProxy mapping")
	}

	// Hardcoding the missing required fields
	si.Telemetry = &telemetry.Telemetry{Enabled: false}              // Set Telemetry
	si.Keyboard = &keyboard.Keymap{Code: keyboard.DefaultKeyboard}   // Set Keyboard
	si.Language = &language.Language{Code: language.DefaultLanguage} // Set Language

	if ic.VersionURL != "" {
		fmt.Println("WARNING: Skipping VersionURL mapping as it not supported in clr-installer config")
		log.Warning("Skipping VersionURL mapping as it not supported in clr-installer config")
	}

	cf = strings.TrimSuffix(cf, filepath.Ext(cf)) + ".yaml"
	info, err := os.Stat(cf)
	if err != nil {
		if os.IsNotExist(err) {
			// File does not exist, skip backup
		} else {
			return cf, errors.Wrap(err)
		}
	} else { // Make backup
		mt := info.ModTime()
		suffix := fmt.Sprintf("-%d-%02d-%02d-%02d%02d%02d",
			mt.Year(), mt.Month(), mt.Day(),
			mt.Hour(), mt.Minute(), mt.Second())
		bf := strings.TrimSuffix(cf, filepath.Ext(cf)) + suffix + ".yaml"
		err = os.Rename(cf, bf)
		if err != nil {
			return cf, errors.Wrap(err)
		}
		fmt.Printf("WARNING: Config file %s already exists. Making a backup: %s\n", cf, bf)
		log.Warning("Config file %s already exists. Taking a backup: %s\n", cf, bf)
	}

	err = si.WriteFile(cf)
	if err != nil {
		return cf, errors.Wrap(err)
	}
	fmt.Println("Converted config file from JSON to YAML: " + cf)
	log.Info("Converted config file from JSON to YAML: " + cf)
	return cf, nil
}

// setStorageValues sets name, type and size of a BlockDevice
func setStorageValues(name string, part uint64, size string) (storage.BlockDevice, error) {
	var err error
	bd := storage.BlockDevice{}
	bd.Name = "${" + strings.TrimSuffix(name, filepath.Ext(name)) + "}" + strconv.FormatUint(part, 10)
	bd.Type = storage.BlockDeviceTypePart
	if size != "" {
		if size == "rest" {
			bd.Size = 0
		} else {
			bd.Size, err = storage.ParseVolumeSize(size)
			if err != nil {
				return bd, errors.Wrap(err)
			}
		}
	}
	return bd, nil
}
