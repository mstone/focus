// Copyright 2016 Akamai Technologies, Inc.
// Please see the accompanying LICENSE file for licensing information.

package ot

import (
	"testing"
)

func TestZipperInsert(t *testing.T) {
	root := Branch(nil)

	z := NewZipper(&root, 0, 10)

	if z.Parent() != &root {
		t.Fatalf("parent != root")
	}

	if z.Index() != 0 {
		t.Fatalf("index != 0")
	}

	if z.Current() != &root {
		t.Fatalf("current != nil")
	}

	if z.Depth() != 1 {
		t.Fatalf("depth != 1")
	}

	if z.HasDown() {
		t.Fatalf("kids != nil")
	}

	la := Leaf('a')
	z.Insert(la)

	if z.Parent() != &root {
		t.Fatalf("parent != root")
	}

	if z.Index() != 0 {
		t.Fatalf("index != 0")
	}

	if z.Current() == nil || !z.Current().IsLeaf() || z.Current().Leaf != 'a' {
		t.Fatalf("current != Leaf('a')")
	}

	if z.Depth() != 1 {
		t.Fatalf("depth != 1")
	}

	if z.HasDown() {
		t.Fatalf("leaf has kids")
	}

	z.Delete(1)

	// back to square empty root
	if z.Parent() != &root {
		t.Fatalf("parent != root")
	}

	if z.Index() != 0 {
		t.Fatalf("index != 0")
	}

	if z.Current() != &root {
		t.Fatalf("current != nil")
	}

	if z.Depth() != 1 {
		t.Fatalf("depth != 1")
	}

	if z.HasDown() {
		t.Fatalf("kids != nil")
	}
}
