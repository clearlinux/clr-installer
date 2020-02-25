// Same copyright and license as the rest of the files in this project
// +build !gtk_3_6,!gtk_3_8,!gtk_3_10,!gtk_3_12

package gtk

// #include <stdlib.h>
// #include <gtk/gtk.h>
// #include "gtk_since_3_14.go.h"
import "C"
import (
	"sync"
	"unsafe"

	"github.com/gotk3/gotk3/glib"
)

/*
 * Constants
 */

const (
	STATE_FLAG_CHECKED StateFlags = C.GTK_STATE_FLAG_CHECKED
)

/*
 * GtkStack
 */

// TODO:
// GtkStackTransitionType
// GTK_STACK_TRANSITION_TYPE_OVER_DOWN_UP
// GTK_STACK_TRANSITION_TYPE_OVER_LEFT_RIGHT
// GTK_STACK_TRANSITION_TYPE_OVER_RIGHT_LEFT

/*
 * GtkListBox
 */

// UnselectRow is a wrapper around gtk_list_box_unselect_row().
func (v *ListBox) UnselectRow(row *ListBoxRow) {
	C.gtk_list_box_unselect_row(v.native(), row.native())
}

// SelectAll is a wrapper around gtk_list_box_select_all().
func (v *ListBox) SelectAll() {
	C.gtk_list_box_select_all(v.native())
}

// UnselectAll is a wrapper around gtk_list_box_unselect_all().
func (v *ListBox) UnselectAll() {
	C.gtk_list_box_unselect_all(v.native())
}

type ListBoxForeachFunc func(box *ListBox, row *ListBoxRow, userData uintptr) int

type listBoxForeachFuncData struct {
	fn       ListBoxForeachFunc
	userData uintptr
}

var (
	listBoxForeachFuncRegistry = struct {
		sync.RWMutex
		next int
		m    map[int]listBoxForeachFuncData
	}{
		next: 1,
		m:    make(map[int]listBoxForeachFuncData),
	}
)

// SelectedForeach is a wrapper around gtk_list_box_selected_foreach().
func (v *ListBox) SelectedForeach(fn ListBoxForeachFunc, userData uintptr) {
	listBoxForeachFuncRegistry.Lock()
	id := listBoxForeachFuncRegistry.next
	listBoxForeachFuncRegistry.next++
	listBoxForeachFuncRegistry.m[id] = listBoxForeachFuncData{fn: fn, userData: userData}
	listBoxForeachFuncRegistry.Unlock()

	C._gtk_list_box_selected_foreach(v.native(), C.gpointer(uintptr(id)))

	listBoxForeachFuncRegistry.Lock()
	delete(listBoxForeachFuncRegistry.m, id)
	listBoxForeachFuncRegistry.Unlock()
}

// GetSelectedRows is a wrapper around gtk_list_box_get_selected_rows().
func (v *ListBox) GetSelectedRows() *glib.List {
	clist := C.gtk_list_box_get_selected_rows(v.native())
	if clist == nil {
		return nil
	}

	glist := glib.WrapList(uintptr(unsafe.Pointer(clist)))
	glist.DataWrapper(func(ptr unsafe.Pointer) interface{} {
		return wrapListBoxRow(glib.Take(ptr))
	})

	return glist
}

/*
 * GtkListBoxRow
 */

// IsSelected is a wrapper around gtk_list_box_row_is_selected().
func (v *ListBoxRow) IsSelected() bool {
	c := C.gtk_list_box_row_is_selected(v.native())
	return gobool(c)
}

// SetActivatable is a wrapper around gtk_list_box_row_set_activatable().
func (v *ListBoxRow) SetActivatable(activatable bool) {
	C.gtk_list_box_row_set_activatable(v.native(), gbool(activatable))
}

// GetActivatable is a wrapper around gtk_list_box_row_get_activatable().
func (v *ListBoxRow) GetActivatable() bool {
	c := C.gtk_list_box_row_get_activatable(v.native())
	return gobool(c)
}

// SetSelectable is a wrapper around gtk_list_box_row_set_selectable().
func (v *ListBoxRow) SetSelectable(selectable bool) {
	C.gtk_list_box_row_set_selectable(v.native(), gbool(selectable))
}

// GetSelectable is a wrapper around gtk_list_box_row_get_selectable().
func (v *ListBoxRow) GetSelectable() bool {
	c := C.gtk_list_box_row_get_selectable(v.native())
	return gobool(c)
}

/*
 * GtkPlacesSidebar
 */

// TODO:
// gtk_places_sidebar_get_show_enter_location().
// gtk_places_sidebar_set_show_enter_location().

/*
 * GtkSwitch
 */

// TODO:
// gtk_switch_set_state().
// gtk_switch_get_state().
