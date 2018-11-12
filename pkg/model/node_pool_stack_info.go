package model

import (
	"bytes"
	"fmt"
	"text/tabwriter"
)

type NodePoolStackInfo struct {
	Name string
}

func (c *NodePoolStackInfo) String() string {
	buf := new(bytes.Buffer)
	w := new(tabwriter.Writer)
	w.Init(buf, 0, 8, 0, '\t', 0)

	fmt.Fprintf(w, "Cluster Name:\t%s\n", c.Name)

	w.Flush()
	return buf.String()
}
