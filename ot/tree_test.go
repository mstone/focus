package ot

import (
	"testing"
)

func TestZipperInsert(t *testing.T) {
	root := Branch(nil)

	z := NewZipper(&root, 10)

	if z.Current() != &root {
		t.Fatalf("current != root")
	}
}
