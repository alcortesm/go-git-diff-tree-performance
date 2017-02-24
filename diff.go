package main

import "bytes"

type diff struct {
	from  string
	to    string
	lines []string
}

func newDiff(from, to string, lines []string) diff {
	return diff{
		from:  from,
		to:    to,
		lines: lines,
	}
}

func (d *diff) String() string {
	prefix := d.from + " " + d.to + " "

	var buf bytes.Buffer
	for _, l := range d.lines {
		buf.WriteString(prefix)
		buf.WriteString(l)
		buf.WriteByte('\n')
	}

	return buf.String()
}
