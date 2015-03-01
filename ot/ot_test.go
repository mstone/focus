// Copyright 2014 Akamai Technologies, Inc.
// Please see the accompanying LICENSE file for licensing information.

package ot

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestJSON(t *testing.T) {
	c1 := Ops{Op{Size: -1}, Op{Size: 1}, Op{Body: AsRunes("hi")}}

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
		t.Logf("compose %d, composing A[0]: %s, A[1]: %s, expecting B: %s", idx, c.A[0], c.A[1], c.B)
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
				Ops{I("a")},
				Ops{R(1), I("b")},
			},
			B: Ops{I("ab")},
		},
		ComposeCase{
			A: [2]Ops{
				Ops{I("a")},
				Ops{D(1)},
			},
			B: Ops{},
		},
		ComposeCase{
			A: [2]Ops{
				Ops{I("ex")},
				Ops{R(2), I("4")},
			},
			B: Ops{I("ex4")},
		},
		ComposeCase{
			A: [2]Ops{
				Ops{I("x")},
				Ops{R(1), I("4")},
			},
			B: Ops{I("x4")},
		},
		ComposeCase{
			A: [2]Ops{
				Ops{I("ex")},
				Ops{R(1), I("4"), R(1)},
			},
			B: Ops{I("e4x")},
		},
	}

	doComposeTable(t, table)
}

type InsertCase struct {
	A Ops
	B []rune
	C Ops
}

func doInsertTable(t *testing.T, cases []InsertCase) {
	for idx, c := range cases {
		t.Logf("insert %d, inserting A: %s, B: %s, expecting C: %s", idx, c.A, AsString(c.B), c.C)
		a := c.A.Clone()
		a.Insert(c.B)
		if !reflect.DeepEqual(a, c.C) {
			t.Errorf("insert %d failed;\n\tA: %s\n\ta: %s\n\tB:\n\tC: %s", idx, c.A, a, c.B, c.C)
		}
	}
}

func TestInsert(t *testing.T) {
	table := []InsertCase{
		InsertCase{
			A: Ops{},
			B: AsRunes("a"),
			C: Ops{I("a")},
		},
		InsertCase{
			A: Ops{I("a")},
			B: AsRunes("b"),
			C: Ops{I("ab")},
		},
		InsertCase{
			A: Ops{I("a"), D(2)},
			B: AsRunes("b"),
			C: Ops{I("ab"), D(2)},
		},
		InsertCase{
			A: Ops{D(2)},
			B: AsRunes("a"),
			C: Ops{I("a"), D(2)},
		},
	}

	doInsertTable(t, table)
}

type TransformCase struct {
	A, B, C, D Ops
}

func doTransformTable(t *testing.T, cases []TransformCase) {
	for idx, c := range cases {
		t.Logf("transform %d, transforming A: %s, B: %s, expecting C: %s, D: %s", idx, c.A, c.B, c.C, c.D)
		x, y := Transform(c.A, c.B)
		if !reflect.DeepEqual(x, c.C) {
			t.Errorf("transform %d failed;\n\tA: %s\n\tB: %s\n\tC: %s\n\tD: %s\n\tx: %s\n\ty: %s", idx, c.A, c.B, c.C, c.D, x, y)
		}
		if !reflect.DeepEqual(y, c.D) {
			t.Errorf("transform %d failed;\n\tA: %s\n\tB: %s\n\tC: %s\n\tD: %s\n\tx: %s\n\ty: %s", idx, c.A, c.B, c.C, c.D, x, y)
		}
	}
}

func TestTransform(t *testing.T) {
	table := []TransformCase{
		// xy     x5y
		// xby    x5by
		TransformCase{
			A: Ops{Op{1, nil}, Op{0, AsRunes("5")}, Op{1, nil}},
			B: Ops{Op{1, nil}, Op{0, AsRunes("b")}, Op{1, nil}},
			C: Ops{Op{1, nil}, Op{0, AsRunes("5")}, Op{2, nil}},
			D: Ops{Op{2, nil}, Op{0, AsRunes("b")}, Op{1, nil}},
		},
	}
	doTransformTable(t, table)
}

func testOneCompose(t *testing.T) {
	composedOps := Ops{}

	d1 := NewDoc()

	for i := 0; i < 100; i++ {
		ops := d1.GetRandomOps(4)
		d1.Apply(ops)
		composedOps = Compose(composedOps, ops)
	}

	d2 := NewDoc()
	d2.Apply(composedOps)

	s1 := d1.String()
	s2 := d2.String()
	if s1 != s2 {
		t.Fatalf("Compose fail: s1 != s2: \n\t%q\n\t%q", s1, s2)
	}
}

func TestRandomCompose(t *testing.T) {
	for i := 0; i < 100; i++ {
		testOneCompose(t)
	}
}

func testOneTransform(t *testing.T) {
	d1 := NewDoc()
	d2 := NewDoc()

	a1 := Ops{}
	a2 := Ops{}

	for i := 0; i < 100; i++ {
		o1 := d1.GetRandomOps(4)
		d1.Apply(o1)

		o2 := d2.GetRandomOps(4)
		d2.Apply(o2)

		a1 = Compose(a1, o1)
		a2 = Compose(a2, o2)
	}

	b1, b2 := Transform(a1, a2)

	d1.Apply(b2)
	d2.Apply(b1)

	s1 := d1.String()
	s2 := d2.String()
	if s1 != s2 {
		t.Fatalf("Transform fail: s1 != s2: \n\t%q\n\t%q", s1, s2)
	}
}

func TestRandomTransform(t *testing.T) {
	for i := 0; i < 100; i++ {
		testOneTransform(t)
	}
}
