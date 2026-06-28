package flow

import (
	"fmt"
	"sort"

	"senda/internal/model"
)

// bodyTypes are the node types a loop body or parallel branch may contain;
// runLinear (flow.go) only handles these, so anything else is a static error
// rather than a per-run "got X" surprise.
var bodyTypes = map[string]bool{"request": true, "setvar": true, "delay": true}

// Validate statically checks a flow graph and returns one message per problem
// (empty = valid). Structural only — no disk or network — so callers can run it
// as a preflight before any node fires a request or mutates a variable.
func Validate(fl model.Flow) []string {
	var msgs []string

	if fl.Start == "" {
		msgs = append(msgs, "flow has no start node")
	} else if _, ok := fl.Nodes[fl.Start]; !ok {
		msgs = append(msgs, fmt.Sprintf("start node %q not found", fl.Start))
	}

	// owned[id] = the container that lists id in its body/branches. An owned node
	// runs linearly under its container and must not also be a main-graph target.
	owned := map[string]string{}
	for id, n := range fl.Nodes {
		switch n.Type {
		case "loop":
			for _, b := range n.Body {
				owned[b] = id
			}
		case "parallel":
			for _, br := range n.Branches {
				for _, b := range br {
					owned[b] = id
				}
			}
		}
	}

	// edge validates a single outgoing edge target (empty = ends the flow).
	edge := func(from, label, to string) {
		if to == "" {
			return
		}
		if _, ok := fl.Nodes[to]; !ok {
			msgs = append(msgs, fmt.Sprintf("node %q: %s edge targets missing node %q", from, label, to))
		} else if owner, isOwned := owned[to]; isOwned {
			msgs = append(msgs, fmt.Sprintf("node %q: %s edge targets %q, which is owned by %q (loop/parallel body) — don't also target it from the main graph", from, label, to, owner))
		}
	}

	// bodyRef validates a loop/parallel body id: must exist and be a body type.
	bodyRef := func(from, id string) {
		n, ok := fl.Nodes[id]
		if !ok {
			msgs = append(msgs, fmt.Sprintf("node %q: body references missing node %q", from, id))
			return
		}
		if !bodyTypes[n.Type] {
			msgs = append(msgs, fmt.Sprintf("node %q: body node %q is type %q — loop/parallel bodies support request/setvar/delay only", from, id, n.Type))
		}
	}

	for _, id := range sortedKeys(fl.Nodes) {
		n := fl.Nodes[id]
		switch n.Type {
		case "request":
			if n.Request == "" {
				msgs = append(msgs, fmt.Sprintf("node %q: request node missing request path", id))
			}
			edge(id, "next", n.Next)
		case "branch":
			if n.Cond == nil {
				msgs = append(msgs, fmt.Sprintf("node %q: branch node missing cond", id))
			}
			edge(id, "onTrue", n.OnTrue)
			edge(id, "onFalse", n.OnFalse)
		case "setvar":
			if n.Var == "" {
				msgs = append(msgs, fmt.Sprintf("node %q: setvar node missing var", id))
			}
			edge(id, "next", n.Next)
		case "delay":
			edge(id, "next", n.Next)
		case "loop":
			if n.Data == "" {
				msgs = append(msgs, fmt.Sprintf("node %q: loop node missing data file", id))
			}
			for _, b := range n.Body {
				bodyRef(id, b)
			}
			edge(id, "next", n.Next)
		case "parallel":
			for _, br := range n.Branches {
				for _, b := range br {
					bodyRef(id, b)
				}
			}
			edge(id, "next", n.Next)
		case "":
			msgs = append(msgs, fmt.Sprintf("node %q: missing type", id))
		default:
			msgs = append(msgs, fmt.Sprintf("node %q: unknown type %q", id, n.Type))
		}
	}

	return msgs
}

// sortedKeys yields node ids in a stable order so validation messages don't
// depend on Go's randomized map iteration.
func sortedKeys(m map[string]model.FlowNode) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
