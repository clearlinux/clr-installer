package pages

import (
	"github.com/clearlinux/clr-installer/model"
	"github.com/clearlinux/clr-installer/utils"
	"github.com/gotk3/gotk3/gtk"
)

// NetworkCheckPage is an empty page that creates a menu entry that triggers a network check.
type NetworkCheckPage struct {
}

// NewNetworkPage only exists to add a menu entry that creates a network test pop-up.
func NewNetworkPage(ctlr Controller, model *model.SystemInstall) (Page, error) {
	return &NetworkCheckPage{}, nil
}

// ResetChanges will reset this page to match the model
func (page *NetworkCheckPage) ResetChanges() {
	return
}

// IsDone checks if all the steps are completed
func (page *NetworkCheckPage) IsDone() bool {
	return true
}

// IsRequired will return false as we have default values
func (page *NetworkCheckPage) IsRequired() bool {
	return false
}

// GetID returns the ID for this page
func (page *NetworkCheckPage) GetID() int {
	return PageIDNetwork
}

// GetIcon returns the icon for this page
func (page *NetworkCheckPage) GetIcon() string {
	return "network-wireless-symbolic"
}

// GetRootWidget returns the root embeddable widget for this page
func (page *NetworkCheckPage) GetRootWidget() gtk.IWidget {
	return nil
}

// GetSummary will return the summary for this page
func (page *NetworkCheckPage) GetSummary() string {
	return utils.Locale.Get("Test Network Settings")
}

// GetTitle will return the title for this page
func (page *NetworkCheckPage) GetTitle() string {
	return page.GetSummary()
}

// StoreChanges will store this pages changes into the model
func (page *NetworkCheckPage) StoreChanges() {
	return
}

// GetConfiguredValue returns our current config
func (page *NetworkCheckPage) GetConfiguredValue() string {
	return utils.Locale.Get("Test connectivity")
}
