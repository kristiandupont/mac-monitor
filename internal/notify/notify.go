// Package notify posts macOS notifications.
package notify

/*
#cgo LDFLAGS: -framework Foundation -framework UserNotifications
#include <stdlib.h>
#include "notify.h"
*/
import "C"

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"unsafe"
)

var (
	nextID  atomic.Int64
	pending sync.Map // int64 → chan error

	onIgnore func(subject string)
)

// bundled reports whether the native API can be used. Outside an app bundle
// (e.g. `go run`) UNUserNotificationCenter throws, so osascript is used.
func bundled() bool { return C.mmNotifyAvailable() != 0 }

// Init must be called before the Cocoa run loop starts. ignore is called when
// the user picks "Don't Alert for This App" on a notification. With
// requestAuth, macOS asks for permission now rather than at the first alert
// (when the prompt would hold up delivery).
func Init(ignore func(subject string), requestAuth bool) {
	onIgnore = ignore
	if bundled() {
		auth := C.int(0)
		if requestAuth {
			auth = 1
		}
		C.mmNotifyInit(auth)
	}
}

// Send posts a notification and returns once macOS has accepted it (which
// is as much as the API can tell us). id identifies what the notification
// is about; a newer one with the same id replaces the older.
// category "cpu" adds the ignore action; subject is passed back to it.
func Send(ctx context.Context, id, title, body, category, subject string) error {
	if !bundled() {
		return sendOsascript(ctx, title, body)
	}
	reqID := nextID.Add(1)
	done := make(chan error, 1)
	pending.Store(reqID, done)
	defer pending.Delete(reqID)

	cstrs := []*C.char{C.CString(id), C.CString(title), C.CString(body), C.CString(category), C.CString(subject)}
	C.mmNotifySend(C.longlong(reqID), cstrs[0], cstrs[1], cstrs[2], cstrs[3], cstrs[4])
	for _, s := range cstrs {
		C.free(unsafe.Pointer(s))
	}

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func complete(reqID int64, errMsg string, failed bool) {
	ch, ok := pending.Load(reqID)
	if !ok {
		return // timed out already
	}
	var err error
	if failed {
		err = errors.New(errMsg)
	}
	ch.(chan error) <- err
}

func action(id, subject string) {
	if id == "ignore" && subject != "" && onIgnore != nil {
		go onIgnore(subject)
	}
}

func sendOsascript(ctx context.Context, title, body string) error {
	out, err := exec.CommandContext(ctx, "osascript",
		"-e", "on run argv",
		"-e", "display notification (item 2 of argv) with title (item 1 of argv)",
		"-e", "end run",
		title, body,
	).CombinedOutput()
	if err != nil {
		if msg := strings.TrimSpace(string(out)); msg != "" {
			return errors.New("osascript: " + msg)
		}
		return err
	}
	return nil
}
