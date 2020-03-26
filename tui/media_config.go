// Copyright © 2020 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/VladimirMarkelov/clui"
	term "github.com/nsf/termbox-go"

	"github.com/clearlinux/clr-installer/log"
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
	saveRadio             *clui.Radio
	group                 *clui.RadioGroup
	isSafeSelected        bool
	isDestructiveSelected bool
	isAdvancedSelected    bool

	chooserList   *clui.ListBox
	listBackColor term.Attribute
	listTextColor term.Attribute

	safeTargets        []storage.InstallTarget
	destructiveTargets []storage.InstallTarget

	saveSelected map[string]storage.InstallTarget
	saveMedias   []*storage.BlockDevice

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
	model := page.getModel()
	tm := model.TargetMedias
	page.done = page.getModel().TargetMedias != nil

	if page.isAdvancedSelected {
		results := storage.ServerValidateAdvancedPartitions(tm, model.LegacyBios, model.SkipValidationSize)
		if len(results) > 0 {
			return fmt.Sprintf("Warning: %s", strings.Join(results, ", "))
		}
		if storage.AdvancedPartitionsRequireEncryption(tm) && model.CryptPass == "" {
			return fmt.Sprintf("Warning: %s", "Encryption passphrase required")
		}
		return fmt.Sprintf("Advanced: %s", strings.Join(storage.GetAdvancedPartitions(tm), ", "))
	}

	if len(tm) == 0 {
		return "No -media- selected"
	} else if len(tm) > 1 {
		log.Warning("Too many media found, only 1 supported: %+v", tm)
		return "Too many media found"
	}

	bd := tm[0]
	target := model.InstallSelected[bd.Name]
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

	si := page.getModel()
	tm := si.TargetMedias

	if tm == nil {
		return ConfigNotDefined
	}

	if page.isAdvancedSelected {
		results := storage.ServerValidateAdvancedPartitions(tm, si.LegacyBios, si.SkipValidationSize)
		if len(results) > 0 {
			model := page.getModel()
			model.ClearInstallSelected()
			model.TargetMedias = nil
			return ConfigNotDefined
		}
		return ConfigDefinedByUser
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

	page.done = page.getModel().TargetMedias != nil

	// TODO start using new API page.GotoPage() when finished merging
	// disk pages
	page.tui.gotoPage(TuiPageMenu, page)

	return false
}

// Activate updates the UI elements with the most current list of block devices
func (page *MediaConfigPage) Activate() {
	si := page.getModel()

	page.saveSelected = map[string]storage.InstallTarget{}
	for k, v := range si.InstallSelected {
		page.saveSelected[k] = v
	}
	page.saveMedias = append([]*storage.BlockDevice{}, si.TargetMedias...)

	page.confirmBtn.SetEnabled(false)
	page.encryptCheck.SetEnabled(true)

	if len(page.safeTargets) == 0 && len(page.destructiveTargets) == 0 {
		if err := page.buildMediaLists(); err != nil {
			page.Panic(err)
		}
	}

	advEncryption := storage.AdvancedPartitionsRequireEncryption(si.TargetMedias)

	if page.isSafeSelected {
		page.activated = page.safeRadio
		page.saveRadio = page.safeRadio
	} else if page.isDestructiveSelected {
		page.activated = page.destructiveRadio
		page.saveRadio = page.destructiveRadio
	} else if page.isAdvancedSelected {
		page.activated = page.advancedRadio
		page.saveRadio = page.advancedRadio
		if !advEncryption {
			page.encryptCheck.SetEnabled(false)
		}
	} else {
		page.activated = page.cancelBtn
	}

	clui.RefreshScreen()

	if page.isAdvancedSelected {
		if advEncryption && si.CryptPass == "" {
			page.encryptCheck.SetState(1)
		}
	}
}

// DeActivate will reset the selection case the user has pressed cancel
func (page *MediaConfigPage) DeActivate() {
	log.Debug("DeActivate media")
	if page.action != ActionCancelButton {
		return
	}

	log.Debug("DeActivate: page.action: %+v", page.action)

	// The starting start is not the selected state
	// We changed the active button, but then canceled
	if page.saveRadio != nil && !page.saveRadio.Selected() {
		si := page.getModel()
		si.InstallSelected = map[string]storage.InstallTarget{}
		for k, v := range page.saveSelected {
			si.InstallSelected[k] = v
		}
		si.TargetMedias = append([]*storage.BlockDevice{}, page.saveMedias...)

		log.Debug("media choice toggle, but we canceled")
		page.group.SelectItem(page.saveRadio)
	}
}

func (page *MediaConfigPage) setConfirmButton() {
	if page.labelWarning.BackColor() == errorLabelBg &&
		page.labelWarning.TextColor() == errorLabelFg {
		if page.labelWarning.Title() == "" {
			page.confirmBtn.SetEnabled(true)
		} else {
			page.confirmBtn.SetEnabled(false)
		}
	} else {
		page.confirmBtn.SetEnabled(true)
	}
}

func (page *MediaConfigPage) safeRadioOnChange(active bool) {
	if !active {
		return
	}

	page.labelWarning.SetTitle("")
	page.labelDestructive.SetTitle("")
	page.isDestructiveSelected = false
	page.isAdvancedSelected = false
	page.encryptCheck.SetEnabled(true)
	page.advancedCfgBtn.SetEnabled(false)

	// Disable the Confirm Button if we toggled
	if !page.isSafeSelected {
		page.isSafeSelected = true
		page.confirmBtn.SetEnabled(false)

		page.buildChooserList()
	}

	if len(page.safeTargets) < 1 || len(page.safeTargets) == 0 {
		warning := "No media or space available for installation"
		log.Warning(warning)
		warning = fmt.Sprintf("Warning: %s", warning)
		page.labelWarning.SetTitle(warning)
	}
}

func (page *MediaConfigPage) destructiveRadioOnChange(active bool) {
	if !active {
		return
	}

	page.labelWarning.SetTitle("")
	page.labelDestructive.SetTitle("")
	page.isSafeSelected = false
	page.isAdvancedSelected = false
	page.encryptCheck.SetEnabled(true)
	page.advancedCfgBtn.SetEnabled(false)

	// Disable the Confirm Button if we toggled
	if !page.isDestructiveSelected {
		page.isDestructiveSelected = true
		page.confirmBtn.SetEnabled(false)

		page.buildChooserList()
	}

	if len(page.devs) < 1 || len(page.destructiveTargets) == 0 {
		warning := "No media found for installation"
		log.Warning(warning)
		warning = fmt.Sprintf("Warning: %s", warning)
		page.labelWarning.SetTitle(warning)
	} else {
		page.labelDestructive.SetTitle(storage.DestructiveWarning)
	}
}

func (page *MediaConfigPage) advancedRadioOnChange(active bool) {
	if !active {
		return
	}

	page.labelWarning.SetTitle("")
	page.labelDestructive.SetTitle("")
	page.isSafeSelected = false
	page.isDestructiveSelected = false

	page.encryptCheck.SetEnabled(storage.AdvancedPartitionsRequireEncryption(page.getModel().TargetMedias))
	page.encryptCheck.SetState(0) // Force off for Advance as not support yet

	page.advancedCfgBtn.SetEnabled(true)

	// Disable the Confirm Button if we toggled
	if !page.isAdvancedSelected {
		page.isAdvancedSelected = true

		if err := page.buildMediaLists(); err != nil {
			page.Panic(err)
		}
	}

	if len(page.devs) < 1 {
		warning := "No media found for installation"
		log.Warning(warning)
		warning = fmt.Sprintf("Warning: %s", warning)
		page.labelWarning.SetTitle(warning)
		page.advancedCfgBtn.SetEnabled(false)
	} else {
		si := page.getModel()
		results := storage.ServerValidateAdvancedPartitions(si.TargetMedias, si.LegacyBios, si.SkipValidationSize)
		warning := ""
		if len(results) > 0 {
			warning = strings.Join(results, ", ")
			warning = fmt.Sprintf("Warning: %s", warning)
			page.labelWarning.SetBackColor(errorLabelBg)
			page.labelWarning.SetTextColor(errorLabelFg)
		} else {
			// No warning, so re-use the label to show what is configured
			warning = page.GetConfiguredValue()
			page.labelWarning.SetBackColor(infoLabelBg)
			page.labelWarning.SetTextColor(infoLabelFg)
		}
		if len(warning) > 0 {
			// Truncate long messages
			max, _ := page.labelWarning.Size()
			if len(warning) > max {
				warning = warning[0:max-3] + "..."
			}
			page.labelWarning.SetTitle(warning)
		}
		page.setConfirmButton()
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
	page.safeRadio.OnChange(page.safeRadioOnChange)
	// Description of Safe
	descLabel := clui.CreateLabel(radioButtonFrame, 1, 1,
		"Install on an unallocated disk or alongside existing partitions.", Fixed)
	descLabel.SetMultiline(true)

	radioLabel = fmt.Sprintf("Destructive Installation")
	page.destructiveRadio = clui.CreateRadio(radioButtonFrame, 50, radioLabel, AutoSize)
	page.destructiveRadio.SetStyle("Media")
	page.destructiveRadio.SetPack(clui.Horizontal)
	page.group.AddItem(page.destructiveRadio)
	page.destructiveRadio.OnChange(page.destructiveRadioOnChange)
	// Description of Destructive
	clui.CreateLabel(radioButtonFrame, 1, 1, "Erase all data on selected media and install Clear Linux* OS.", Fixed)

	radioLabel = fmt.Sprintf("Advanced Installation")
	page.advancedRadio = clui.CreateRadio(radioButtonFrame, 50, radioLabel, AutoSize)
	page.advancedRadio.SetStyle("Media")
	page.advancedRadio.SetPack(clui.Horizontal)
	page.group.AddItem(page.advancedRadio)
	page.advancedRadio.OnChange(page.advancedRadioOnChange)

	// Description of Advanced
	clui.CreateLabel(radioButtonFrame, 1, 1, "Use partitioning tool to select media via partition names.", Fixed)

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
		page.runDiskPartitionTool(page.destructiveTargets[page.chooserList.SelectedItem()].Name)
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
			if _, found := page.getModel().InstallSelected[target.Name]; found {
				target.EraseDisk = false
			}
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
	page.destructiveTargets = storage.FindAllInstallTargets(storage.MinimumServerInstallSize, page.devs)

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
		page.setConfirmButton()
	}

	clui.RefreshScreen()

	return nil
}

func (page *MediaConfigPage) runDiskPartitionTool(disk string) {

	stdMsg := "Could not launch " + diskUtil + ". Check " + log.GetLogFileName()
	msg := ""

	drive := filepath.Join("/dev", disk)
	exists, err := utils.FileExists(drive)
	if err != nil {
		log.Warning("Failed to check drive %s: %v", drive, err)
		msg = stdMsg
	} else if !exists {
		log.Warning("Request drive %s does not exist", drive)
		msg = stdMsg
	}

	diskUtilCmd := fmt.Sprintf("/usr/bin/%s %s", diskUtil, drive)

	tmpYaml, err := page.getModel().WriteScrubModelTargetMedias()
	if err != nil {
		log.Warning("%v", err)
		msg = stdMsg
	}

	lockFile := page.getModel().LockFile

	// Need remove directories, files which might normally be handled by
	// defer functions since utils.RunDiskPartitionTool does an exec
	remove := []string{}
	remove = append(remove, page.tui.rootDir)
	cf := page.getModel().ClearCfFile
	if cf != "" {
		remove = append(remove, cf)
	}

	script, err := utils.RunDiskPartitionTool(tmpYaml, lockFile, diskUtilCmd, remove, false)
	if err != nil {
		log.Warning("%v", err)
		msg = stdMsg
	}

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
	err = syscall.Exec("/bin/bash", []string{"/bin/bash", "-l", "-c", script}, []string{"TERM=" + term})
	if err != nil {
		log.Warning("Could not start disk utility: %v", err)
		msg = stdMsg
		if _, err := CreateWarningDialogBox(msg); err != nil {
			log.Warning("Attempt to launch %s: warning dialog failed: %s", diskUtil, err)
		}
		return
	}
}

func fmtInstallTarget(target storage.InstallTarget) string {
	portion := storage.FormatInstallPortion(target)

	// Size string
	size, _ := storage.HumanReadableSizeWithPrecision(target.FreeEnd-target.FreeStart, 1)

	return fmt.Sprintf("%-32s  %10s  %-14s  %8s", target.Friendly, target.Name, portion, size)
}
