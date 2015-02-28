// Copyright 2015 Akamai Technologies, Inc.
// Please see the accompanying LICENSE file for licensing information.

package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"regexp"

	log "gopkg.in/inconshreveable/log15.v2"

	"github.com/kr/logfmt"
)

var in = flag.String("i", "focus.err", "input file")
var out = flag.String("o", "-", "output file")
var numClients = flag.Int("c", 4, "number of clients")
var numDocs = flag.Int("d", 1, "number of documents")

type Record struct {
	Msg     string
	Obj     string
	Action  string
	Cmd     string
	Conn    int
	Doc     string
	Client  int
	Name    string
	Fd      int
	Rev     int
	Ops     string
	Comp    string
	Hist    string
	Body    string
	From    string
	To      string
	Pdoc    string
	Ndoc    string
	Prev    string
	Nrev    string
	Pstate  string
	Nstate  string
	Kind    string
	Clnst   string
	Clnhist string
	Docrev  int
	Dochist string
	Tops    string
}

var ampersand = regexp.MustCompile("&")

func escape(s string) string {
	s = ampersand.ReplaceAllLiteralString(s, `\&`)
	return s
}

func main() {
	flag.Parse()

	log.Root().SetHandler(
		log.StderrHandler,
	)

	var err error

	outf := os.Stdout
	if out != nil && len(*out) > 0 && *out != "-" {
		outf, err = os.OpenFile(*out, os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Crit("unable to open out", "out", *out, "err", err)
			return
		}
	}
	defer outf.Close()

	inf := os.Stdin
	if in != nil && len(*in) > 0 && *in != "-" {
		inf, err = os.Open(*in)
		if err != nil {
			log.Crit("unable to open in", "in", *in, "err", err)
			return
		}
	}
	defer inf.Close()

	outf.WriteString(`
% Adapted from http://tex.stackexchange.com/questions/174207/adding-content-on-sequence-diagram-tikz-uml-pgf-umlsd
% + http://christopherpoole.github.io/fitting-page-size-to-tikz-figure-without-the-standalone-package/
\documentclass{article}
\usepackage{float}
\usepackage{tikz}
\usepackage[active,tightpage]{preview}
\usetikzlibrary{positioning, fit, calc, shapes, arrows, shadows}
\usepackage[underline=false]{pgf-umlsd}

% \bloodymess[delay]{sender}{message content}{receiver}{DIR}{start note}{end note}
% \newcommand{\bloodymess}[7][0]{
%   \stepcounter{seqlevel}
%   \path
%   (#2)+(0,-\theseqlevel*\unitfactor-0.7*\unitfactor) node (mess from) {};
%   \addtocounter{seqlevel}{#1}
%   \path
%   (#4)+(0,-\theseqlevel*\unitfactor-0.7*\unitfactor) node (mess to) {};
%   \draw[->,>=angle 60] (mess from) -- (mess to) node[midway, above, font=\footnotesize]
%   {#3};
%
%   \if R#5
%     \node (#3 from) at (mess from) {\llap{#6~}};
%     \node (#3 to) at (mess to) {\rlap{~#7}};
%   \else\if L#5
%          \node (#3 from) at (mess from) {\rlap{~#6}};
%          \node (#3 to) at (mess to) {\llap{#7~}};
%        \else
%          \node (#3 from) at (mess from) {#6};
%          \node (#3 to) at (mess to) {#7};
%        \fi
%   \fi
% }




%message between threads
%Example:
%\bloodymess[delay]{sender}{message content}{receiver}{DIR}{start note}{end note}{message attributes}
\newcommand{\bloodymess}[8][0]{
  \stepcounter{seqlevel}
  \path
  (#2)+(0,-\theseqlevel*\unitfactor-0.7*\unitfactor) node (mess from) {};
  \addtocounter{seqlevel}{#1}
  \path
  (#4)+(0,-\theseqlevel*\unitfactor-0.7*\unitfactor) node (mess to) {};
  \draw[->,>=angle 60] (mess from) -- (mess to) node#8
  {#3};

  \if R#5
    \node (#3 from) at (mess from) {\llap{#6~}};
    \node (#3 to) at (mess to) {\rlap{~#7}};
  \else\if L#5
         \node (#3 from) at (mess from) {\rlap{~#6}};
         \node (#3 to) at (mess to) {\llap{#7~}};
       \else
         \node (#3 from) at (mess from) {#6};
         \node (#3 to) at (mess to) {#7};
       \fi
  \fi
}

% \newthread[color]{width}{var}{name}{class}
\newcommand{\newthreadx}[4][gray!30]{
  \newinst[#2]{#3}{#4}
  \stepcounter{threadnum}
  \node[below of=inst\theinstnum,node distance=0.8cm] (thread\thethreadnum) {};
  \tikzstyle{threadcolor\thethreadnum}=[fill=#1]
  \tikzstyle{instcolor#2}=[fill=#1]
}


\PreviewEnvironment{tikzpicture}
\setlength\PreviewBorder{5pt}

\begin{document}

\begin{figure}[H]
    \centering
    \begin{sequencediagram}` + "\n")

	for i := 0; i < *numDocs; i++ {
		outf.WriteString(fmt.Sprintf(`            \newthreadx{1}{d%d}{Doc %d}`+"\n", i, i))
	}
	for i := 0; i < *numClients; i++ {
		outf.WriteString(fmt.Sprintf(`            \newthreadx{3}{c%d}{Client %d}`+"\n", i, i))
	}

	scanner := bufio.NewScanner(inf)
	var rec Record

	for scanner.Scan() {
		line := scanner.Text()
		rec = Record{}
		err = logfmt.Unmarshal([]byte(line), &rec)
		if err != nil {
			log.Error("unmarshal err", "line", line, "err", err)
		}

		if rec.Obj == "cln" {
			switch rec.Msg {
			case "genn":
				at := fmt.Sprintf("c%d", rec.Client)
				before := fmt.Sprintf("gen %s : %s : %s", rec.Ops, rec.Clnst, rec.Clnhist)
				after := fmt.Sprintf("")
				fmt.Fprintf(outf, `\begin{callself}{%s}{\footnotesize %s}{\footnotesize %s}
				\end{callself}`+"\n", escape(at), escape(before), escape(after))
			case "stat":
				at := fmt.Sprintf("c%d", rec.Client)
				before := fmt.Sprintf("stat %s : %s", rec.Body, rec.Clnst)
				after := fmt.Sprintf("")
				fmt.Fprintf(outf, `\begin{callself}{%s}{\footnotesize %s}{\footnotesize %s}
				\end{callself}`+"\n", escape(at), escape(before), escape(after))
			case "recv":
				from := "d0"
				var label string
				switch rec.Kind {
				case "wrt":
					label = fmt.Sprintf("wrt %d %s : %s : %s", rec.Rev, rec.Ops, rec.Clnst, rec.Clnhist)
				case "ack":
					label = fmt.Sprintf("ack %d", rec.Rev)
				}
				to := fmt.Sprintf("c%d", rec.Client)
				dir := "R"
				start := ""
				end := ""
				labelPos := ""
				if rec.Conn%2 == 1 {
					labelPos = "[midway, above right = -2mm and 10mm, font=\\footnotesize, blue]"
				} else {
					labelPos = "[midway, above, font=\\footnotesize, blue]"
				}
				fmt.Fprintf(outf, "            \\bloodymess[1]{%s}{%s}{%s}{%s}{%s}{%s}{%s}\n",
					escape(from), escape(label), escape(to), escape(dir), escape(start), escape(end), escape(labelPos))
			}
		}
		if rec.Obj == "doc" {
			switch rec.Msg {
			case "recv":
				at := "d0"
				before := fmt.Sprintf("recv %d %s $\\rightarrow$ %s", rec.Rev, rec.Ops, rec.Tops)
				after := fmt.Sprintf("%d : %s", rec.Docrev, rec.Dochist)
				fmt.Fprintf(outf, `\begin{callself}{%s}{\footnotesize %s}{\footnotesize %s}
				\end{callself}`+"\n", escape(at), escape(before), escape(after))
			case "stat":
				at := fmt.Sprintf("d0")
				before := fmt.Sprintf("stat %s", rec.Body)
				after := fmt.Sprintf("")
				fmt.Fprintf(outf, `\begin{callself}{%s}{\footnotesize %s}{\footnotesize %s}
				\end{callself}`+"\n", escape(at), escape(before), escape(after))
			}
		}
	}

	outf.WriteString(`
    \end{sequencediagram}
    \caption{Client-Server messaging}
\end{figure}
\end{document}`)

}

// from := fmt.Sprintf("c%d", rec.Conn)
// from := fmt.Sprintf("c%d", rec.Conn)
// label := fmt.Sprintf("wrt %d %s", rec.Rev, rec.Ops)
// to := "d0"
// dir := "L"
// start := ""
// end := ""
// // labelPos := "[midway, above, font=\\footnotesize]" // right = 2mm and 2mm"
// labelPos := ""
// if rec.Conn%2 == 1 {
// 	labelPos = "[midway, above left = -2mm and 10mm, font=\\footnotesize, red]"
// } else {
// 	labelPos = "[midway, above, font=\\footnotesize, red]"
// }
// fmt.Fprintf(outf, "            \\bloodymess[1]{%s}{%s}{%s}{%s}{%s}{%s}{%s}\n",
// 	from, label, to, dir, start, end, labelPos)
