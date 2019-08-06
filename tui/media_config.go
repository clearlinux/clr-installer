// Copyright © 2019 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/VladimirMarkelov/clui"
	term "github.com/nsf/termbox-go"
	"gopkg.in/yaml.v2"

	"github.com/clearlinux/clr-installer/log"
	"github.com/clearlinux/clr-installer/model"
	"github.com/clearlinux/clr-installer/storage"
	"github.com/clearlinux/clr-installer/utils"
)

const (
	mediaConfigMenuTitle = `Configure Installation Media`
	mediaConfigTitle     = `Select Installation Media`
	diskUtil             = `cgdisk`
)

// MediaConfigPage is the Page implementation for the disk partitioning menu page
type MediaConfigPage struct {
	BasePage

	safeRadio             *clui.Radio
	destructiveRadio      *clui.Radio
	advancedRadio         *clui.Radio
	group                 *clui.RadioGroup
	isSafeSelected        bool
	isDestructiveSelected bool
	isAdvancedSelected    bool

	chooserList   *clui.ListBox
	listBackColor term.Attribute
	listTextColor term.Attribute

	safeTargets        []storage.InstallTarget
	destructiveTargets []storage.InstallTarget

	labelWarning     *clui.Label
	labelDestructive *clui.Label

	encryptCheck *clui.CheckBox

	advancedCfgBtn *SimpleButton

	devs         []*storage.BlockDevice
	activeDisk   *storage.BlockDevice
	activeSerial string
}

// GetConfiguredValue Returns the string representation of currently value set
func (page *MediaConfigPage) GetConfiguredValue() string {
	if page.isAdvancedSelected {
		results := storage.ValidateAdvancedPartitions(page.getModel().TargetMedias)
		if len(results) > 0 {
			return fmt.Sprintf("Advanced: %s", strings.Join(results, ", "))
		}
		return fmt.Sprintf("Advanced: %s", strings.Join(storage.GetAdvancedPartitions(page.getModel().TargetMedias), ", "))
	}

	tm := page.getModel().TargetMedias
	if len(tm) == 0 {
		return "No -media- selected"
	} else if len(tm) > 1 {
		log.Warning("Too many media found, one 1 supported: %+v", tm)
		return "Too many media found"
	}

	bd := tm[0]
	target := page.getModel().InstallSelected[bd.Name]
	portion := storage.FormatInstallPortion(target)

	// Size string
	size, _ := storage.HumanReadableSizeWithPrecision(target.FreeEnd-target.FreeStart, 1)

	encrypted := ""
	for _, ch := range bd.Children {
		if ch.Type == storage.BlockDeviceTypeCrypt {
			encrypted = " Encryption"
		}
	}

	return fmt.Sprintf("%s (%s) %s%s %s", target.Friendly, target.Name, portion, encrypted, size)
}

// GetConfigDefinition returns if the config was interactively defined by the user,
// was loaded from a config file or if the config is not set.
func (page *MediaConfigPage) GetConfigDefinition() int {
	if len(page.safeTargets) == 0 && len(page.destructiveTargets) == 0 {
		if err := page.buildMediaLists(); err != nil {
			page.Panic(err)
		}
	}

	tm := page.getModel().TargetMedias

	if tm == nil {
		return ConfigNotDefined
	}

	for _, bd := range tm {
		if !bd.IsUserDefined() {
			return ConfigDefinedByConfig
		}

		for _, ch := range bd.Children {
			if !ch.IsUserDefined() {
				return ConfigDefinedByConfig
			}
		}
	}

	return ConfigDefinedByUser
}

// SetDone sets the configured disk into the model and sets the page as done
func (page *MediaConfigPage) SetDone(done bool) bool {
	var installBlockDevice *storage.BlockDevice

	if page.safeRadio.Selected() {
		page.getModel().ClearInstallSelected()
		selected := page.safeTargets[page.chooserList.SelectedItem()]
		page.getModel().InstallSelected[selected.Name] = selected
		log.Debug("Safe Install Target %v", page.getModel().InstallSelected)
		page.getModel().TargetMedias = nil
	} else if page.destructiveRadio.Selected() {
		page.getModel().ClearInstallSelected()
		selected := page.destructiveTargets[page.chooserList.SelectedItem()]
		page.getModel().InstallSelected[selected.Name] = selected
		log.Debug("Destructive Install Target %v", page.getModel().InstallSelected)
		page.getModel().TargetMedias = nil
	} else {
		log.Warning("Failed to find and save the selected installation media")
	}

	if page.advancedRadio.Selected() {
		log.Debug("Advanced Install Confirmed")
	} else {
		bds, err := storage.ListAvailableBlockDevices(page.getModel().TargetMedias)
		if err != nil {
			log.Error("Failed to find storage media for install during save: %s", err)
		}

		for _, selected := range page.getModel().InstallSelected {
			for _, curr := range bds {

				if curr.Name == selected.Name {
					installBlockDevice = curr.Clone()
					// Using the whole disk
					if selected.WholeDisk {
						storage.NewStandardPartitions(installBlockDevice)
					} else {
						// Partial Disk, Add our partitions
						size := selected.FreeEnd - selected.FreeStart
						size = size - storage.AddBootStandardPartition(installBlockDevice)
						if !installBlockDevice.DeviceHasSwap() {
							size = size - storage.AddSwapStandardPartition(installBlockDevice)
						}
						storage.AddRootStandardPartition(installBlockDevice, size)
					}
					page.getModel().AddTargetMedia(installBlockDevice)
					break
				}
			}
		}

		if page.encryptCheck.State() != 0 {
			for _, child := range installBlockDevice.Children {
				if child.MountPoint == "/" {
					child.Type = storage.BlockDeviceTypeCrypt
				}
			}
		}
	}

	// TODO start using new API page.GotoPage() when finished merging
	// disk pages
	page.tui.gotoPage(TuiPageMenu, page)

	return false
}

// Activate updates the UI elements with the most current list of block devices
func (page *MediaConfigPage) Activate() {

	page.confirmBtn.SetEnabled(false)
	page.encryptCheck.SetEnabled(true)

	if len(page.safeTargets) == 0 && len(page.destructiveTargets) == 0 {
		if err := page.buildMediaLists(); err != nil {
			page.Panic(err)
		}
	}

	if page.isSafeSelected {
		page.activated = page.safeRadio
	} else if page.isDestructiveSelected {
		page.activated = page.destructiveRadio
	} else if page.isAdvancedSelected {
		page.activated = page.advancedRadio
		page.encryptCheck.SetEnabled(false)
	} else {
		page.activated = page.cancelBtn
	}

	clui.RefreshScreen()
}

func (page *MediaConfigPage) setConfirmButton() {
	if page.labelWarning.Title() == "" {
		page.confirmBtn.SetEnabled(true)
	} else {
		page.confirmBtn.SetEnabled(false)
	}
}

// The disk page gives the user the option so select how to set the storage device,
// if to manually configure it or a guided standard partition schema
func newMediaConfigPage(tui *Tui) (Page, error) {
	page := &MediaConfigPage{
		BasePage: BasePage{
			// Tag this Page as required to be complete for the Install to proceed
			required: true,
		},
	}
	page.setupMenu(tui, TuiPageMediaConfig, mediaConfigMenuTitle, CancelButton|ConfirmButton, TuiPageMenu)

	// Top label for the page
	lbl := clui.CreateLabel(page.content, 2, 1, mediaConfigTitle, clui.Fixed)
	lbl.SetPaddings(0, 1)

	contentFrame := clui.CreateFrame(page.content, AutoSize, AutoSize, BorderNone, Fixed)
	contentFrame.SetPack(clui.Vertical)

	// Installation Type Radio button selection
	radioButtonFrame := clui.CreateFrame(contentFrame, AutoSize, AutoSize, BorderNone, Fixed)
	radioButtonFrame.SetPack(clui.Vertical)
	radioButtonFrame.SetPaddings(2, 1)

	page.group = clui.CreateRadioGroup()
	radioLabel := fmt.Sprintf("Safe Installation")
	page.safeRadio = clui.CreateRadio(radioButtonFrame, 50, radioLabel, AutoSize)
	page.safeRadio.SetStyle("Media")
	page.safeRadio.SetPack(clui.Horizontal)
	page.group.AddItem(page.safeRadio)
	page.safeRadio.OnChange(func(active bool) {
		if active {
			page.labelWarning.SetTitle("")
			page.labelDestructive.SetTitle("")
			page.isDestructiveSelected = false
			page.isAdvancedSelected = false
			page.encryptCheck.SetEnabled(true)
			page.advancedCfgBtn.SetEnabled(false)
		}
		// Disable the Confirm Button if we toggled
		if !page.isSafeSelected && active {
			page.isSafeSelected = true
			page.confirmBtn.SetEnabled(false)

			page.buildChooserList()
		}

		if active {
			if len(page.safeTargets) < 1 {
				warning := "No media or space available for installation"
				log.Warning(warning)
				warning = fmt.Sprintf("Warning: %s", warning)
				page.labelWarning.SetTitle(warning)
			}
		}
	})
	// Description of Safe
	descLabel := clui.CreateLabel(radioButtonFrame, 1, 1,
		"Install on an unallocated disk or alongside existing partitions.", Fixed)
	descLabel.SetMultiline(true)

	radioLabel = fmt.Sprintf("Destructive Installation")
	page.destructiveRadio = clui.CreateRadio(radioButtonFrame, 50, radioLabel, AutoSize)
	page.destructiveRadio.SetStyle("Media")
	page.destructiveRadio.SetPack(clui.Horizontal)
	page.group.AddItem(page.destructiveRadio)
	page.destructiveRadio.OnChange(func(active bool) {
		if active {
			page.labelWarning.SetTitle("")
			page.labelDestructive.SetTitle("")
			page.isSafeSelected = false
			page.isAdvancedSelected = false
			page.encryptCheck.SetEnabled(true)
			page.advancedCfgBtn.SetEnabled(false)
		}
		// Disable the Confirm Button if we toggled
		if !page.isDestructiveSelected && active {
			page.isDestructiveSelected = true
			page.confirmBtn.SetEnabled(false)

			page.buildChooserList()
		}

		if active {
			if len(page.devs) < 1 {
				warning := "No media found for installation"
				log.Warning(warning)
				warning = fmt.Sprintf("Warning: %s", warning)
				page.labelWarning.SetTitle(warning)
			} else {
				page.labelDestructive.SetTitle(storage.DestructiveWarning)
			}
		}
	})
	// Description of Destructive
	clui.CreateLabel(radioButtonFrame, 1, 1, "Erase all data on selected media and install Clear Linux* OS.", Fixed)

	radioLabel = fmt.Sprintf("Advanced Installation")
	page.advancedRadio = clui.CreateRadio(radioButtonFrame, 50, radioLabel, AutoSize)
	page.advancedRadio.SetStyle("Media")
	page.advancedRadio.SetPack(clui.Horizontal)
	page.group.AddItem(page.advancedRadio)
	page.advancedRadio.OnChange(func(active bool) {
		if active {
			page.labelWarning.SetTitle("")
			page.labelDestructive.SetTitle("")
			page.isSafeSelected = false
			page.isDestructiveSelected = false
			page.encryptCheck.SetEnabled(false)
			page.encryptCheck.SetState(0)
			page.advancedCfgBtn.SetEnabled(true)
		}

		// Disable the Confirm Button if we toggled
		if !page.isAdvancedSelected && active {
			page.isAdvancedSelected = true

			if err := page.buildMediaLists(); err != nil {
				page.Panic(err)
			}
		}

		if active {
			if len(page.devs) < 1 {
				warning := "No media found for installation"
				log.Warning(warning)
				warning = fmt.Sprintf("Warning: %s", warning)
				page.labelWarning.SetTitle(warning)
			} else {
				si := page.getModel()
				/*
					if storage.AdvancedPartitionsRequireEncryption(si.TargetMedias) && si.CryptPass == "" {
						if dialog, err := CreateEncryptPassphraseDialogBox(si); err == nil {
							dialog.OnClose(func() {
								if dialog.Confirmed {
									page.encryptCheck.SetState(1)
								} else {
									page.encryptCheck.SetState(0)
								}
							})
						}
					}
				*/
				results := storage.ValidateAdvancedPartitions(si.TargetMedias)
				if len(results) > 0 {
					page.confirmBtn.SetEnabled(false)
					warning := strings.Join(results, ", ")
					warning = fmt.Sprintf("Advanced: %s", warning)
					// Truncate long messages
					max, _ := page.labelWarning.Size()
					if len(warning) > max {
						warning = warning[0:max-3] + "..."
					}
					page.labelWarning.SetTitle(warning)
				} else {
					page.confirmBtn.SetEnabled(true)
				}
			}
		}
	})
	// Description of Advanced
	clui.CreateLabel(radioButtonFrame, 1, 1, "User selected media via partition labels.", Fixed)

	listFrame := clui.CreateFrame(contentFrame, 60, 4, BorderNone, Fixed)
	listFrame.SetPack(clui.Vertical)

	page.chooserList = clui.CreateListBox(listFrame, 60, 3, Fixed)
	page.chooserList.SetAlign(AlignLeft)
	page.chooserList.SetStyle("List")
	page.listBackColor = page.chooserList.BackColor()
	page.listTextColor = page.chooserList.TextColor()

	page.chooserList.OnActive(func(active bool) {
		if active {
			page.chooserList.SetStyle("ListActive")
			page.setConfirmButton()
		} else {
			page.chooserList.SetStyle("List")
		}
	})

	page.chooserList.OnKeyPress(func(k term.Key) bool {
		if k == term.KeyEnter {
			if page.confirmBtn != nil {
				page.confirmBtn.ProcessEvent(clui.Event{Type: clui.EventKey, Key: k})
			}
			return true
		}

		return false
	})

	// Warning label
	page.labelWarning = clui.CreateLabel(contentFrame, 1, 1, "", Fixed)
	page.labelWarning.SetBackColor(errorLabelBg)
	page.labelWarning.SetTextColor(errorLabelFg)

	// Destructive label
	page.labelDestructive = clui.CreateLabel(contentFrame, 1, 1, "", Fixed)
	page.labelDestructive.SetBackColor(errorLabelBg)
	page.labelDestructive.SetTextColor(errorLabelFg)

	// Encryption Checkbox
	page.encryptCheck = clui.CreateCheckBox(contentFrame, AutoSize, "Enable Encryption", AutoSize)
	page.encryptCheck.OnChange(func(state int) {
		if state != 0 {
			if dialog, err := CreateEncryptPassphraseDialogBox(page.getModel()); err == nil {
				dialog.OnClose(func() {
					if !dialog.Confirmed {
						page.encryptCheck.SetState(0)
					}
				})
			}
		}
	})

	// Add a Rescan media button
	rescanBtn := CreateSimpleButton(page.cFrame, AutoSize, AutoSize, "Rescan Media", Fixed)
	rescanBtn.OnClick(func(ev clui.Event) {
		var err error
		page.devs, err = storage.RescanBlockDevices(page.getModel().TargetMedias)
		if err != nil {
			page.Panic(err)
		}

		if err := page.buildMediaLists(); err != nil {
			page.Panic(err)
		}

		// Check if the active device is still present
		var found bool
		for _, bd := range page.devs {
			if bd.Serial == page.activeSerial {
				found = true
				page.activeDisk = bd
			}
		}
		if !found {
			page.activeSerial = ""
			page.activeDisk = nil
			page.getModel().TargetMedias = nil
		}

		page.GotoPage(TuiPageMediaConfig)
	})

	// Add an Advanced Configuration  button
	page.advancedCfgBtn = CreateSimpleButton(page.cFrame, AutoSize, AutoSize, "Partition", Fixed)
	page.advancedCfgBtn.OnClick(func(ev clui.Event) {
		log.Debug("%s diskUtil called for Target %v", diskUtil,
			page.destructiveTargets[page.chooserList.SelectedItem()])
		page.rundiskUtil(page.destructiveTargets[page.chooserList.SelectedItem()].Name)
	})

	if len(page.safeTargets) == 0 && len(page.destructiveTargets) == 0 {
		if err := page.buildMediaLists(); err != nil {
			page.Panic(err)
		}
	}

	page.activated = page.backBtn

	page.setConfirmButton()

	return page, nil
}

func (page *MediaConfigPage) buildChooserList() {
	clui.WindowManager().BeginUpdate()
	defer clui.WindowManager().EndUpdate()
	page.chooserList.Clear()
	page.chooserList.SetBackColor(page.listBackColor)
	page.chooserList.SetTextColor(page.listTextColor)

	found := false

	if page.isAdvancedSelected {
		for _, target := range page.destructiveTargets {
			target.Advanced = true
			page.chooserList.AddItem(fmtInstallTarget(target))
			found = true
		}
	} else if page.isSafeSelected {
		for _, target := range page.safeTargets {
			page.chooserList.AddItem(fmtInstallTarget(target))
			found = true
		}
	} else if page.isDestructiveSelected {
		for _, target := range page.destructiveTargets {
			page.chooserList.AddItem(fmtInstallTarget(target))
			found = true
		}
		page.chooserList.SetBackColor(errorLabelBg)
		page.chooserList.SetTextColor(errorLabelFg)
	} else {
		log.Warning("buildChooserList: unknown radio button state")
	}

	if found {
		page.chooserList.SelectItem(0)
	}
}

// buildMediaLists is used to create the valid chooser lists for Safe and
// Destructive Media choices. Also scans for Advanced Media configurations.
func (page *MediaConfigPage) buildMediaLists() error {
	page.labelWarning.SetTitle("")
	page.labelDestructive.SetTitle("")

	var err error
	page.devs, err = storage.ListAvailableBlockDevices(page.getModel().TargetMedias)
	if err != nil {
		page.Panic(err)
	}

	page.safeTargets = storage.FindSafeInstallTargets(storage.MinimumServerInstallSize, page.devs)
	page.destructiveTargets = storage.FindAllInstallTargets(page.devs)
	// Hook for searching CLR_*?
	model := page.getModel()
	model.TargetMedias = nil
	for _, curr := range storage.FindAdvancedInstallTargets(page.devs) {
		model.AddTargetMedia(curr)
		log.Debug("AddTargetMedia %+v", curr)
		model.InstallSelected[curr.Name] = storage.InstallTarget{Name: curr.Name, Friendly: curr.Model,
			Removable: curr.RemovableDevice}
		page.isAdvancedSelected = true
	}

	if page.isAdvancedSelected {
		if !page.group.SelectItem(page.advancedRadio) {
			log.Warning("Could not select the advanced install radio button")
		}
		page.isAdvancedSelected = true
	} else {
		if len(page.safeTargets) > 0 {
			if !page.group.SelectItem(page.safeRadio) {
				log.Warning("Could not select the safe install radio button")
			}
			page.isSafeSelected = true
		} else {
			if !page.group.SelectItem(page.destructiveRadio) {
				log.Warning("Could not select the destructive install radio button")
			}
			page.isDestructiveSelected = true
		}
	}

	page.buildChooserList()

	if page.isAdvancedSelected {
		log.Debug("Found Advanced partitions")
	}

	clui.RefreshScreen()

	return nil
}

func (page *MediaConfigPage) rundiskUtil(disk string) {

	stdMsg := "Could not launch " + diskUtil + ". Check A " + log.GetLogFileName()
	msg := ""

	// We need to save the current state for the relaunch of clr-installer
	tmpYaml, err := ioutil.TempFile("", "clr-installer-diskUtil-*.yaml")
	if err != nil {
		log.Warning("Could not make YAML tempfile: %v", err)
		msg = stdMsg
	}

	scrubbed := scrubModel(page.getModel())

	if saveErr := scrubbed.WriteFile(tmpYaml.Name()); saveErr != nil {
		log.Warning("Could not save config to %s %s", tmpYaml.Name(), msg)
		msg = stdMsg
	}

	tmpBash, err := ioutil.TempFile("", "clr-installer-diskUtil-*.sh")
	if err != nil {
		log.Warning("Could not make BASH tempfile: %v", err)
		msg = stdMsg
	}

	drive := filepath.Join("/dev", disk)
	exists, err := utils.FileExists(drive)
	if err != nil {
		log.Warning("Failed to check drive %s: %v", drive, err)
		msg = stdMsg
	} else if !exists {
		log.Warning("Request drive %s does not exist", drive)
		msg = stdMsg
	}

	var content bytes.Buffer
	_, _ = fmt.Fprintf(&content, "#!/bin/bash\n")
	// To ensure another instance is not launched, first recreate the
	// the installer lock file using the PID of the running script
	lockFile := page.getModel().LockFile
	_, _ = fmt.Fprintf(&content, "echo $$ > %s\n", lockFile)
	_, _ = fmt.Fprintf(&content, "echo Switching to %s %s\n", diskUtil, drive)
	_, _ = fmt.Fprintf(&content, "sleep 2\n")
	_, _ = fmt.Fprintf(&content, "/usr/bin/%s %s\n", diskUtil, drive)
	_, _ = fmt.Fprintf(&content, "sleep 1\n")
	_, _ = fmt.Fprintf(&content, "echo Checking partitions with partprobe %s\n", drive)
	_, _ = fmt.Fprintf(&content, "/usr/bin/partprobe %s\n", drive)
	_, _ = fmt.Fprintf(&content, "sleep 1\n")
	_, _ = fmt.Fprintf(&content, "/bin/rm %s %s\n", tmpBash.Name(), lockFile)
	_, _ = fmt.Fprintf(&content, "echo Restarting Clear Linux OS Installer ...\n")
	_, _ = fmt.Fprintf(&content, "sleep 2\n")
	args := append(os.Args, "--config", tmpYaml.Name(), "--tui")
	allArgs := strings.Join(args, " ")
	_, err = fmt.Fprintf(&content, "exec %s", allArgs)
	if err != nil {
		log.Warning("Could not write BASH buffer: %v", err)
		msg = stdMsg
	}
	if _, err := tmpBash.Write(content.Bytes()); err != nil {
		log.Warning("Could not write BASH tempfile: %v", err)
		msg = stdMsg
	}
	_ = tmpBash.Close()
	_ = os.Chmod(tmpBash.Name(), 0700)

	if msg != "" {
		if _, err := CreateWarningDialogBox(msg); err != nil {
			log.Warning("Attempt to launch %s: warning dialog failed: %s", diskUtil, err)
		}
		return
	}

	// We will NEVER return from this function
	clui.Stop()
	clui.DeinitLibrary()
	term := os.Getenv("TERM")
	err = syscall.Exec("/bin/bash", []string{"/bin/bash", "-l", "-c", tmpBash.Name()}, []string{"TERM=" + term})
	if err != nil {
		log.Warning("Could not start disk utility: %v", err)
		msg = stdMsg
		if _, err := CreateWarningDialogBox(msg); err != nil {
			log.Warning("Attempt to launch %s: warning dialog failed: %s", diskUtil, err)
		}
		return
	}
}

func scrubModel(md *model.SystemInstall) *model.SystemInstall {
	// Sanitized the model to remove meida
	var cleanModel model.SystemInstall
	// Marshal current into bytes
	confBytes, bytesErr := yaml.Marshal(md)
	if bytesErr != nil {
		log.Error("Failed to generate a copy of YAML data for %s (%v)", diskUtil, bytesErr)
		return nil
	}
	// Unmarshal into a copy
	if yamlErr := yaml.Unmarshal(confBytes, &cleanModel); yamlErr != nil {
		log.Error("Failed to duplicate YAML data for %s (%v)", diskUtil, bytesErr)
		return nil
	}
	// Sanitize the config data to remove any potential
	// Remove the target media
	cleanModel.TargetMedias = nil

	return &cleanModel
}

func fmtInstallTarget(target storage.InstallTarget) string {
	portion := storage.FormatInstallPortion(target)

	// Size string
	size, _ := storage.HumanReadableSizeWithPrecision(target.FreeEnd-target.FreeStart, 1)

	return fmt.Sprintf("%-32s  %10s  %-14s  %8s", target.Friendly, target.Name, portion, size)
}
