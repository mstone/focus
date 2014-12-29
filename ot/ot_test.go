// Copyright 2014 Akamai Technologies, Inc.
// Please see the accompanying LICENSE file for licensing information.

package ot

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestJSON(t *testing.T) {
	c1 := Ops{Op{Size: -1}, Op{Size: 1}, Op{Body: AsSlice("hi")}}

	j1, err := json.Marshal(c1)
	if err != nil {
		t.Fatalf("unable to marshal c1 to json, err %q", err)
	}

	t.Logf("j1: %s", j1)

	var c2 Ops
	err = json.Unmarshal(j1, &c2)
	if err != nil {
		t.Fatalf("unable to unmarshal c2 from j1, err: %q", err)
	}

	if !reflect.DeepEqual(c1, c2) {
		t.Fatalf("expected c1 == c2; got c1: %s, c2: %s", c1, c2)
	}
}

type Applier interface {
	Apply(r *Doc) Ops
}

type Ins struct {
	Pos int
	Str string
}

func (i Ins) Apply(r *Doc) Ops {
	ops := NewInsert(r.Len(), i.Pos, i.Str)
	r.Apply(ops)
	return ops
}

type Del struct {
	Pos int
	Len int
}

func (d Del) Apply(r *Doc) Ops {
	ops := NewDelete(r.Len(), d.Pos, d.Len)
	r.Apply(ops)
	return ops
}

type A Applier

type TestCase struct {
	First [2][]A
	Then  [2][]A
	Rest  [2][]A
}

func Tee(ops []A) [2][]A {
	return [2][]A{ops, ops}
}

func isOk(t *testing.T, r1, r2 *Doc) {
	if r1.String() != r2.String() {
		t.Fatalf("r1 != r2\nr1: %q\nr2: %q", r1, r2)
	}
}

func doEpoch(t *testing.T, r1, r2 *Doc, trace [2][]A) {
	b1 := Ops{}
	b2 := Ops{}
	for _, o := range trace[0] {
		b1 = Compose(b1, o.Apply(r1))
	}
	for _, o := range trace[1] {
		b2 = Compose(b2, o.Apply(r2))
	}
	c1, c2 := Transform(b1, b2)
	r1.Apply(c2)
	r2.Apply(c1)
	isOk(t, r1, r2)
}

func doTable(t *testing.T, table []TestCase) {
	for idx, test := range table {
		t.Logf("running test case %d", idx)
		r1 := NewDoc()
		r2 := NewDoc()

		doEpoch(t, r1, r2, test.First)
		doEpoch(t, r1, r2, test.Then)
		doEpoch(t, r1, r2, test.Rest)
	}
}

func TestSerial(t *testing.T) {
	table := []TestCase{
		TestCase{
			First: Tee([]A{Ins{0, "a"}, Ins{0, "b"}}),
			Then:  [2][]A{[]A{Ins{0, "c"}}, []A{Ins{0, "d"}}},
			Rest:  Tee([]A{Ins{1, "e"}}),
		},
		TestCase{
			First: Tee([]A{Ins{0, "a"}, Del{0, 1}}),
			Then:  [2][]A{[]A{Ins{0, "c"}}, []A{Ins{0, "d"}}},
			Rest:  Tee([]A{Ins{1, "e"}}),
		},
		TestCase{
			First: Tee([]A{Ins{0, "a"}, Ins{0, "b"}}),
			Then:  [2][]A{[]A{Ins{0, "c"}}, []A{Del{0, 1}}},
			Rest:  Tee([]A{Ins{1, "e"}}),
		},
		TestCase{
			First: Tee([]A{Ins{0, "a"}, Ins{0, "b"}}),
			Then:  [2][]A{[]A{Del{0, 1}}, []A{Del{0, 1}}},
			Rest:  Tee([]A{Ins{1, "e"}}),
		},
		TestCase{
			First: Tee([]A{Ins{0, "a"}, Ins{0, "b"}}),
			Then:  [2][]A{[]A{Del{0, 1}}, []A{Del{1, 1}}},
			Rest:  Tee([]A{Ins{1, "e"}}),
		},
		TestCase{
			First: Tee([]A{Ins{0, "a"}, Ins{0, "b"}}),
			Then:  [2][]A{[]A{Del{0, 1}}, []A{Ins{1, "c"}}},
			Rest:  Tee([]A{Ins{1, "e"}}),
		},
		TestCase{
			First: Tee([]A{Ins{0, "a"}, Ins{0, "b"}, Del{0, 1}}),
			Then:  [2][]A{[]A{Del{0, 1}}, []A{Ins{1, "c"}}},
			Rest:  Tee([]A{Ins{1, "e"}}),
		},
		TestCase{
			First: Tee([]A{Ins{0, "a"}, Ins{1, "b"}, Del{0, 1}, Ins{1, "c"}}),
			Then:  [2][]A{[]A{Del{1, 1}}, []A{Ins{0, "c"}}},
			Rest:  Tee([]A{Ins{2, "e"}}),
		},
	}

	doTable(t, table)
}

func TestConcurrent(t *testing.T) {
	table := []TestCase{
		// batch concurrent
		TestCase{
			First: [2][]A{
				[]A{Ins{0, "a"}, Ins{1, "d"}},
				[]A{Ins{0, "b"}, Ins{1, "c"}},
			},
			Then: Tee(nil),
			Rest: Tee(nil),
		},
		TestCase{
			First: [2][]A{
				[]A{Ins{0, "a"}, Ins{1, "b"}},
				[]A{Ins{0, "m"}, Ins{1, "n"}},
			},
			Then: [2][]A{
				[]A{Del{0, 1}, Ins{1, "d"}},
				[]A{Ins{0, "r"}},
			},
			Rest: Tee(nil),
		},
		TestCase{
			First: [2][]A{
				[]A{Ins{0, "a"}},
				[]A{Ins{0, "m"}, Ins{1, "n"}},
			},
			Then: [2][]A{
				[]A{Del{0, 1}, Ins{1, "d"}},
				[]A{Del{1, 1}},
			},
			Rest: Tee(nil),
		},
	}
	doTable(t, table)
}

type ComposeCase struct {
	A [2]Ops
	B Ops
}

func doComposeTable(t *testing.T, cases []ComposeCase) {
	for idx, c := range cases {
		a := Compose(c.A[0], c.A[1])
		if !reflect.DeepEqual(a, c.B) {
			t.Errorf("compose %d failed; %s -> %s != expected %s", idx, c.A, a, c.B)
		}
	}
}

func TestCompose(t *testing.T) {
	table := []ComposeCase{
		ComposeCase{
			A: [2]Ops{
				Ops{Op{0, []rune{'a'}}},
				Ops{Op{1, nil}, Op{0, []rune{'b'}}},
			},
			B: Ops{Op{0, []rune{'a', 'b'}}},
		},
		ComposeCase{
			A: [2]Ops{
				Ops{Op{0, []rune{'a'}}},
				Ops{Op{-1, nil}},
			},
			B: Ops{},
		},
	}

	doComposeTable(t, table)
}
