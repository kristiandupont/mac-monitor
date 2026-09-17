package notify

// Callbacks from notify.m. Kept apart from notify.go because a file with
// //export may only have declarations in its cgo preamble.

import "C"

//export mmNotifyDone
func mmNotifyDone(reqID C.longlong, err *C.char) {
	if err == nil {
		complete(int64(reqID), "", false)
		return
	}
	complete(int64(reqID), C.GoString(err), true)
}

//export mmNotifyAction
func mmNotifyAction(action_, subject *C.char) {
	action(C.GoString(action_), C.GoString(subject))
}
