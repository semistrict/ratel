// Copyright 2023 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package ctxutil

import (
	"context"
)

// WhenDoneFunc is the callback invoked by context when it becomes done.
// The callback is passed the error from the parent context.
type WhenDoneFunc func(err error)

// WhenDoneCauseFunc accepts context error (context.Err()) as well
// as the cause for cancellation.
type WhenDoneCauseFunc func(err, cause error)

// WhenDone arranges for the specified function to be invoked when
// parent context becomes done and returns true.
// If the context is non-cancellable (i.e. `Done() == nil`), returns false and
// never calls the function.
func WhenDone(parent context.Context, done WhenDoneFunc) bool {
	if parent.Done() == nil {
		return false
	}
	context.AfterFunc(parent, func() {
		done(parent.Err())
	})
	return true
}

// WhenDoneCause is the same as WhenDone, but accepts WhenDoneCauseFunc as a
// callback.
func WhenDoneCause(parent context.Context, done WhenDoneCauseFunc) bool {
	if parent.Done() == nil {
		return false
	}
	context.AfterFunc(parent, func() {
		done(parent.Err(), context.Cause(parent))
	})
	return true
}

// CanDirectlyDetectCancellation reports whether parent exposes a Done
// channel, which is a prerequisite for WhenDone/WhenDoneCause to install
// a callback without leaking a watcher goroutine.
func CanDirectlyDetectCancellation(parent context.Context) bool {
	return parent.Done() != nil
}
