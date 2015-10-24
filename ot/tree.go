package ot

import (
	"fmt"
	"strings"

	"github.com/juju/errors"
)

type TreeTag int

const (
	T_NIL TreeTag = iota
	T_LEAF
	T_BRANCH
)

type Tree struct {
	Tag  TreeTag
	Leaf rune
	Kids Trees
}

type Trees []Tree

func (t Tree) Clone() Tree {
	return Tree{
		Tag:  t.Tag,
		Leaf: t.Leaf,
		Kids: t.Kids.Clone(),
	}
}

func (ts Trees) Clone() Trees {
	if len(ts) == 0 {
		return nil
	}
	ret := make(Trees, len(ts))
	for k, v := range ts {
		ret[k] = v.Clone()
	}
	return ret
}

func (t Tree) String() string {
	switch {
	case t.Tag == T_LEAF:
		return AsString([]rune{t.Leaf})
	case t.Tag == T_BRANCH:
		return t.Kids.String()
	default:
		panic(errors.Errorf("String(): tree with unknown tag, t: %#v", t))
	}
}

func (ts Trees) String() string {
	ks := make([]string, len(ts))
	for k, v := range ts {
		ks[k] = v.String()
	}
	return fmt.Sprintf("[%s]", strings.Join(ks, " "))
}

func (t Tree) Len() int {
	switch {
	case t.Tag == T_NIL:
		return 0 // BUG(mistone): needed for op.IsZero(), but is it correct?
	case t.Tag == T_LEAF:
		// return len(t.Body)
		return 1
	case t.Tag == T_BRANCH:
		// SUBTLE(mistone): Tree nodes take up 1 space for transformation purposes -- they act like individual letters from a countably infinite alphabet, which can be recursed into.
		return 1
	default:
		panic(errors.Errorf("Len(): tree with unknown tag, t: %#v", t))
	}
}

func (t *Tree) IsZero() bool {
	return t.Tag == T_NIL && t.Leaf == 0 && t.Kids == nil
}

func (t *Tree) IsLeaf() bool {
	if t == nil {
		return false
	}
	return t.Tag == T_LEAF
}

func (t *Tree) IsBranch() bool {
	if t == nil {
		return false
	}
	return t.Tag == T_BRANCH
}

func Zt() Tree {
	return Tree{}
}

func Leaf(leaf rune) Tree {
	return Tree{
		Tag:  T_LEAF,
		Leaf: leaf,
	}
}

func Branch(kids Trees) Tree {
	return Tree{
		Tag:  T_BRANCH,
		Kids: kids.Clone(),
	}
}

func (ts Trees) Len() int {
	return len(ts)
}

func (ts Trees) First() *Tree {
	return &ts[0]
}

func (ts Trees) Last() *Tree {
	return &ts[len(ts)-1]
}

func (ts Trees) Rest() Trees {
	return ts[1:]
}

func (ts Trees) Empty() bool {
	return len(ts) == 0
}

func (t Tree) SplitAt(n int) (Tree, Tree, error) {
	switch {
	// case t.IsLeaf():
	// 	return t.splitAtLeaf(n)
	case t.IsBranch():
		return t.splitAtBranch(n)
	default:
		return Zt(), Zt(), errors.Errorf("Tree.SplitAt bad tag, t: %s, n: %d", t.String(), n)
	}
}

// func (t Tree) splitAtLeaf(n int) (Tree, Tree, error) {
// 	if !t.IsLeaf() || n > len(t.Body) {
// 		return Tree{}, Tree{}, errors.Errorf("Tree.splitAtLeaf failed, t: %s, n: %d", t.String(), n)
// 	}
// 	l, r := CloneRunes(t.Body[:n]), CloneRunes(t.Body[n:])
// 	return Leaf(l), Leaf(r), nil

// }

func (t Tree) splitAtBranch(n int) (Tree, Tree, error) {
	if !t.IsBranch() || n > len(t.Kids) {
		return Tree{}, Tree{}, errors.Errorf("Tree.splitAtBranch failed, t: %s, n: %d", t.String(), n)
	}
	l, r := t.Kids[:n].Clone(), t.Kids[n:].Clone()
	return Branch(l), Branch(r), nil
}

func (ts Trees) SplitAt(n int) (Trees, Trees, error) {
	if n > len(ts) {
		return nil, nil, errors.Errorf("Trees.SplitAt failed, t: %s, n: %d", ts.String(), n)
	}
	l, r := ts[:n].Clone(), ts[n:].Clone()
	return l, r, nil
}

func (ts *Trees) insertUltimate(t Tree) {
	ret := make(Trees, len(*ts)+1)
	copy(ret, *ts)
	ret[len(*ts)] = t
	*ts = ret
}

type Zipper struct {
	ts []*Tree
	ns []int
}

func NewZipper(root *Tree, index int, depth int) *Zipper {
	ts, ns := make([]*Tree, 1, depth), make([]int, 1, depth)
	ts[0] = root
	ns[0] = index
	return &Zipper{
		ts: ts,
		ns: ns,
	}
}

func (z *Zipper) Down() {
	ts, ns := z.ts, z.ns
	z.ts = append(ts, &z.Parent().Kids[0])
	z.ns = append(ns, 0)
}

func (z *Zipper) Up() {
	z.ts = z.ts[:len(z.ts)-1]
	z.ns = z.ns[:len(z.ns)-1]
}

func (z *Zipper) Skip(n int) {
	z.ns[len(z.ns)-1] = z.Index() + n
}

func (z *Zipper) Current() *Tree {
	p, n := z.Parent(), z.Index()
	if 0 <= n && n < len(p.Kids) {
		return &z.Parent().Kids[z.Index()]
	}
	return nil
}

func (z *Zipper) Parent() *Tree {
	return z.ts[len(z.ts)-1]
}

func (z *Zipper) Index() int {
	return z.ns[len(z.ns)-1]
}

func (z *Zipper) IndexP() *int {
	return &z.ns[len(z.ns)-1]
}

func (z *Zipper) Depth() int {
	return len(z.ns)
}

func (z *Zipper) CanSkip(n int) bool {
	p, n := z.Parent(), z.Index()+n
	return 0 <= n && n < len(p.Kids)
}

func (z *Zipper) HasDown() bool {
	c := z.Current()
	if c != nil && len(c.Kids) > 0 {
		return true
	}
	return false
}

func (z *Zipper) Insert(t Tree) {
	p, n := z.Parent(), z.Index()
	ks := p.Kids
	p.Kids = make([]Tree, len(ks)+1)
	copy(p.Kids[:n], ks[:n])
	p.Kids[n] = t
	copy(p.Kids[n+1:len(p.Kids)], ks[n:])
}

func (z *Zipper) Retain(n int) {
	z.Skip(n)
}

func (z *Zipper) Delete(n int) {
	p, i := z.Parent(), z.Index()
	ks := p.Kids
	p.Kids = make([]Tree, len(ks)-n)
	copy(p.Kids[:i], ks[:i])
	copy(p.Kids[i:len(p.Kids)], ks[i+n:])
}
