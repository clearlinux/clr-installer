// Copyright © 2019 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"fmt"
	"strings"

	"github.com/VladimirMarkelov/clui"
	term "github.com/nsf/termbox-go"

	"github.com/clearlinux/clr-installer/log"
	"github.com/clearlinux/clr-installer/storage"
)

// DiskPartitionPage is the Page implementation for partition configuration page
type DiskPartitionPage struct {
	BasePage
	fsList        *clui.ListBox
	fsOriginal    string
	encryptCheck  *clui.CheckBox
	formatCheck   *clui.CheckBox
	labelEdit     *clui.EditField
	labelWarning  *clui.Label
	mPointEdit    *clui.EditField
	mPointWarning *clui.Label
	sizeEdit      *clui.EditField
	confirmBtn    *SimpleButton
	deleteBtn     *SimpleButton
	cancelBtn     *SimpleButton
	sizeWarning   *clui.Label
	sizeInfo      *clui.Label
	sizeOriginal  string
	sizeTrue      uint64 // size of the device
}

const (
	nPartitionHelp = "Set the new partition's file system, mount point and size."

	// partConfirmBtn mask defines a partition configuration page will have a confirm button
	partConfirmBtn = 1 << 1

	// partDeleteBtn mask defines a partition configuration page will have a delete button
	partDeleteBtn = 1 << 2

	// partCancelBtn mask defines a partition configuration page will have a cancel button
	partCancelBtn = 1 << 3

	// partAllBtns mask defines a partition configuration page will have show both:
	// delete, add and confirm buttons
	partAllBtns = partConfirmBtn | partDeleteBtn | partCancelBtn
)

func (page *DiskPartitionPage) setPartitionButtonsVisible(visible bool, mask int) {
	if mask&partConfirmBtn == partConfirmBtn {
		page.confirmBtn.SetVisible(visible)
		page.setConfirmButton()
	}

	if mask&partDeleteBtn == partDeleteBtn {
		page.deleteBtn.SetVisible(visible)
	}

	if mask&partCancelBtn == partCancelBtn {
		page.cancelBtn.SetVisible(visible)
	}
}

func (page *DiskPartitionPage) setPartitionForm(part *storage.BlockDevice) {
	idx := page.fsList.FindItem(part.FsType, true)
	page.fsList.SelectItem(idx)

	if part.Type == storage.BlockDeviceTypeCrypt {
		page.encryptCheck.SetState(1)
	} else {
		page.encryptCheck.SetState(0)
	}

	page.setFormatCheckbox()

	page.labelEdit.SetTitle(part.Label)
	page.validateLabel(part.FsType)

	page.mPointEdit.SetEnabled(true)
	if part.FsType == "swap" {
		page.mPointEdit.SetEnabled(false)
	} else {
		page.mPointEdit.SetTitle(part.MountPoint)
		page.validateMountPoint()
	}

	size, err := part.HumanReadableSize()
	if err != nil {
		page.Panic(err)
	}

	page.fsOriginal = part.FsType

	page.sizeOriginal = size
	page.sizeTrue = part.Size // The actual size, not the human readable
	page.sizeEdit.SetTitle(size)

	page.setPartitionButtonsVisible(true, partAllBtns)
}

func (page *DiskPartitionPage) setFormatCheckbox() {
	sel := page.getSelectedBlockDevice()

	// Default to enabled
	page.formatCheck.SetEnabled(true)

	// If this was a user defined (added) partition, it has to be formatted
	if sel.part.UserDefined {
		page.formatCheck.SetState(1)
		page.formatCheck.SetEnabled(false)
	} else {
		// We always need to format root (/) for an install
		if page.mPointEdit.Title() == "/" {
			page.formatCheck.SetState(1)
			page.formatCheck.SetEnabled(false)
		}

		// If the user changed the file system type
		if page.fsList.SelectedItemText() != page.fsOriginal {
			page.formatCheck.SetState(1)
			page.formatCheck.SetEnabled(false)
		}

		// If the user changed the file system size
		if page.sizeEdit.Title() != page.sizeOriginal {
			page.formatCheck.SetState(1)
			page.formatCheck.SetEnabled(false)
		}
	}
}

func (page *DiskPartitionPage) getSelectedBlockDevice() *SelectedBlockDevice {
	var sel *SelectedBlockDevice
	var ok bool

	prevPage := page.tui.getPage(TuiPageDiskConfig)
	if sel, ok = prevPage.GetData().(*SelectedBlockDevice); !ok {
		return nil
	}

	return sel
}

// Activate is called when the window is "shown", this implementation adjusts
// the currently displayed data
func (page *DiskPartitionPage) Activate() {
	sel := page.getSelectedBlockDevice()

	if sel == nil {
		return
	}

	page.labelEdit.SetTitle("")
	page.labelWarning.SetTitle("")
	page.mPointEdit.SetTitle("")
	page.mPointWarning.SetTitle("")
	page.sizeEdit.SetTitle("")
	page.sizeInfo.SetTitle("'+/=' to force Maximum size")
	page.sizeWarning.SetTitle("")

	page.setPartitionForm(sel.part)

	if sel.addMode {
		page.setPartitionButtonsVisible(false, partCancelBtn)
		// In Add partition mode, the Delete button is really
		// our "Cancel" as the new partition was already added.
		page.deleteBtn.SetTitle("Cancel")
		// and the Confirm button is really our "Add" button
		page.confirmBtn.SetTitle("Add")
	} else {
		page.setPartitionButtonsVisible(true, partCancelBtn)
		page.deleteBtn.SetTitle("Delete")
		page.confirmBtn.SetTitle("Confirm")
	}
}

func (page *DiskPartitionPage) setConfirmButton() {
	if page.labelWarning.Title() == "" &&
		page.mPointWarning.Title() == "" &&
		page.sizeWarning.Title() == "" {
		page.confirmBtn.SetEnabled(true)
	} else {
		page.confirmBtn.SetEnabled(false)
	}
}

func (page *DiskPartitionPage) validateLabel(fstype string) {
	page.labelWarning.SetTitle(
		storage.IsValidLabel(page.labelEdit.Title(), fstype))

	page.setConfirmButton()
}

func (page *DiskPartitionPage) validateMountPoint() {
	page.mPointWarning.SetTitle(storage.IsValidMount(page.mPointEdit.Title()))

	page.setConfirmButton()
}

func newDiskPartitionPage(tui *Tui) (Page, error) {
	page := &DiskPartitionPage{}

	page.setup(tui, TuiPageDiskPart, NoButtons, TuiPageDiskConfig)

	lbl := clui.CreateLabel(page.content, 2, 2, "Partition Setup", Fixed)
	lbl.SetPaddings(0, 2)

	clui.CreateLabel(page.content, 2, 2, nPartitionHelp, Fixed)

	frm := clui.CreateFrame(page.content, AutoSize, AutoSize, BorderNone, Fixed)
	frm.SetPack(clui.Horizontal)

	lblFrm := clui.CreateFrame(frm, 20, AutoSize, BorderNone, Fixed)
	lblFrm.SetPack(clui.Vertical)
	lblFrm.SetPaddings(1, 0)

	lbl = clui.CreateLabel(lblFrm, AutoSize, 4, "File System:", Fixed)
	lbl.SetAlign(AlignRight)

	lbl = clui.CreateLabel(lblFrm, AutoSize, 2, "[Optional] Label:", Fixed)
	lbl.SetAlign(AlignRight)

	lbl = clui.CreateLabel(lblFrm, AutoSize, 2, "Mount Point:", Fixed)
	lbl.SetAlign(AlignRight)

	lbl = clui.CreateLabel(lblFrm, AutoSize, 2, "Size:", Fixed)
	lbl.SetAlign(AlignRight)

	fldFrm := clui.CreateFrame(frm, 30, AutoSize, BorderNone, Fixed)
	fldFrm.SetPack(clui.Vertical)

	partWrapperFrm := clui.CreateFrame(fldFrm, 30, 4, BorderNone, Fixed)
	partWrapperFrm.SetPack(clui.Vertical)
	partWrapperFrm.SetPaddings(0, 0)
	partWrapperFrm.SetGaps(0, 0)

	partFrm := clui.CreateFrame(partWrapperFrm, 30, 3, BorderNone, Fixed)
	partFrm.SetPack(clui.Vertical)
	partFrm.SetPaddings(0, 0)
	partFrm.SetGaps(2, 1)

	page.fsList = clui.CreateListBox(partFrm, 20, 3, Fixed)
	page.fsList.SetAlign(AlignLeft)
	page.fsList.SetStyle("List")

	page.fsList.OnActive(func(active bool) {
		if active {
			page.fsList.SetStyle("ListActive")
		} else {
			page.fsList.SetStyle("List")
		}
	})

	for _, fs := range storage.SupportedFileSystems() {
		page.fsList.AddItem(fs)
	}
	page.fsList.SelectItem(0)

	partFrm.SetPack(clui.Horizontal)
	page.encryptCheck = clui.CreateCheckBox(partFrm, AutoSize, "Encrypt", AutoSize)

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

	page.formatCheck = clui.CreateCheckBox(partFrm, AutoSize, "Format", AutoSize)

	labelFrm := clui.CreateFrame(fldFrm, 4, 2, BorderNone, Fixed)
	labelFrm.SetPack(clui.Vertical)
	labelFrm.SetPaddings(0, 0)

	page.labelEdit = clui.CreateEditField(labelFrm, 3, "", Fixed)
	page.labelEdit.OnChange(func(ev clui.Event) {
		page.validateLabel(page.fsList.SelectedItemText())
	})

	page.labelWarning = clui.CreateLabel(labelFrm, 1, 1, "", Fixed)
	page.labelWarning.SetMultiline(true)
	page.labelWarning.SetBackColor(errorLabelBg)
	page.labelWarning.SetTextColor(errorLabelFg)

	mPointFrm := clui.CreateFrame(fldFrm, 4, 2, BorderNone, Fixed)
	mPointFrm.SetPack(clui.Vertical)
	mPointFrm.SetPaddings(0, 0)

	page.mPointEdit = clui.CreateEditField(mPointFrm, 3, "", Fixed)
	page.mPointEdit.OnChange(func(ev clui.Event) {
		page.validateMountPoint()

		if page.mPointEdit.Title() == "/boot" {
			page.encryptCheck.SetState(0)
			page.encryptCheck.SetEnabled(false)
		} else {
			page.encryptCheck.SetEnabled(true)
		}

		page.setFormatCheckbox()
	})

	page.mPointWarning = clui.CreateLabel(mPointFrm, 1, 1, "", Fixed)
	page.mPointWarning.SetMultiline(true)
	page.mPointWarning.SetBackColor(errorLabelBg)
	page.mPointWarning.SetTextColor(errorLabelFg)

	page.fsList.OnSelectItem(func(evt clui.Event) {
		page.mPointEdit.SetEnabled(true)

		if page.fsList.SelectedItemText() == "swap" {
			page.mPointEdit.SetEnabled(false)
			page.mPointEdit.SetTitle("")
			page.mPointWarning.SetTitle("")
		}

		page.setFormatCheckbox()

		page.validateLabel(page.fsList.SelectedItemText())
	})

	sizeFrm := clui.CreateFrame(fldFrm, 5, 3, BorderNone, Fixed)
	sizeFrm.SetPack(clui.Vertical)

	page.sizeEdit = clui.CreateEditField(sizeFrm, 3, "", Fixed)
	page.sizeEdit.OnChange(func(ev clui.Event) {
		if page.sizeEdit.Title() != page.sizeOriginal {
			sel := page.getSelectedBlockDevice()
			page.sizeWarning.SetTitle(sel.part.IsValidSize(page.sizeEdit.Title(), page.sizeTrue))
		} else {
			page.sizeWarning.SetTitle("")
		}
		page.setFormatCheckbox()
		page.setConfirmButton()
	})
	page.sizeEdit.OnKeyPress(func(k term.Key, ch rune) bool {
		maxSizeKeys := []rune{'=', '+'}
		for _, curr := range maxSizeKeys {
			if curr == ch {
				page.sizeEdit.SetTitle(fmt.Sprintf("%v", page.sizeTrue))
				return true
			}
		}

		return false
	})

	page.sizeInfo = clui.CreateLabel(sizeFrm, 1, 1, "", Fixed)
	page.sizeInfo.SetMultiline(false)
	page.sizeWarning = clui.CreateLabel(sizeFrm, 1, 1, "", Fixed)
	page.sizeWarning.SetMultiline(true)
	page.sizeWarning.SetBackColor(errorLabelBg)
	page.sizeWarning.SetTextColor(errorLabelFg)

	btnFrm := clui.CreateFrame(fldFrm, 30, 1, BorderNone, Fixed)
	btnFrm.SetPack(clui.Horizontal)
	btnFrm.SetGaps(1, 1)
	btnFrm.SetPaddings(2, 0)

	page.confirmBtn = CreateSimpleButton(btnFrm, AutoSize, AutoSize, "Confirm", Fixed)
	page.confirmBtn.OnClick(func(ev clui.Event) {
		var warnings []string

		sel := page.getSelectedBlockDevice()

		if sel.part != nil {
			sel.part.FsType = page.fsList.SelectedItemText()
			if page.encryptCheck.State() != 0 {
				sel.part.Type = storage.BlockDeviceTypeCrypt
				if !sel.addMode {
					warnings = append(warnings, "Enabling Encryption")
				}
			} else {
				sel.part.Type = storage.BlockDeviceTypePart
			}
			if page.formatCheck.State() != 0 {
				if !sel.addMode {
					warnings = append(warnings, "Enabling Formatting")
				}
				sel.part.FormatPartition = true
			} else {
				sel.part.FormatPartition = false
			}
			sel.part.Label = page.labelEdit.Title()
			sel.part.MountPoint = page.mPointEdit.Title()
			sizeChanged := false
			if page.sizeEdit.Title() == page.sizeOriginal {
				// Use the actual size, not the human readable
				sel.part.Size = page.sizeTrue
			} else {
				sizeChanged = true
				size, err := storage.ParseVolumeHumanSize(page.sizeEdit.Title())
				if err == nil {
					sel.part.Size = size
				}
			}

			if sel.addMode {
				log.Debug("Updating free %v using size %d", sel.freePartition, sel.part.Size)
				sel.bd.AddFromFreePartition(sel.freePartition, sel.part)
			} else {
				if sel.part.FsType != page.fsOriginal {
					warnings = append(warnings, "Changing File System Type")
				}
				if sizeChanged {
					warnings = append(warnings, "Changing Partition Size")
					freePart := sel.bd.RemovePartition(sel.part)
					sel.part.MakePartition = true
					sel.part.FormatPartition = true
					sel.bd.AddFromFreePartition(freePart, sel.part)
				}
			}

			// Check for data loss warnings
			if len(warnings) > 0 {
				sel.dataLoss = true
				message := "Data Loss due to: " + strings.Join(warnings, ", ")
				if dialog, err := CreateWarningDialogBox(message); err != nil {
					log.Warning("%s: %s", message, err)
					page.GotoPage(TuiPageDiskConfig)
				} else {
					dialog.OnClose(func() {
						page.GotoPage(TuiPageDiskConfig)
					})
				}
			} else {
				page.GotoPage(TuiPageDiskConfig)
			}
		} else {
			page.GotoPage(TuiPageDiskConfig)
		}
	})

	page.deleteBtn = CreateSimpleButton(btnFrm, AutoSize, AutoSize, "Delete", Fixed)
	page.deleteBtn.OnClick(func(ev clui.Event) {
		sel := page.getSelectedBlockDevice()
		log.Debug("Removing partition %v", sel.part)
		_ = sel.bd.RemovePartition(sel.part)
		sel.dataLoss = true
		message := "Deleting a partition results in data loss"
		if dialog, err := CreateWarningDialogBox(message); err != nil {
			log.Warning("%s: %s", message, err)
		} else {
			dialog.OnClose(func() {
				page.GotoPage(TuiPageDiskConfig)
			})
		}
	})

	page.cancelBtn = CreateSimpleButton(btnFrm, AutoSize, AutoSize, "Cancel", Fixed)
	page.cancelBtn.OnClick(func(ev clui.Event) {
		page.GotoPage(TuiPageDiskConfig)
	})

	page.activated = page.fsList
	return page, nil
}
