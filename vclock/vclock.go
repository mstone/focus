// Copyright 2014 Akamai Technologies, Inc.
// Please see the accompanying LICENSE file for licensing information.

package vclock

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type Ordering [6]bool

func (o Ordering) String() string {
	s := []string{}
	if o[ORDER_LT] {
		s = append(s, "LT")
	}
	if o[ORDER_LE] {
		s = append(s, "LE")
	}
	if o[ORDER_EQ] {
		s = append(s, "EQ")
	}
	if o[ORDER_GE] {
		s = append(s, "GE")
	}
	if o[ORDER_GT] {
		s = append(s, "GT")
	}
	if o[ORDER_NC] {
		s = append(s, "NC")
	}
	return strings.Join(s, " ")
}

type OrderingFlags int

const (
	ORDER_LT OrderingFlags = iota
	ORDER_LE
	ORDER_EQ
	ORDER_GE
	ORDER_GT
	ORDER_NC // incomparable
)

type Clock map[int]int

type ByKey [][2]int

func (b ByKey) Less(i, j int) bool {
	return b[i][0] < b[j][0]
}

func (b ByKey) Swap(i, j int) {
	b[i][0], b[i][1], b[j][0], b[j][1] = b[j][0], b[j][1], b[i][0], b[i][1]
}

func (b ByKey) Len() int {
	return len(b)
}

func asSlice(c Clock) [][2]int {
	b := ByKey{}
	for k, v := range c {
		b = append(b, [2]int{k, v})
	}
	sort.Sort(b)
	return b
}

func (c Clock) String() string {
	s := asSlice(c)
	ret := make([]string, len(s))
	for i, v := range s {
		ret[i] = fmt.Sprintf("[%d %d]", v[0], v[1])
	}
	return "[" + strings.Join(ret, " ") + "]"
}

func (c Clock) MarshalJSON() ([]byte, error) {
	return json.Marshal(asSlice(c))
}

func (c *Clock) UnmarshalJSON(data []byte) error {
	var err error
	var slice [][2]int

	if err = json.Unmarshal(data, &slice); err != nil {
		return err
	}

	m := make(map[int]int, len(slice))
	for _, pair := range slice {
		m[pair[0]] = pair[1]
	}
	*c = m
	return nil
}

func New() Clock {
	return Clock{}
}

func (c Clock) Tick(site int) {
	c[site]++
}

func (c Clock) Get(site int) (int, bool) {
	val, ok := c[site]
	return val, ok
}

func (c Clock) Set(site int, tick int) {
	c[site] = tick
}

func (c Clock) Max() int {
	epoch := 0
	for _, ticks := range c {
		if ticks > epoch {
			epoch = ticks
		}
	}
	return epoch
}

func (c Clock) Copy() Clock {
	m := make(map[int]int, len(c))
	for k, v := range c {
		m[k] = v
	}
	return m
}

func Compare(l, r Clock) Ordering {
	ret := [6]bool{true, true, true, true, true, true}
	maybeLT, maybeGT := false, false
	for k, v1 := range l {
		if v2, ok := r[k]; ok {
			switch {
			case v1 > v2:
				maybeGT = true
				ret[ORDER_LT] = false
				ret[ORDER_LE] = false
				ret[ORDER_EQ] = false
			case v1 < v2:
				maybeLT = true
				ret[ORDER_GT] = false
				ret[ORDER_GE] = false
				ret[ORDER_EQ] = false
			}
		} else {
			ret[ORDER_LT] = false
			ret[ORDER_LE] = false
			ret[ORDER_EQ] = false
		}
	}
	for k, v2 := range r {
		if v1, ok := l[k]; ok {
			switch {
			case v1 > v2:
				maybeGT = true
				ret[ORDER_LT] = false
				ret[ORDER_LE] = false
				ret[ORDER_EQ] = false
			case v1 < v2:
				maybeLT = true
				ret[ORDER_GT] = false
				ret[ORDER_GE] = false
				ret[ORDER_EQ] = false
			}
		} else {
			ret[ORDER_GT] = false
			ret[ORDER_GE] = false
			ret[ORDER_EQ] = false
		}
	}

	ret[ORDER_LT] = ret[ORDER_LT] && maybeLT
	ret[ORDER_GT] = ret[ORDER_GT] && maybeGT

	numTrue := 0
	for _, v := range ret[0:5] {
		if v {
			numTrue++
		}
	}
	ret[ORDER_NC] = numTrue == 0
	return ret
}
