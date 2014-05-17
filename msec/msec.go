// Copyright 2014 Akamai Technologies, Inc.
// Please see the accompanying LICENSE file for licensing information.

// Package msec converts time.Times to and from int64-sized counters of msecs
// since the Unix epoch.
package msec

import (
	"time"
)

func From(t time.Time) int64 {
	return t.UnixNano() / 1e6
}

func Time(ms int64) time.Time {
	return time.Unix(0, ms*1e6)
}
