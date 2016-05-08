// Copyright 2016 Akamai Technologies, Inc.
// Please see the accompanying LICENSE file for licensing information.

package ace

import ()

type Range struct {
	doc Document
	se  StartEnd
}

func NewRange(doc Document, se StartEnd) *Range {
	return &Range{
		doc: doc,
		se:  se,
	}
}

func (r *Range) asLinearIndex(pos Position) int {
	// add the lengths of the lines before startRow, plus
	// startCol, plus 1 * startRow for the line delimiters
	row, col := pos.Row(), pos.Col()

	linesBefore := r.doc.GetLines(0, row-1)
	// alert.JSON(linesBefore)

	idx := col + row
	for i := 0; i < linesBefore.Length(); i++ {
		idx += linesBefore.Index(i).Length()
	}
	return idx
}

func (r *Range) Start() int {
	start := r.se.Start()
	return r.asLinearIndex(start)
}

func (r *Range) End() int {
	end := r.se.End()
	return r.asLinearIndex(end)
}
