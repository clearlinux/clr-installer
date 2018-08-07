// Copyright © 2018 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"fmt"

	"github.com/clearlinux/clr-installer/storage"

	"github.com/VladimirMarkelov/clui"
	"github.com/nsf/termbox-go"
)

// ManualPartPage is the Page implementation for manual partitioning page
type ManualPartPage struct {
	BasePage
	bds  []*storage.BlockDevice
	btns []*SimpleButton
}

// SelectedBlockDevice holds the shared date between the manual partitioning page and
// the partition configuration page
type SelectedBlockDevice struct {
	bd      *storage.BlockDevice
	part    *storage.BlockDevice
	addMode bool
}

const (
	manualDesc = `Select a partition to modify its configuration and to define it as the
target installation disk.`
)

var (
	partBtnBg termbox.Attribute
)

func (page *ManualPartPage) showManualDisk(bd *storage.BlockDevice, frame *clui.Frame) error {
	size, err := bd.HumanReadableSizeWithPrecision(1)
	if err != nil {
		return err
	}

	mm := fmt.Sprintf("(%s)", bd.MajorMinor)
	lbl := fmt.Sprintf("%s %s %s %s", bd.Model, bd.Name, mm, size)

	btn := CreateSimpleButton(frame, AutoSize, AutoSize, lbl, Fixed)
	btn.SetAlign(AlignLeft)

	page.btns = append(page.btns, btn)
	lfs := storage.LargestFileSystemName()

	for _, part := range bd.Children {
		sel := &SelectedBlockDevice{bd: bd, part: part, addMode: false}

		size, err = sel.part.HumanReadableSize()
		if err != nil {
			return err
		}

		// builds the fsmask to align fstype column, also give 2 char padding
		fsMask := fmt.Sprintf("%%10s %%10s %%%ds %%s", lfs+2)

		txt := fmt.Sprintf(fsMask, sel.part.Name, size, sel.part.FsType,
			sel.part.MountPoint)

		btn = page.newPartBtn(frame, txt)
		btn.OnClick(func(ev clui.Event) {
			page.data = sel
			page.GotoPage(TuiPageDiskPart)
		})
	}

	freeSpace, err := bd.FreeSpace()
	if err != nil {
		return err
	}

	freeSpaceLbl, err := storage.HumanReadableSize(freeSpace)
	if err != nil {
		return err
	}

	btn = page.newPartBtn(frame, fmt.Sprintf("%16s: %s", "Free space", freeSpaceLbl))
	if freeSpace > 0 {
		btn.OnClick(func(ev clui.Event) {
			newPart := &storage.BlockDevice{
				FsType:     "ext4",
				MountPoint: "",
				Size:       freeSpace,
				Parent:     bd,
			}
			bd.AddChild(newPart)
			page.data = &SelectedBlockDevice{bd: bd, part: newPart, addMode: true}
			page.GotoPage(TuiPageDiskPart)
		})
	}

	return nil
}

func (page *ManualPartPage) newPartBtn(frame *clui.Frame, label string) *SimpleButton {
	btn := CreateSimpleButton(frame, AutoSize, AutoSize, label, Fixed)
	btn.SetStyle("Part")
	btn.SetAlign(AlignLeft)
	btn.SetBackColor(partBtnBg)

	page.btns = append(page.btns, btn)
	return btn
}

func (page *ManualPartPage) showManualStorageList() error {
	for _, bd := range page.bds {
		if err := page.showManualDisk(bd.Clone(), page.content); err != nil {
			return err
		}
	}

	return nil
}

// Activate is called when the manual disk partitioning page is activated and resets the
// page's displayed data
func (page *ManualPartPage) Activate() {
	var err error
	var selected *storage.BlockDevice

	if sel, ok := page.data.(*SelectedBlockDevice); ok {
		selected = sel.bd
	}

	bds, err := storage.ListAvailableBlockDevices(page.getModel().TargetMedias)
	if err != nil {
		page.Panic(err)
	}

	nList := []*storage.BlockDevice{}

	for _, curr := range bds {
		if curr.Equals(selected) {
			nList = append(nList, selected)
		} else {
			nList = append(nList, curr)
		}
	}

	page.bds = nList

	for _, curr := range page.btns {
		curr.Destroy()
	}

	if err = page.showManualStorageList(); err != nil {
		page.Panic(err)
	}

	for _, bd := range page.bds {
		if err = bd.Validate(); err == nil {
			page.doneBtn.SetEnabled(true)
		}
	}
}

// SetDone set's the configured disk into the model and sets the previous page
// as done
func (page *ManualPartPage) SetDone(done bool) bool {
	if sel, ok := page.data.(*SelectedBlockDevice); ok {
		var selected *storage.BlockDevice

		bds, err := storage.ListAvailableBlockDevices(page.getModel().TargetMedias)
		if err != nil {
			page.Panic(err)
		}

		for _, curr := range bds {
			if !curr.Equals(sel.bd) {
				continue
			}

			selected = curr
			break
		}

		selected.Children = sel.bd.Children
		page.getModel().AddTargetMedia(selected)
		page.data = nil
	}

	diskPage := page.tui.getPage(TuiPageDiskMenu)
	diskPage.SetDone(done)

	// TODO start using new API page.GotoPage() when finished merging
	// disk pages
	page.tui.gotoPage(TuiPageMenu, diskPage)
	return false
}

func newManualPartitionPage(tui *Tui) (Page, error) {
	partBtnBg = clui.RealColor(clui.ColorDefault, "ManualPartition", "Back")

	page := &ManualPartPage{}
	page.setup(tui, TuiPageManualPart, AllButtons, TuiPageMenu)

	lbl := clui.CreateLabel(page.content, 2, 2, "Manual Partition", Fixed)
	lbl.SetPaddings(0, 2)

	lbl = clui.CreateLabel(page.content, 70, 3, manualDesc, Fixed)
	lbl.SetMultiline(true)

	page.doneBtn.SetEnabled(false)
	return page, nil
}
