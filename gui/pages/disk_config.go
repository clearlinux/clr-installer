// Copyright © 2019 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package pages

import (
	"fmt"
	"time"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"

	"github.com/clearlinux/clr-installer/gui/common"
	"github.com/clearlinux/clr-installer/log"
	"github.com/clearlinux/clr-installer/model"
	"github.com/clearlinux/clr-installer/storage"
	"github.com/clearlinux/clr-installer/utils"
)

// DiskConfig is a simple page to help with DiskConfig settings
type DiskConfig struct {
	devs               []*storage.BlockDevice
	safeTargets        []storage.InstallTarget
	destructiveTargets []storage.InstallTarget
	activeDisk         *storage.BlockDevice
	activeSerial       string
	controller         Controller
	model              *model.SystemInstall
	box                *gtk.Box
	scroll             *gtk.ScrolledWindow
	scrollBox          *gtk.Box
	mediaGrid          *gtk.Grid
	safeButton         *gtk.RadioButton
	destructiveButton  *gtk.RadioButton
	chooserCombo       *gtk.ComboBox
	errorMessage       *gtk.Label
	rescanButton       *gtk.Button
	rescanDialog       *gtk.Dialog
	encryptCheck       *gtk.CheckButton
	passphraseDialog   *gtk.Dialog
	passphrase         *gtk.Entry
	passphraseConfirm  *gtk.Entry
	passphraseChanged  bool
	passphraseWarning  *gtk.Label
	passphraseOK       *gtk.Button
	passphraseCancel   *gtk.Button
}

// NewDiskConfigPage returns a new DiskConfigPage
func NewDiskConfigPage(controller Controller, model *model.SystemInstall) (Page, error) {
	disk := &DiskConfig{
		controller: controller,
		model:      model,
	}
	var err error

	// Page Box
	disk.box, err = setBox(gtk.ORIENTATION_VERTICAL, 0, "box-page-new")
	if err != nil {
		return nil, err
	}

	// Build storage for scrollBox
	disk.scroll, err = gtk.ScrolledWindowNew(nil, nil)
	if err != nil {
		return nil, err
	}
	disk.scroll.SetMarginTop(10)
	disk.scroll.SetMarginEnd(common.StartEndMargin)
	disk.box.PackStart(disk.scroll, true, true, 0)
	disk.scroll.SetPolicy(gtk.POLICY_NEVER, gtk.POLICY_AUTOMATIC)

	// Build scrollBox
	disk.scrollBox, err = gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 20)
	if err != nil {
		return nil, err
	}

	disk.scroll.Add(disk.scrollBox)

	// Media Grid
	disk.mediaGrid, err = gtk.GridNew()
	if err != nil {
		return nil, err
	}

	// Build the Safe Install Section
	safeBox, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	safeBox.SetMarginStart(common.StartEndMargin)
	safeBox.SetMarginTop(common.TopBottomMargin)
	disk.safeButton, err = gtk.RadioButtonNewWithLabelFromWidget(nil, utils.Locale.Get("Safe Installation"))
	if err != nil {
		return nil, err
	}
	sc, err := disk.safeButton.GetStyleContext()
	if err != nil {
		log.Warning("Error getting style context: ", err) // Just log trivial error
	} else {
		sc.AddClass("label-radio")
	}
	safeBox.PackStart(disk.safeButton, false, false, 0)
	if _, err := disk.safeButton.Connect("toggled", func() {
		// Enable/Disable the Combo Choose Box based on the radio button
		if err := disk.populateComboBoxes(); err != nil {
			log.Warning("Problem populating possible disk selections")
		}
	}); err != nil {
		return nil, err
	}

	safeDescription := utils.Locale.Get("Install on an unallocated disk or alongside existing partitions.")
	safeLabel, err := gtk.LabelNew(safeDescription)
	if err != nil {
		return nil, err
	}
	safeLabel.SetLineWrap(true)
	safeLabel.SetXAlign(0.0)
	safeLabel.SetMarginStart(30)
	safeLabel.SetUseMarkup(true)
	safeBox.PackStart(safeLabel, false, false, 0)

	safeBox.ShowAll()
	disk.mediaGrid.Attach(safeBox, 0, 0, 1, 1)

	// Build Destructive Install Section
	destructiveBox, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	destructiveBox.SetMarginStart(common.StartEndMargin)
	disk.destructiveButton, err = gtk.RadioButtonNewWithLabelFromWidget(disk.safeButton, utils.Locale.Get("Destructive Installation"))
	if err != nil {
		return nil, err
	}
	sc, err = disk.destructiveButton.GetStyleContext()
	if err != nil {
		log.Warning("Error getting style context: ", err) // Just log trivial error
	} else {
		sc.AddClass("label-radio-warning")
	}
	destructiveBox.PackStart(disk.destructiveButton, false, false, 0)
	if _, err := disk.destructiveButton.Connect("toggled", func() {
		// Enable/Disable the Combo Choose Box based on the radio button
		if err := disk.populateComboBoxes(); err != nil {
			log.Warning("Problem populating possible disk selections")
		}
	}); err != nil {
		return nil, err
	}

	destructiveDescription := utils.Locale.Get("Erase all data on selected media and install Clear Linux* OS.")
	destructiveLabel, err := gtk.LabelNew(destructiveDescription)
	if err != nil {
		return nil, err
	}
	destructiveLabel.SetLineWrap(true)
	destructiveLabel.SetXAlign(0.0)
	destructiveLabel.SetMarginStart(30)
	destructiveLabel.SetUseMarkup(true)
	destructiveBox.PackStart(destructiveLabel, false, false, 0)

	destructiveBox.ShowAll()
	disk.mediaGrid.Attach(destructiveBox, 0, 1, 1, 1)

	log.Debug("Before making ComboBox")
	disk.chooserCombo, err = gtk.ComboBoxNew()
	if err != nil {
		log.Warning("Failed to make disk.chooserCombo")
		return nil, err
	}

	// Add the renderers to the ComboBox
	mediaRenderer, _ := gtk.CellRendererPixbufNew()
	disk.chooserCombo.PackStart(mediaRenderer, true)
	disk.chooserCombo.AddAttribute(mediaRenderer, "pixbuf", 0)

	friendlyRenderer, _ := gtk.CellRendererTextNew()
	disk.chooserCombo.PackStart(friendlyRenderer, true)
	disk.chooserCombo.AddAttribute(friendlyRenderer, "text", 1)

	nameRenderer, _ := gtk.CellRendererTextNew()
	disk.chooserCombo.PackStart(nameRenderer, true)
	disk.chooserCombo.AddAttribute(nameRenderer, "text", 2)

	portionRenderer, _ := gtk.CellRendererTextNew()
	disk.chooserCombo.PackStart(portionRenderer, true)
	disk.chooserCombo.AddAttribute(portionRenderer, "text", 3)

	sizeRenderer, _ := gtk.CellRendererTextNew()
	disk.chooserCombo.PackStart(sizeRenderer, true)
	disk.chooserCombo.AddAttribute(sizeRenderer, "text", 4)

	disk.mediaGrid.Attach(disk.chooserCombo, 1, 0, 1, 2)

	disk.mediaGrid.SetRowSpacing(10)
	disk.mediaGrid.SetColumnSpacing(10)
	disk.mediaGrid.SetColumnHomogeneous(true)
	disk.scrollBox.Add(disk.mediaGrid)

	separator, err := gtk.SeparatorNew(gtk.ORIENTATION_HORIZONTAL)
	if err != nil {
		return nil, err
	}
	separator.ShowAll()
	disk.scrollBox.Add(separator)

	// Error Message Label
	disk.errorMessage, err = gtk.LabelNew("")
	if err != nil {
		return nil, err
	}
	disk.errorMessage.SetUseMarkup(true)
	disk.errorMessage.SetMarginStart(common.StartEndMargin)
	disk.scrollBox.Add(disk.errorMessage)

	// Encryption button
	disk.encryptCheck, err = gtk.CheckButtonNew()
	if err != nil {
		return nil, err
	}

	disk.encryptCheck.SetLabel("  " + utils.Locale.Get("Enable Encryption"))
	disk.encryptCheck.SetMarginStart(common.StartEndMargin)
	disk.encryptCheck.SetHAlign(gtk.ALIGN_START) // Ensures that clickable area is only within the label
	disk.scrollBox.PackStart(disk.encryptCheck, false, false, 0)

	// Generate signal on encryptCheck button click
	if _, err := disk.encryptCheck.Connect("clicked", disk.onEncryptClick); err != nil {
		return nil, err
	}

	// Buttons
	disk.rescanButton, err = setButton(utils.Locale.Get("RESCAN MEDIA"), "button-page")
	if err != nil {
		return nil, err
	}
	disk.rescanButton.SetTooltipText(utils.Locale.Get("Rescan for changes to hot swappable media."))

	if _, err = disk.rescanButton.Connect("clicked", disk.onRescanClick); err != nil {
		return nil, err
	}

	rescanBox, err := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 0)
	if err != nil {
		return nil, err
	}
	rescanBox.SetMarginStart(common.StartEndMargin)
	rescanBox.PackStart(disk.rescanButton, false, false, 10)

	rescanBox.ShowAll()
	disk.scrollBox.Add(rescanBox)

	disk.box.ShowAll()

	return disk, nil
}

func newListStoreMedia() (*gtk.ListStore, error) {
	store, err := gtk.ListStoreNew(glib.TYPE_OBJECT, glib.TYPE_STRING, glib.TYPE_STRING, glib.TYPE_STRING, glib.TYPE_STRING)
	return store, err
}

func (disk *DiskConfig) runScanLoop() {
	var duration time.Duration
	for {
		select {
		case <-disk.controller.GetScanChannel():
			return
		default:
			time.Sleep(common.LoopWaitDuration)
			duration += common.LoopWaitDuration
			// Safety check. In case reading the channel gets delayed for some reason,
			// do not hold up loading the page.
			if duration > common.LoopTimeOutDuration {
				return
			}
		}
	}
}

// addListStoreMediaRow adds new row to the ListStore widget for the given media
func addListStoreMediaRow(store *gtk.ListStore, installMedia storage.InstallTarget) error {

	// Create icon image
	mediaType := "drive-harddisk-system"
	if installMedia.Removable {
		mediaType = "media-removable"
	}
	mediaType = mediaType + "-symbolic"
	image, err := gtk.ImageNewFromIconName(mediaType, gtk.ICON_SIZE_DIALOG)
	if err != nil {
		log.Warning("gtk.ImageNewFromIconName failed for icon %q", mediaType)
		return err
	}

	iter := store.Append()

	err = store.SetValue(iter, 0, image.GetPixbuf())
	if err != nil {
		log.Warning("SetValue store failed for icon %q", mediaType)
		return err
	}

	// Friendly string
	friendlyString := installMedia.Friendly

	err = store.SetValue(iter, 1, friendlyString)
	if err != nil {
		log.Warning("SetValue store failed for friendly string: %q", friendlyString)
		return err
	}

	// Name string
	nameString := installMedia.Name

	err = store.SetValue(iter, 2, nameString)
	if err != nil {
		log.Warning("SetValue store failed for name string: %q", nameString)
		return err
	}

	// Portion string
	portionString := storage.FormatInstallPortion(installMedia)
	err = store.SetValue(iter, 3, portionString)
	if err != nil {
		log.Warning("SetValue store failed for portion string: %q", portionString)
		return err
	}

	// Size string
	sizeString, _ := storage.HumanReadableSizeWithPrecision(installMedia.FreeEnd-installMedia.FreeStart, 1)

	err = store.SetValue(iter, 4, sizeString)
	if err != nil {
		log.Warning("SetValue store failed for size string: %q", sizeString)
		return err
	}

	return nil
}

func (disk *DiskConfig) onRescanClick() {
	log.Debug("Rescanning media...")
	disk.createRescanDialog()
	disk.rescanDialog.ShowAll()
	go func() {
		scannedMedia, err := storage.RescanBlockDevices(disk.model.TargetMedias)
		if err != nil {
			log.Warning("Error scanning media %s", err.Error())
		}
		disk.controller.SetScanMedia(scannedMedia)
		disk.devs = disk.controller.GetScanMedia()

		// Check if the active device is still present
		var found bool
		for _, bd := range disk.devs {
			if bd.Serial == disk.activeSerial {
				found = true
				disk.activeDisk = bd
			}
		}
		if !found {
			disk.activeSerial = ""
			disk.activeDisk = nil
			disk.model.TargetMedias = nil
		}
		disk.refreshPage()
		disk.rescanDialog.Close() // Unlike Destroy(), Close() closes the dialog window and seems to not crash
	}()
}

func (disk *DiskConfig) createRescanDialog() {
	title := utils.Locale.Get("Rescanning media")
	text := utils.Locale.Get("Searching the system for available media.") + " " + utils.Locale.Get("Please wait.")

	contentBox, err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	contentBox.SetHAlign(gtk.ALIGN_FILL)
	contentBox.SetMarginBottom(common.TopBottomMargin)
	if err != nil {
		log.Warning("Error creating box")
		return
	}

	label, err := gtk.LabelNew(text)
	if err != nil {
		log.Warning("Error creating label")
		return
	}

	label.SetHAlign(gtk.ALIGN_START)
	label.SetMarginBottom(common.TopBottomMargin)
	contentBox.PackStart(label, true, true, 0)

	disk.rescanDialog, err = common.CreateDialog(contentBox, title)
	if err != nil {
		log.Warning("Error creating dialog")
		return
	}
}

func (disk *DiskConfig) createPassphraseDialog() {
	title := utils.Locale.Get(storage.EncryptionPassphrase)
	text := utils.Locale.Get(storage.PassphraseMessage)

	contentBox, err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	contentBox.SetHAlign(gtk.ALIGN_FILL)
	contentBox.SetMarginBottom(common.TopBottomMargin)
	if err != nil {
		log.Warning("Error creating box")
		return
	}

	label, err := gtk.LabelNew(text)
	if err != nil {
		log.Warning("Error creating label")
		return
	}

	label.SetHAlign(gtk.ALIGN_START)
	label.SetMarginBottom(common.TopBottomMargin)
	contentBox.PackStart(label, true, true, 0)

	disk.passphrase, err = setEntry("")
	if err != nil {
		log.Warning("Error creating entry")
		return
	}
	disk.passphrase.SetMarginBottom(common.TopBottomMargin)
	contentBox.PackStart(disk.passphrase, true, true, 0)

	disk.passphraseConfirm, err = setEntry("")
	if err != nil {
		log.Warning("Error creating entry")
		return
	}
	disk.passphraseConfirm.SetMarginBottom(common.TopBottomMargin)
	contentBox.PackStart(disk.passphraseConfirm, true, true, 0)

	disk.passphraseWarning, err = setLabel("", "label-warning", 0.0)
	if err != nil {
		log.Warning("Error creating label")
		return
	}
	contentBox.PackStart(disk.passphraseWarning, true, true, 0)

	disk.passphraseCancel, err = common.SetButton(utils.Locale.Get("CANCEL"), "button-cancel")
	disk.passphraseCancel.SetMarginEnd(common.ButtonSpacing)
	if err != nil {
		return
	}

	disk.passphraseOK, err = common.SetButton(utils.Locale.Get("CONFIRM"), "button-confirm")
	if err != nil {
		return
	}
	disk.passphraseOK.SetMarginEnd(common.StartEndMargin)
	disk.passphraseOK.SetSensitive(false)

	disk.passphrase.SetVisibility(false)
	disk.passphraseConfirm.SetVisibility(false)
	/*
		if disk.model.CryptPass != "" {
			disk.passphraseChanged = false
			setTextInEntry(disk.passphrase, "********")
			setTextInEntry(disk.passphraseConfirm, "********")
		}
	*/
	if _, err := disk.passphrase.Connect("changed", disk.onPassphraseChange); err != nil {
		log.Warning("Error connecting to entry")
		return
	}

	if _, err := disk.passphrase.Connect("activate", disk.onPassphraseActive); err != nil {
		log.Warning("Error connecting to entry")
		return
	}

	if _, err := disk.passphrase.Connect("key-press-event", disk.onPassphraseKeyPress); err != nil {
		log.Warning("Error connecting to entry")
		return
	}

	// Generate signal on PassphraseConfirm change
	if _, err := disk.passphraseConfirm.Connect("changed", disk.onPassphraseChange); err != nil {
		log.Warning("Error connecting to entry")
		return
	}

	if _, err := disk.passphraseConfirm.Connect("activate", disk.onPassphraseActive); err != nil {
		log.Warning("Error connecting to entry")
		return
	}

	if _, err := disk.passphraseConfirm.Connect("key-press-event", disk.onPassphraseKeyPress); err != nil {
		log.Warning("Error connecting to entry")
		return
	}

	disk.passphraseDialog, err = common.CreateDialog(contentBox, title)
	if err != nil {
		log.Warning("Error creating dialog")
		return
	}

	disk.passphraseDialog.AddActionWidget(disk.passphraseCancel, gtk.RESPONSE_CANCEL)
	disk.passphraseDialog.AddActionWidget(disk.passphraseOK, gtk.RESPONSE_OK)

	_, err = disk.passphraseDialog.Connect("response", disk.dialogResponse)
	if err != nil {
		log.Warning("Error connecting to dialog")
	}
}

func (disk *DiskConfig) onPassphraseChange(entry *gtk.Entry) {
	disk.validatePassphrase()
}

func (disk *DiskConfig) onPassphraseActive(entry *gtk.Entry) {
	if disk.passphrase.IsFocus() {
		disk.validatePassphrase()
	}
}

func (disk *DiskConfig) onPassphraseKeyPress(entry *gtk.Entry, event *gdk.Event) {
	// TODO: Implement specific key presses

	if !disk.passphraseChanged {
		disk.passphraseChanged = true
		setTextInEntry(disk.passphrase, "")
		setTextInEntry(disk.passphraseConfirm, "")
	}
}

func (disk *DiskConfig) validatePassphrase() {
	if !disk.passphraseChanged {
		return
	}

	if ok, msg := storage.IsValidPassphrase(getTextFromEntry(disk.passphrase)); !ok {
		disk.passphraseWarning.SetText(msg)
		disk.passphraseOK.SetSensitive(false)
	} else if getTextFromEntry(disk.passphrase) != getTextFromEntry(disk.passphraseConfirm) {
		disk.passphraseWarning.SetText(utils.Locale.Get("Passphrases do not match"))
		disk.passphraseOK.SetSensitive(false)
	} else {
		disk.passphraseWarning.SetText("")
		disk.passphraseOK.SetSensitive(true)
	}
}

// dialogResponse handles the response from the dialog message
func (disk *DiskConfig) dialogResponse(msgDialog *gtk.Dialog, responseType gtk.ResponseType) {
	if responseType == gtk.RESPONSE_OK {
		disk.model.CryptPass = getTextFromEntry(disk.passphrase)
	} else {
		disk.encryptCheck.SetActive(false)
	}
	msgDialog.Destroy()
}

func (disk *DiskConfig) onEncryptClick(button *gtk.CheckButton) {
	if disk.encryptCheck.GetActive() {
		disk.createPassphraseDialog()
		disk.passphraseDialog.ShowAll()
	}
}

// populateComboBoxes populates the scrollBox with usable widget things
func (disk *DiskConfig) populateComboBoxes() error {
	// Clear any previous warning
	disk.errorMessage.SetMarkup("")
	disk.controller.SetButtonState(ButtonConfirm, true)

	safeStore, err := newListStoreMedia()
	if err != nil {
		log.Warning("ListStoreNew safeStore failed")
		return err
	}

	destructiveStore, err := newListStoreMedia()
	if err != nil {
		log.Warning("ListStoreNew destructiveStore failed")
		return err
	}

	if len(disk.devs) < 1 {
		warning := utils.Locale.Get("No media found for installation")
		log.Warning(warning)
		warning = fmt.Sprintf("<big><b><span foreground=\"#FDB814\">" + utils.Locale.Get("Warning: %s", warning) + "</span></b></big>")
		disk.errorMessage.SetMarkup(warning)
		emptyStore, err := newListStoreMedia()
		if err != nil {
			log.Warning("ListStoreNew emptyStore failed")
			return err
		}
		disk.chooserCombo.SetModel(emptyStore)
		disk.controller.SetButtonState(ButtonConfirm, false)

		return nil
	}

	disk.safeTargets = storage.FindSafeInstallTargets(storage.MinimumDesktopInstallSize, disk.devs)
	disk.destructiveTargets = storage.FindAllInstallTargets(disk.devs)

	// Hook for searching CLR_*?
	disk.model.TargetMedias = nil
	for _, curr := range storage.FindAdvancedInstallTargets(disk.devs) {
		disk.model.AddTargetMedia(curr)
		log.Debug("AddTargetMedia %+v", curr)
	}

	if disk.safeButton.GetActive() {
		if len(disk.safeTargets) > 0 {
			for _, target := range disk.safeTargets {
				log.Debug("Adding safe install target %s", target.Name)
				err := addListStoreMediaRow(safeStore, target)
				if err != nil {
					log.Warning("SetValue safeStore")
					return err
				}
			}
			disk.chooserCombo.SetModel(safeStore)
			disk.chooserCombo.SetActive(0)
		} else {
			disk.chooserCombo.SetModel(safeStore)
			warning := utils.Locale.Get("No safe media found for installation")
			log.Warning(warning)
			warning = fmt.Sprintf("<big><b><span foreground=\"#FDB814\">" + utils.Locale.Get("Warning: %s", warning) + "</span></b></big>")
			disk.errorMessage.SetMarkup(warning)
			disk.controller.SetButtonState(ButtonConfirm, false)
		}
	} else if disk.destructiveButton.GetActive() {
		for _, target := range disk.destructiveTargets {
			log.Debug("Adding destructive install target %s", target.Name)
			err := addListStoreMediaRow(destructiveStore, target)
			if err != nil {
				log.Warning("SetValue destructiveStore")
				return err
			}
		}
		disk.chooserCombo.SetModel(destructiveStore)
		disk.chooserCombo.SetActive(0)
	}

	return nil
}

// IsRequired will return true as we always need a DiskConfig
func (disk *DiskConfig) IsRequired() bool {
	return true
}

// IsDone checks if all the steps are completed
func (disk *DiskConfig) IsDone() bool {
	return disk.model.TargetMedias != nil
}

// GetID returns the ID for this page
func (disk *DiskConfig) GetID() int {
	return PageIDDiskConfig
}

// GetIcon returns the icon for this page
func (disk *DiskConfig) GetIcon() string {
	return "drive-harddisk-system"
}

// GetRootWidget returns the root embeddable widget for this page
func (disk *DiskConfig) GetRootWidget() gtk.IWidget {
	return disk.box
}

// GetSummary will return the summary for this page
func (disk *DiskConfig) GetSummary() string {
	return utils.Locale.Get("Select Installation Media")
}

// GetTitle will return the title for this page
func (disk *DiskConfig) GetTitle() string {
	return disk.GetSummary()
}

// StoreChanges will store this pages changes into the model
func (disk *DiskConfig) StoreChanges() {
	var installBlockDevice *storage.BlockDevice

	if disk.safeButton.GetActive() {
		log.Debug("Safe Install chooserCombo selected %v", disk.chooserCombo.GetActive())
		selected := disk.safeTargets[disk.chooserCombo.GetActive()]
		disk.model.InstallSelected[selected.Name] = selected
		log.Debug("Safe Install Target %v", selected)
		disk.model.TargetMedias = nil
	} else if disk.destructiveButton.GetActive() {
		log.Debug("Destructive Install chooserCombo selected %v", disk.chooserCombo.GetActive())
		selected := disk.destructiveTargets[disk.chooserCombo.GetActive()]
		disk.model.InstallSelected[selected.Name] = selected
		log.Debug("Destructive Install Target %v", selected)
		disk.model.TargetMedias = nil
	} else {
		log.Warning("Failed to find and save the selected installation media")
	}

	bds, err := storage.ListAvailableBlockDevices(disk.model.TargetMedias)
	if err != nil {
		log.Error("Failed to find storage media for install during save: %s", err)
	}

	for _, selected := range disk.model.InstallSelected {
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
				// Give the active disk to the model
				disk.model.AddTargetMedia(installBlockDevice)
				break
			}
		}
	}

	if disk.encryptCheck.GetActive() {
		for _, child := range installBlockDevice.Children {
			if child.MountPoint == "/" {
				child.Type = storage.BlockDeviceTypeCrypt
			}
		}
	}
}

// ResetChanges will reset this page to match the model
func (disk *DiskConfig) ResetChanges() {
	if !disk.controller.GetScanDone() { // If media has not been scanned even once, wait till scanning completes
		disk.runScanLoop()
		disk.controller.SetScanDone(true)
	}
	disk.devs = disk.controller.GetScanMedia()
	disk.refreshPage()
}

// refreshPage will refresh the UI
func (disk *DiskConfig) refreshPage() {
	log.Debug("Refreshing page...")
	disk.activeDisk = nil
	disk.chooserCombo.SetSensitive(false)

	if err := disk.populateComboBoxes(); err != nil {
		log.Warning("Problem populating possible disk selections")
	}

	// Choose the most appropriate button
	if len(disk.safeTargets) > 0 {
		disk.safeButton.SetActive(true)
		disk.chooserCombo.SetSensitive(true)
	} else if len(disk.destructiveTargets) > 0 {
		disk.destructiveButton.SetActive(true)
		disk.chooserCombo.SetSensitive(true)
	} else {
		//disk.rescanButton.SetActive(true)
		//TODO: Make this button have focus/default
		log.Debug("Need to make the rescan button default")
	}

	// TODO: Match list to target medias. But we have an ugly
	// list of root target medias and you can only select one
	// right now as our manual partitioning is missing.
	if disk.model.TargetMedias == nil {
		return
	}
}

// GetConfiguredValue returns our current config
func (disk *DiskConfig) GetConfiguredValue() string {
	tm := disk.model.TargetMedias
	if len(tm) == 0 {
		return utils.Locale.Get("No Media Selected")
	} else if len(tm) > 1 {
		log.Warning("Too many media found, one 1 supported: %+v", tm)
		return utils.Locale.Get("Too many media found")
	}

	bd := tm[0]
	target := disk.model.InstallSelected[bd.Name]
	portion := storage.FormatInstallPortion(target)

	// Size string
	size, _ := storage.HumanReadableSizeWithPrecision(target.FreeEnd-target.FreeStart, 1)

	encrypted := ""
	for _, ch := range bd.Children {
		if ch.Type == storage.BlockDeviceTypeCrypt {
			encrypted = " " + utils.Locale.Get("Encryption")
		}
	}

	return fmt.Sprintf("%s (%s) %s%s %s", target.Friendly, target.Name, portion, encrypted, size)
}
