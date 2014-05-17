// Copyright 2014 Akamai Technologies, Inc.
// Please see the accompanying LICENSE file for licensing information.

package vclock

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestJSON(t *testing.T) {
	c1 := Clock{}
	c1.Tick(0)

	j1, err := json.Marshal(c1)
	if err != nil {
		t.Fatalf("unable to marshal c1 to json, err %q", err)
	}

	var c2 Clock
	err = json.Unmarshal(j1, &c2)
	if err != nil {
		t.Fatalf("unable to unmarshal c2 from j1, err: %q", err)
	}

	ret := Compare(c1, c2)
	if !reflect.DeepEqual(ret, Ordering{false, true, true, true, false, false}) {
		t.Fatalf("expected c1 == c2; got ret: %s, c1: %s, c2: %s", ret, c1, c2)
	}
}
