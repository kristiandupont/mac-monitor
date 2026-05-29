package tray

// exports.go must have a minimal preamble — //export and import "C" preambles
// cannot coexist in the same file per CGo rules.

import "C"

// menuClickCh receives item IDs from Objective-C menu callbacks.
var menuClickCh = make(chan int, 4)

const (
	menuItemOpen = 1
	menuItemQuit = 2
)

//export onMenuItemClicked
func onMenuItemClicked(itemID C.int) {
	select {
	case menuClickCh <- int(itemID):
	default:
	}
}
