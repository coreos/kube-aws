package cluster

import (
	"bytes"
	"fmt"
	"text/tabwriter"
)

type Info struct {
	Name           string
	ControllerHost string
}

func (c *Info) String() string {
	buf := new(bytes.Buffer)
	w := new(tabwriter.Writer)
	w.Init(buf, 0, 8, 0, '\t', 0)

	fmt.Fprintf(w, "Cluster Name:\t%s\n", c.Name)
	fmt.Fprintf(w, "Controller DNS Name:\t%s\n", c.ControllerHost)

	w.Flush()
	return buf.String()
}
