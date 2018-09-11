package tui

import (
	"fmt"
	"unicode"

	"github.com/VladimirMarkelov/clui"
	"github.com/clearlinux/clr-installer/errors"
)

// TabGroup represents a tab group and holds logically grouped tabs
type TabGroup struct {
	mainFrame    clui.Control // the widget content frame
	btnsFrame    clui.Control // this frame holds the buttons elements of the tab component
	contentFrame clui.Control // where the contents are stacked
	pages        []*TabPage   // represents the pages - each page contains a button and its content frame
	selected     *TabPage     // currently selected page
	height       int          // the menu frame height
	paddingX     int          // the menu X padding
	window       clui.Control // the window the tab was added to
}

// TabPage represents an individual element containing basically a hotkey, button and content
type TabPage struct {
	hotKey     rune          // the kotkey char
	label      string        // the button label (non formated)
	btn        *SimpleButton // the tab button
	frame      *clui.Frame   // the content frame
	activeMenu clui.Control
}

// TabKeyCb holds the contexts for the keybinds handling iterations
type TabKeyCb struct {
	tab *TabGroup
}

const (
	tabButtonHeight     = 3
	keyBindingDescWidth = 4
	tabPadding          = 4
)

// NewTabGroup allocates a new TabGroup object and initializes the basic frames and triggers
// the events pooling
func NewTabGroup(parent clui.Control, paddingX int, height int) *TabGroup {
	mainFrame := clui.CreateFrame(parent, AutoSize, height, BorderNone, Fixed)
	mainFrame.SetPack(clui.Vertical)

	btnsFrame := clui.CreateFrame(mainFrame, AutoSize, tabButtonHeight, BorderNone, Fixed)
	btnsFrame.SetStyle("Tab")

	contentFrame := clui.CreateFrame(mainFrame, AutoSize, height, BorderNone, Fixed)
	contentFrame.SetPack(clui.Vertical)

	res := &TabGroup{
		btnsFrame:    btnsFrame,
		mainFrame:    mainFrame,
		contentFrame: contentFrame,
		height:       height,
		paddingX:     paddingX,
	}

	ctrl := parent
	for ctrl.Parent() != nil {
		ctrl = ctrl.Parent()
	}

	wnd := ctrl.(*clui.Window)
	wnd.OnKeyDown(keyEventCb, &TabKeyCb{tab: res})

	res.window = wnd

	return res
}

// AddTab allocates a new TagPage, creates button and frame accordingly
func (tg *TabGroup) AddTab(label string, hotKey rune) (*TabPage, error) {
	for _, curr := range tg.pages {
		if curr.hotKey == hotKey {
			return nil, errors.Errorf("Duplicated hotkey between %s and %s", curr.label, label)
		}
	}

	width := len(label) + keyBindingDescWidth + tabPadding
	page := &TabPage{hotKey: hotKey, label: label}

	active := len(tg.pages) == 0
	tg.pages = append(tg.pages, page)

	flabel := fmt.Sprintf("[%c] %s", unicode.To(unicode.UpperCase, hotKey), label)

	page.btn = CreateSimpleButton(tg.btnsFrame, width, 3, flabel, Fixed)
	page.btn.SetForceActiveStyle(active)
	page.btn.SetAlign(AlignLeft)
	page.btn.SetPaddings(2, 1)
	page.btn.SetStyle("Tab")
	page.btn.SetTabStop(false)

	page.btn.OnClick(func(clui.Event) {
		var hidden []*TabPage

		for _, curr := range tg.pages {
			if curr == page {
				continue
			}

			hidden = append(hidden, curr)
		}

		tg.setSelected(page, hidden)
	})

	page.frame = clui.CreateFrame(tg.contentFrame, AutoSize, tg.height, BorderNone, Fixed)
	page.frame.SetPack(clui.Vertical)
	page.frame.SetPaddings(tg.paddingX, 1)
	page.frame.SetScrollable(true)

	if active {
		tg.selected = page
	} else {
		page.frame.SetVisible(false)
	}

	return page, nil
}

func keyEventCb(ev clui.Event, data interface{}) bool {
	cbData := data.(*TabKeyCb)

	var selected *TabPage
	var hidden []*TabPage

	for _, curr := range cbData.tab.pages {
		if ev.Ch == curr.hotKey || ev.Ch == unicode.ToUpper(curr.hotKey) {
			selected = curr
		} else {
			hidden = append(hidden, curr)
		}
	}

	if selected == nil {
		return false
	}

	cbData.tab.setSelected(selected, hidden)
	return true
}

func (tg *TabGroup) setSelected(selected *TabPage, hidden []*TabPage) {
	if selected == nil || tg.selected == selected {
		return
	}

	for _, curr := range hidden {
		if curr.frame != nil {
			curr.activeMenu = clui.ActiveControl(curr.frame)
			curr.frame.SetVisible(false)
		}
		curr.btn.SetForceActiveStyle(false)
	}

	selected.btn.SetForceActiveStyle(true)
	tg.selected = selected

	selected.frame.SetVisible(true)

	activeMenu := selected.activeMenu
	if activeMenu == nil {
		for _, curr := range selected.frame.Children() {
			if _, ok := curr.(*MenuButton); ok {
				activeMenu = curr
				break
			}
		}
	}

	selected.activeMenu = activeMenu
	clui.ActivateControl(tg.window, activeMenu)
	activeMenu.SetActive(true)
}

// SetActive sets the nth page of a TabGroup as active
func (tg *TabGroup) SetActive(idx int) error {
	if idx > len(tg.pages)-1 || idx < 0 {
		return errors.Errorf("Invalid page index: %d", idx)
	}

	for i, curr := range tg.pages {
		curr.btn.SetForceActiveStyle(i == idx)
	}

	return nil
}

// GetVisibleFrame returns the visible page's content frame
func (tg *TabGroup) GetVisibleFrame() *clui.Frame {
	for _, curr := range tg.pages {
		if curr.IsVisible() {
			return curr.frame
		}
	}
	return nil
}

// IsVisible returns true if a given tab page is visible, and false otherwise
func (tp *TabPage) IsVisible() bool {
	return tp.frame.Visible()
}
