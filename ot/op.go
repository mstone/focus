// Copyright 2016 Akamai Technologies, Inc.
// Please see the accompanying LICENSE file for licensing information.

package ot

import (
	"fmt"
	"strings"

	"github.com/juju/errors"
)

type OpTag int

const (
	O_NIL OpTag = iota
	O_INSERT
	O_RETAIN
	O_DELETE
	O_WITH
)

type Op struct {
	// Tag indicates what kind of Op we have
	Tag OpTag

	// Len is either delete-len (if negative) or retain-len (if positive)
	Size int

	// Body is the tree to be inserted
	Body Tree

	// Kids are the child-ops for parent With operations
	Kids Ops
}

func (o Op) Clone() Op {
	return Op{
		Tag:  o.Tag,
		Size: o.Size,
		Body: o.Body.Clone(),
		Kids: o.Kids.Clone(),
	}
}

func (o *Op) IsRetain() bool {
	if o == nil {
		return false
	}
	return o.Tag == O_RETAIN && o.Size > 0
}

func (o *Op) IsDelete() bool {
	if o == nil {
		return false
	}
	return o.Tag == O_DELETE && o.Size < 0
}

func (o *Op) IsInsert() bool {
	if o == nil {
		return false
	}
	return o.Tag == O_INSERT && o.Size == 0 && o.Body.Len() > 0
}

func (o *Op) IsWith() bool {
	if o == nil {
		return false
	}
	return o.Tag == O_WITH
}

func (o *Op) IsZero() bool {
	if o == nil {
		return true
	}
	return o.Tag == O_NIL && o.Size == 0 && o.Body.Len() == 0 && len(o.Kids) == 0
}

func (o *Op) IsInsertLeaf() bool {
	if o == nil {
		return false
	}
	return o.Tag == O_INSERT && o.Size == 0 && o.Body.IsLeaf()
}

func (o *Op) IsInsertBranch() bool {
	if o == nil {
		return false
	}
	return o.Tag == O_INSERT && o.Size == 0 && o.Body.IsBranch()
}

// Len returns the tree-len of o; i.e., the number of tree nodes affected by o.
func (o *Op) Len() int {
	switch {
	case o.IsZero():
		return 0
	case o.IsDelete():
		return -o.Size
	case o.IsRetain():
		return o.Size
	case o.IsInsert():
		return o.Body.Len()
	case o.IsWith():
		// SUBTLE(mistone): W ops have tree-length 1, due to their interaction with D + R ops in compose1().
		return 1
	default:
		panic(fmt.Sprintf("len got bad op, %s", o.String()))
	}
}

func (o *Op) String() string {
	switch {
	case o == nil:
		return "N"
	case o.IsDelete():
		return fmt.Sprintf("D%d", -o.Size)
	case o.IsRetain():
		return fmt.Sprintf("R%d", o.Size)
	case o.IsInsert():
		return fmt.Sprintf("I%s", o.Body.String())
	case o.IsZero():
		return "Z"
	case o.IsWith():
		return fmt.Sprintf("W%s", o.Kids)
	default:
		return fmt.Sprintf("E%#v", o)
	}
}

type Ops []Op

func (os Ops) Clone() Ops {
	if len(os) == 0 {
		return nil
	}
	os2 := make(Ops, len(os))
	for i, op := range os {
		os2[i] = op.Clone()
	}
	return os2
}

func (os Ops) String() string {
	if len(os) > 0 {
		strs := []string{}
		for _, o := range os {
			strs = append(strs, o.String())
		}
		return fmt.Sprintf("[%s]", strings.Join(strs, " "))
	} else {
		return "[]"
	}
}

func (os Ops) First() *Op {
	return &os[0]
}

func (os Ops) Last() *Op {
	return &os[len(os)-1]
}

func (os Ops) Rest() Ops {
	return os[1:]
}

func (os Ops) Empty() bool {
	return len(os) == 0
}

func (o Op) SplitAt(n int) (Op, Op, error) {
	switch {
	case o.IsInsert():
		return o.splitInsert(n)
	case o.IsDelete():
		return o.splitDelete(n)
	case o.IsRetain():
		return o.splitRetain(n)
	case o.IsWith():
		return o.splitWith(n)
	case o.IsZero():
		return Z(), Z(), nil
	default:
		return Z(), Z(), errors.Errorf("Op.SplitAt failed, attempted to split unknown op, o: %s, n: %d", o.String(), n)
	}
}

func (o Op) splitInsert(n int) (Op, Op, error) {
	l, r, err := o.Body.SplitAt(n)
	return It(l), It(r), errors.Trace(err)
}

func (o Op) splitRetain(n int) (Op, Op, error) {
	sz := o.Len()
	if n > sz {
		return Z(), Z(), errors.Errorf("Op.splitRetain failed, o: %s, n: %d", o.String(), n)
	}
	return R(n), R(sz - n), nil
}

func (o Op) splitDelete(n int) (Op, Op, error) {
	sz := o.Len()
	if n > sz {
		return Z(), Z(), errors.Errorf("Op.splitDelete failed, o: %s, n: %d", o.String(), n)
	}
	return D(n), D(sz - n), nil
}

func (o Op) splitWith(n int) (Op, Op, error) {
	switch {
	case n == 0:
		return Z(), o, nil
	case n == 1:
		return o, Z(), nil
	default:
		return Z(), Z(), errors.Errorf("Op.splitWith failed, o: %s, n: %d", o.String(), n)
	}
}

func (os Ops) SplitAt(n int) (Ops, Ops, error) {
	sz := len(os)
	if n > sz {
		return nil, nil, errors.Errorf("Ops.SplitAt failed, os: %s, n: %d", os.String(), n)
	}
	return os[:n], os[n:], nil
}

// func (op *Op) extend(rhs Tree) {
// 	switch {
// 	// case op.Body.IsLeaf() && rhs.IsLeaf():
// 	// 	lhs := op.Body.Body
// 	// 	op.Body.Body = make([]rune, len(lhs)+rhs.Len())
// 	// 	copy(op.Body.Body, lhs)
// 	// 	copy(op.Body.Body[len(lhs):], rhs.Body)
// 	case op.Body.IsBranch(): // && rhs.IsBranch():
// 		lhs := op.Body.Kids
// 		op.Body.Kids = make(Trees, len(lhs)+rhs.Len())
// 		copy(op.Body.Kids, lhs)
// 		copy(op.Body.Kids[len(lhs):], rhs.Kids)
// 	default:
// 		panic(errors.Errorf("extend error: op.Body.Tag != rhs.Tag, op: %s, t: %s", op.String(), rhs.String()))
// 	}
// }

func (os *Ops) insertPenultimate(op Op) {
	rhs := *os
	rlen := len(rhs)
	lhs := make(Ops, rlen+1)
	copy(lhs, rhs[:rlen-1])
	lhs[rlen-1] = op
	lhs[rlen] = rhs[rlen-1]
	*os = lhs
}

func (os *Ops) insertUltimate(op Op) {
	rhs := *os
	lhs := make(Ops, len(rhs))
	copy(lhs, rhs)
	lhs = append(lhs, op)
	*os = lhs
}

func (os *Ops) Insert(t Tree) {
	ops := *os
	olen := len(ops)

	if t.Len() == 0 {
		return
	}

	switch {
	// case olen > 0 && os.Last().IsInsert() && os.Last().Body.Tag == t.Tag:
	// 	ops.Last().extend(t)
	case olen > 0 && os.Last().IsDelete():
		// if olen > 1 && ops[olen-2].IsInsert() && ops[olen-2].Body.Tag == t.Tag {
		// (&ops[olen-2]).extend(t)
		// } else {
		os.insertPenultimate(It(t))
		// }
	default:
		os.insertUltimate(It(t))
	}
}

func (os *Ops) Retain(size int) {
	switch {
	case size == 0:
		return
	case len(*os) > 0 && os.Last().IsRetain():
		os.Last().Size += size
	default:
		os.insertUltimate(R(size))
	}
}

func (os *Ops) Delete(size int) {
	ops := *os
	olen := len(ops)
	if size == 0 {
		return
	}
	if size > 0 {
		size = -size
	}
	if olen > 0 && ops[olen-1].IsDelete() {
		ops[olen-1].Size += size
	} else {
		os.insertUltimate(D(size))
	}
}

func (os *Ops) With(kids Ops) {
	os.insertUltimate(W(kids)) // BUG(mistone): should With() fold into previous With ops?
}
