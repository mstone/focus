#!/bin/sh
M=github.com/mstone/focus/internal/server; go test -c $M; until ! (GOMAXPROCS=1 ./$(basename $M).test -test.run=Random &> focus.err); do echo test; done; grep -c crit focus.err; tac focus.err | sed -n '/crit/q;p' | tac | sed -n '/FAIL/q;p' > zzz; (cd sequence; go build); ./sequence/sequence -i zzz -o foo.tex -c 4; pdflatex foo.tex; pdftk foo.pdf cat 1 output bar.pdf; mupdf bar.pdf
