// Copyright 2014 Akamai Technologies, Inc.
// Please see the accompanying LICENSE file for licensing information.

package ot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/juju/errors"
)

func TestJSON(t *testing.T) {
	c1 := C(Ds(1), Rs(1), Is("hi"))
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
	String() string
}

type Ins struct {
	Pos int
	Str string
}

func (i Ins) String() string {
	return fmt.Sprintf("In(%d, %s)", i.Pos, i.Str)
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

func (d Del) String() string {
	return fmt.Sprintf("De(%d, %d)", d.Pos, d.Len)
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
	var err error
	for _, o := range trace[0] {
		b1, err = Compose(b1, o.Apply(r1))
		if err != nil {
			t.Fatalf("doEpoch() err: %q", err)
		}
	}
	for _, o := range trace[1] {
		t.Logf("b2: %s, op: %s", b2.String(), o.String())
		b2, err = Compose(b2, o.Apply(r2))
		if err != nil {
			t.Fatalf("doEpoch() err 2: %q", err)
		}
	}
	t.Logf("Compose produced b2: %s", b2.String())
	c1, c2, err := Transform(b1, b2)
	if err != nil {
		t.Fatalf("doEpoch() err 3: %s\n\tc1: %s\n\tc2: %s", err, c1.String(), c2.String())
	}
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
		{
			First: Tee([]A{Ins{0, "a"}, Ins{0, "b"}}),
			Then:  [2][]A{{Ins{0, "c"}}, {Ins{0, "d"}}},
			Rest:  Tee([]A{Ins{1, "e"}}),
		},
		{
			First: Tee([]A{Ins{0, "a"}, Del{0, 1}}),
			Then:  [2][]A{{Ins{0, "c"}}, {Ins{0, "d"}}},
			Rest:  Tee([]A{Ins{1, "e"}}),
		},
		{
			First: Tee([]A{Ins{0, "a"}, Ins{0, "b"}}),
			Then:  [2][]A{{Ins{0, "c"}}, {Del{0, 1}}},
			Rest:  Tee([]A{Ins{1, "e"}}),
		},
		{
			First: Tee([]A{Ins{0, "a"}, Ins{0, "b"}}),
			Then:  [2][]A{{Del{0, 1}}, {Del{0, 1}}},
			Rest:  Tee([]A{Ins{1, "e"}}),
		},
		{
			First: Tee([]A{Ins{0, "a"}, Ins{0, "b"}}),
			Then:  [2][]A{{Del{0, 1}}, {Del{1, 1}}},
			Rest:  Tee([]A{Ins{1, "e"}}),
		},
		{
			First: Tee([]A{Ins{0, "a"}, Ins{0, "b"}}),
			Then:  [2][]A{{Del{0, 1}}, {Ins{1, "c"}}},
			Rest:  Tee([]A{Ins{1, "e"}}),
		},
		{
			First: Tee([]A{Ins{0, "a"}, Ins{0, "b"}, Del{0, 1}}),
			Then:  [2][]A{{Del{0, 1}}, {Ins{1, "c"}}},
			Rest:  Tee([]A{Ins{1, "e"}}),
		},
		{
			First: Tee([]A{Ins{0, "a"}, Ins{1, "b"}, Del{0, 1}, Ins{1, "c"}}),
			Then:  [2][]A{{Del{1, 1}}, {Ins{0, "c"}}},
			Rest:  Tee([]A{Ins{2, "e"}}),
		},
	}

	doTable(t, table)
}

func TestConcurrent(t *testing.T) {
	table := []TestCase{
		// batch concurrent
		{
			First: [2][]A{
				{Ins{0, "a"}, Ins{1, "d"}},
				{Ins{0, "b"}, Ins{1, "c"}},
			},
			Then: Tee(nil),
			Rest: Tee(nil),
		},
		{
			First: [2][]A{
				{Ins{0, "a"}, Ins{1, "b"}},
				{Ins{0, "m"}, Ins{1, "n"}},
			},
			Then: [2][]A{
				{Del{0, 1}, Ins{1, "d"}},
				{Ins{0, "r"}},
			},
			Rest: Tee(nil),
		},
		{
			First: [2][]A{
				{Ins{0, "a"}},
				{Ins{0, "m"}, Ins{1, "n"}},
			},
			Then: [2][]A{
				{Del{0, 1}, Ins{1, "d"}},
				{Del{1, 1}},
			},
			Rest: Tee(nil),
		},
	}
	doTable(t, table)
}

type ShortenOpCase struct {
	A Op
	N int
	C Op
	E error
}

func doShortenOpTable(t *testing.T, cases []ShortenOpCase) {
	for idx, x := range cases {
		t.Logf("shortenOp %d, shortening A: %s, N: %d, -> C: %s, E: %s", idx, x.A.String(), x.N, x.C.String(), x.E)
		c, e := shortenOp(x.A, x.N)
		t.Logf("shortenOp %d, got %+v, %+v", idx, c, e)
		if !reflect.DeepEqual(c, x.C) {
			t.Fatalf("shortenOp %d failed; [%s %s] -> [%s %s], %s != expected C [%s]", idx, x.A, x.N, x.C, x.E, c)
		}
		if !reflect.DeepEqual(e, x.E) {
			t.Fatalf("shortenOp %d failed; [%s %s] -> [%s %s], %s != expected E [%s]", idx, x.A, x.N, x.C, x.E, e)
		}
	}
}

func TestShortenOp(t *testing.T) {
	table := []ShortenOpCase{
		{
			A: R(1),
			N: 1,
			C: Z(),
			E: nil,
		},
	}

	doShortenOpTable(t, table)
}

type ShortenOpsCase struct {
	A Op
	B Op
	C Op
	D Op
	E error
}

func doShortenOpsTable(t *testing.T, cases []ShortenOpsCase) {
	for idx, x := range cases {
		t.Logf("shorten %d, shortening A: %s, B: %s, -> C: %s, D: %s, E: %s", idx, x.A.String(), x.B.String(), x.C.String(), x.D.String(), x.E)
		c, d, e := shortenOps(x.A, x.B)
		if !reflect.DeepEqual(c, x.C) {
			t.Fatalf("shorten %d failed; [%s %s] -> [%s %s], %s != expected C [%s]", idx, x.A, x.B, x.C, x.D, x.E, c)
		}
		if !reflect.DeepEqual(d, x.D) {
			t.Fatalf("shorten %d failed; [%s %s] -> [%s %s], %s != expected D [%s]", idx, x.A, x.B, x.C, x.D, x.E, d)
		}
		if !reflect.DeepEqual(e, x.E) {
			t.Fatalf("shorten %d failed; [%s %s] -> [%s %s], %s != expected E [%s]", idx, x.A, x.B, x.C, x.D, x.E, e)
		}
	}
}

func TestShortenOps(t *testing.T) {
	table := []ShortenOpsCase{
		{
			A: R(1),
			B: It(Leaf('a')),
			C: Z(),
			D: Z(),
			E: nil,
		},
	}

	doShortenOpsTable(t, table)
}

type ComposeCase struct {
	A [2]Ops
	B Ops
}

func doComposeTable(t *testing.T, cases []ComposeCase) {
	for idx, c := range cases {
		t.Logf("compose %d, composing A[0]: %s, A[1]: %s, expecting B: %s", idx, c.A[0], c.A[1], c.B)
		a, err := Compose(c.A[0], c.A[1])
		if err != nil {
			t.Fatalf("compose %d failed; err: %s", idx, errors.ErrorStack(err))
		}
		if !reflect.DeepEqual(a, c.B) {
			t.Fatalf("compose %d failed; %s -> %s != expected %s", idx, c.A, a, c.B)
		}
	}
}

func TestCompose(t *testing.T) {
	table := []ComposeCase{
		{
			A: [2]Ops{
				Is("a"),
				C(Rs(1), Is("b")),
			},
			B: Is("ab"),
		},
		{
			A: [2]Ops{
				Is("a"),
				Ds(1),
			},
			B: Ops{},
		},
		{
			A: [2]Ops{
				Is("ex"),
				C(Rs(2), Is("4")),
			},
			B: Is("ex4"),
		},
		{
			A: [2]Ops{
				Is("x"),
				C(Rs(1), Is("4")),
			},
			B: Is("x4"),
		},
		{
			A: [2]Ops{
				Is("ex"),
				C(Rs(1), Is("4"), Rs(1)),
			},
			B: Is("e4x"),
		},
		{
			A: [2]Ops{
				C(Zs(), Is("c"), Zs()),
				C(Zs(), Is("e"), Rs(1)),
			},
			B: Is("ec"),
		},
	}

	doComposeTable(t, table)
}

type InsertCase struct {
	A Ops
	B Tree
	C Ops
}

func doInsertTable(t *testing.T, cases []InsertCase) {
	for idx, c := range cases {
		t.Logf("insert %d, inserting A: %s, B: %s, expecting C: %s", idx, c.A, c.B.String(), c.C)
		a := c.A.Clone()
		a.Insert(c.B)
		if !reflect.DeepEqual(a, c.C) {
			t.Fatalf("insert %d failed;\n\tA: %s\n\tB: %s\n\ta: %s\n\tC: %s\n\ta: %#v\n\tC: %#v", idx, c.A, c.B.String(), a, c.C, a, c.C)
		}
	}
}

func TestInsert(t *testing.T) {
	table := []InsertCase{
		{
			A: Ops{},
			B: Leaf('a'),
			C: Is("a"),
		},
		{
			A: Is("a"),
			B: Leaf('b'),
			C: Is("ab"),
		},
		{
			A: append(Is("a"), D(2)),
			B: Leaf('b'),
			C: append(Is("ab"), D(2)),
		},
		{
			A: Ops{D(2)},
			B: Leaf('a'),
			C: append(Is("a"), D(2)),
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
		x, y, err := Transform(c.B, c.A)
		if err != nil {
			t.Errorf("transform %d failed, err: %q", err)
		}
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
		// xy  A  x5y           I I
		// B       C
		// xby D  xb5y
		{
			A: C(Rs(1), Is("5"), Rs(1)),
			B: C(Rs(1), Is("b"), Rs(1)),
			C: C(Rs(1), Is("b"), Rs(2)),
			D: C(Rs(2), Is("5"), Rs(1)),
		},
		//  [?]    A     []     D I
		//  B             C
		//  [1b?]  D   [1b]
		{
			A: C(Zs(), Ds(1), Zs()),
			B: C(Zs(), Is("1b"), Rs(1)),
			C: C(Is("1b")),
			D: C(Rs(2), Ds(1)),
		},
		//  [?]  A   [1b]       ID D
		//  B           C
		//  []   D   [1b]
		{
			A: C(Zs(), Is("1b"), Ds(1), Zs()),
			B: C(Zs(), Ds(1), Zs()),
			C: C(Rs(2)),
			D: C(Is("1b")),
		},
		//  [?]  A   [1b?]       I D
		//  B           C
		//  []   D   [1b]
		{
			A: C(Zs(), Is("1b"), Rs(1), Zs()),
			B: C(Zs(), Ds(1), Zs()),
			C: C(Rs(2), Ds(1)),
			D: C(Is("1b")),
		},
		//  [?]  A   [?]       R D
		//  B         C
		//  []   D   []
		{
			A: C(Zs(), Rs(1), Zs()),
			B: C(Zs(), Ds(1), Zs()),
			C: C(Ds(1)),
			D: C(),
		},
		//  [?]  A   []        D R
		//  B         C
		//  [?]   D  []
		{
			A: C(Zs(), Ds(1), Zs()),
			B: C(Zs(), Rs(1), Zs()),
			C: C(),
			D: C(Ds(1)),
		},
		//  [xyxy]  A   [xy]        D/D
		//  B             C
		//  [xy]   D      []
		{
			A: C(Rs(1), Ds(2), Rs(1)),
			B: C(Ds(1), Rs(2), Ds(1)),
			C: C(Ds(2)),
			D: C(Ds(2)),
		},
		//  [xyxy]  A   [x]        D/D
		//  B             C
		//  [xaby]  D   [ab]
		{
			A: C(Ds(2), Rs(1), Ds(1)),
			B: C(Rs(1), Is("ab"), Ds(2), Rs(1)),
			C: C(Is("ab"), Ds(1)),
			D: C(Ds(1), Rs(2), Ds(1)),
		},
		//     A:Ia R2          I/I
		//B:Z Z R2    C
		//        D
		{
			A: C(Is("a"), Rs(2)),
			B: C(Zs(), Zs(), Rs(2)),
			C: C(Rs(3)),
			D: C(Is("a"), Rs(2)),
		},
		//     A:Ia R2          I/I
		//B:R1 I6 R1    C
		//        D
		{
			A: C(Is("a"), Rs(2)),
			B: C(Rs(1), Is("6"), Rs(1)),
			C: C(Rs(2), Is("6"), Rs(1)),
			D: C(Is("a"), Rs(3)),
		},
		//     A:I0 R3          I/I
		//B:R2 I6 R1    C
		//        D
		{
			A: C(Is("0"), Rs(3)),
			B: C(Rs(2), Is("6"), Rs(1)),
			C: C(Rs(3), Is("6"), Rs(1)),
			D: C(Is("0"), Rs(4)),
		},
	}
	doTransformTable(t, table)
}

func testOneCompose(t *testing.T) {
	composedOps := Ops{}
	var err error

	d1 := NewDoc()

	nOpsPerRound := 4
	nRounds := 8
	allOps := make([]Ops, nRounds)
	allComposedOps := make([]Ops, nRounds)
	allDocStrings := make([]string, nRounds)

	for i := 0; i < nRounds; i++ {
		ops := d1.GetRandomOps(nOpsPerRound)
		allOps[i] = ops
		d1.Apply(ops)
		allDocStrings[i] = d1.String()
		composedOps, err = Compose(composedOps, ops)
		allComposedOps[i] = composedOps
		if err != nil {
			t.Errorf("testOneCompose compose failed, err: %q", err)
		}
	}

	d2 := NewDoc()
	d2.Apply(composedOps)

	s1 := d1.String()
	s2 := d2.String()
	if s1 != s2 {
		for i, o := range allOps {
			t.Logf("round %d: %s, c: %s, d: %s", i, o.String(), allComposedOps[i].String(), allDocStrings[i])
		}
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

	for i := 0; i < 100; i++ {
		o1 := d1.GetRandomOps(4)
		d1.Apply(o1)

		o2 := d2.GetRandomOps(4)
		d2.Apply(o2)

		t1, t2, err := Transform(o1, o2)
		if err != nil {
			t.Errorf("Transform fail, err: %q", err)
		}
		d1.Apply(t2)
		d2.Apply(t1)
	}

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

type ApplyCase struct {
	A []Ops
	B string
}

func opsToString(label string, os []Ops) string {
	buf := &bytes.Buffer{}
	fmt.Fprintf(buf, "ops list: %s\n", label)
	for i, o := range os {
		fmt.Fprintf(buf, "\ti: %d, o: %s\n", i, o.String())
	}
	return buf.String()
}

func doDocApplyTable(t *testing.T, cases []ApplyCase) {
	for idx, x := range cases {
		t.Logf("apply %d, applying", idx)
		d := NewDoc()
		for _, o := range x.A {
			d.Apply(o)
		}
		if d.String() != x.B {
			t.Fatalf("apply %d failed; apply(%s) -> %s != expected %s", idx, opsToString("A: ", x.A), d.String(), x.B)
		}
	}
}

func TestDocApply(t *testing.T) {
	cases := []ApplyCase{
		{
			A: []Ops{
				C(Zs(), Is("a"), Zs()),
				C(Rs(1), Is("b"), Zs()),
			},
			B: "[a b]",
		},
		{
			A: []Ops{
				C(Zs(), Is("a"), Zs()),
				C(Zs(), Is("b"), Rs(1)),
			},
			B: "[b a]",
		},
		{
			A: []Ops{
				C(Is("ab")),
			},
			B: "[a b]",
		},
	}

	doDocApplyTable(t, cases)
}
