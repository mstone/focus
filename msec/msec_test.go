// Copyright 2014 Akamai Technologies, Inc.
// Please see the accompanying LICENSE file for licensing information.

package msec

import (
	"testing"
	"time"
)

func TestFromZero(t *testing.T) {
	t.Parallel()

	//zero := time.Time{}
	zero := time.Unix(0, 0)

	t.Logf("zUnix: %d, zUnixNano: %d", zero.Unix(), zero.UnixNano())

	msecs := From(zero)

	if msecs != 0 {
		t.Fatalf("got non-zero msecs: %v", msecs)
	}
}

func TestTimeZero(t *testing.T) {
	t.Parallel()

	var zero int64 = 0

	tv := Time(zero)

	//if !tv.Equal(time.Time{}) {
	if !tv.Equal(time.Unix(0, 0)) {
		t.Fatalf("got non-zero time: %v", tv)
	}
}

func TestFrom(t *testing.T) {
	t.Parallel()

	now := time.Unix(11, 5)

	t.Logf("nUnix: %d, nUnixNano: %d", now.Unix(), now.UnixNano())

	msec := From(now)
	t.Logf("msec: %d", msec)

	now2 := Time(msec)

	msec2 := From(now2)
	t.Logf("msec2: %d", msec2)

	t.Logf("2Unix: %d, 2UnixNano: %d", now2.Unix(), now2.UnixNano())

	if now.Sub(now2) > time.Millisecond || now2.Sub(now) > time.Millisecond {
		t.Fatalf("Time(From(now)) lost time: %v", int64(now.Sub(now2)))
	}
}
