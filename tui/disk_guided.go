// Copyright © 2018 Intel Corporation
//
// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"fmt"

	"github.com/clearlinux/clr-installer/storage"

	"github.com/VladimirMarkelov/clui"
)

// GuidedPartPage is the Page implementation for guided partitioning page
type GuidedPartPage struct {
	BasePage
	bd       *storage.BlockDevice
	bdFrames []*clui.Frame
}

const (
	guidedDesc = `Select a partition to modify its configuration and to define it as the
target installation disk.`
)

// SetDone adds a new target media to installation model and sets the previous' page done flag
func (page *GuidedPartPage) SetDone(done bool) bool {
	var selected *storage.BlockDevice

	bds, err := storage.ListAvailableBlockDevices(page.getModel().TargetMedias)
	if err != nil {
		page.Panic(err)
	}

	for _, curr := range bds {
		if !curr.Equals(page.bd) {
			continue
		}

		selected = curr
		break
	}

	selected.Children = page.bd.Children
	page.getModel().AddTargetMedia(selected)
	page.bd = nil

	diskPage := page.tui.getPage(TuiPageDiskMenu)
	diskPage.SetDone(done)

	// TODO start using new API page.GotoPage() when finished merging
	// disk pages
	page.tui.gotoPage(TuiPageMenu, diskPage)

	return false
}

func (page *GuidedPartPage) showGuidedDisk(bd *storage.BlockDevice) error {
	size, err := bd.HumanReadableSizeWithPrecision(1)
	if err != nil {
		return err
	}

	frame := clui.CreateFrame(page.content, AutoSize, AutoSize, BorderNone, clui.Fixed)
	frame.SetPack(clui.Vertical)
	page.bdFrames = append(page.bdFrames, frame)

	mm := fmt.Sprintf("(%s)", bd.MajorMinor)
	lbl := fmt.Sprintf("%s %s %s %s", bd.Model, bd.Name, mm, size)

	btn := CreateSimpleButton(frame, AutoSize, AutoSize, lbl, Fixed)
	btn.SetAlign(AlignLeft)

	labels := []*clui.Label{}
	btn.OnClick(func(ev clui.Event) {
		storage.NewStandardPartitions(bd)

		for _, curr := range labels {
			curr.Destroy()
		}

		labels = []*clui.Label{}
		for _, part := range bd.Children {
			lbl, err := showGuidedPartition(frame, part)
			if err != nil {
				page.Panic(err)
			}

			labels = append(labels, lbl)
		}

		page.doneBtn.SetEnabled(true)
		clui.ActivateControl(page.window, page.doneBtn)
		page.bd = bd
	})

	for _, part := range bd.Children {
		lbl, err := showGuidedPartition(frame, part)
		if err != nil {
			return nil
		}

		labels = append(labels, lbl)
	}

	return nil
}

func showGuidedPartition(frame *clui.Frame, part *storage.BlockDevice) (*clui.Label, error) {
	size, err := part.HumanReadableSize()
	if err != nil {
		return nil, err
	}

	txt := fmt.Sprintf("%10s %10s %s %s", part.Name, size, part.FsType, part.MountPoint)
	return clui.CreateLabel(frame, AutoSize, 1, txt, Fixed), nil
}

// Activate updates the UI elements with the most current list of block devices
func (page *GuidedPartPage) Activate() {
	for _, curr := range page.bdFrames {
		curr.Destroy()
	}
	page.bdFrames = []*clui.Frame{}

	bds, err := storage.ListAvailableBlockDevices(page.getModel().TargetMedias)
	if err != nil {
		page.Panic(err)
	}

	for _, bd := range bds {
		if err = page.showGuidedDisk(bd.Clone()); err != nil {
			page.Panic(err)
		}
	}
}

func newGuidedPartitionPage(tui *Tui) (Page, error) {
	page := &GuidedPartPage{}
	page.setup(tui, TuiPageGuidedPart, AllButtons, TuiPageMenu)

	lbl := clui.CreateLabel(page.content, 2, 2, "Guided Partition", Fixed)
	lbl.SetPaddings(0, 2)

	lbl = clui.CreateLabel(page.content, 70, 3, guidedDesc, Fixed)
	lbl.SetMultiline(true)

	page.doneBtn.SetEnabled(false)
	return page, nil
}
