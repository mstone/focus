#!/bin/bash -eu
(cd sequence; go build)
./sequence/sequence -o=focus.err.tex
pdflatex focus.err.tex
xdg-open focus.err.pdf || open focus.err.pdf
