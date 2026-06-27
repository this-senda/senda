// Package flow executes a *.flow.yaml graph: requests wired with branching,
// loops, parallel fan-out, delays and declarative variable extraction. It is
// transport-agnostic — the caller injects how to send a request and how to
// resolve {{...}} — so it unit-tests without a network, and reuses the folder
// runner to build each request's RunResult.
package flow

import (
	"context"
	"fmt"
	"sync"
	"time"

	"senda/internal/assert"
	"senda/internal/model"
	"senda/internal/runner"
)

// maxSteps bounds the main graph walk so a cycle (e.g. a branch looping back
// forever) can't hang the run.
// ponytail: global step cap; raise if real flows legitimately iterate deeper.
const maxSteps = 5000

// StepResult is the outcome of executing one flow node. Request nodes carry a
// RunResult (so existing reporters/panels render them unchanged); branch nodes
// record the taken edge in Branch.
type StepResult struct {
	NodeID string           `json:"nodeId"`
	Type   string           `json:"type"`
	Branch string           `json:"branch,omitempty"` // "true"/"false" for branch nodes
	Result *model.RunResult `json:"result,omitempty"` // request nodes (incl. loop/parallel bodies)
	Err    string           `json:"err,omitempty"`
}

// Runner carries the injected dependencies for executing a flow. It holds no
// mutable state, so it is safe to pass by value.
type Runner struct {
	// MakeSend builds a Send that injects extra runtime vars (a data row); pass
	// nil extra for the common case. Mirrors runner.RunFolderWithData's hook.
	MakeSend func(extra map[string]string) runner.Send
	// Resolve interpolates {{...}} in branch conditions and setvar sources.
	Resolve func(s string) string
	// Data loads a loop's data file; defaults to runner.LoadDataFile.
	Data func(path string) ([]map[string]string, error)
	// SetVar stores a runtime variable (setvar nodes).
	SetVar func(key, value string)
	// OnStep, if set, streams each StepResult as it completes.
	OnStep func(StepResult)
}

// exec is the per-run mutable state. Kept separate from Runner so Runner stays
// copy-safe (no lock) and exec is only ever shared by pointer (parallel nodes).
type exec struct {
	r   Runner
	mu  sync.Mutex // guards out append + OnStep during parallel branches
	out []StepResult
}

// Run executes the flow starting at fl.Start, following each node's edge until a
// node has no outgoing edge. It returns every step in execution order.
func Run(ctx context.Context, fl model.Flow, r Runner) ([]StepResult, error) {
	if r.Data == nil {
		r.Data = runner.LoadDataFile
	}
	e := &exec{r: r, out: make([]StepResult, 0, len(fl.Nodes))}

	if fl.Start == "" {
		return e.out, fmt.Errorf("flow has no start node")
	}
	cur := fl.Start
	steps := 0
	for cur != "" {
		if err := ctx.Err(); err != nil {
			return e.out, err
		}
		steps++
		if steps > maxSteps {
			return e.out, fmt.Errorf("flow exceeded %d steps (cycle?)", maxSteps)
		}
		node, ok := fl.Nodes[cur]
		if !ok {
			return e.out, fmt.Errorf("flow node %q not found", cur)
		}
		sr := StepResult{NodeID: cur, Type: node.Type}
		next := node.Next

		switch node.Type {
		case "request":
			rr := e.runRequest(ctx, node.Request, nil)
			sr.Result = &rr
			sr.Err = rr.Error
		case "branch":
			pass, err := e.evalBranch(node)
			if err != nil {
				sr.Err = err.Error()
			} else if pass {
				sr.Branch, next = "true", node.OnTrue
			} else {
				sr.Branch, next = "false", node.OnFalse
			}
		case "setvar":
			e.r.SetVar(node.Var, e.r.Resolve(node.From))
		case "delay":
			if err := sleep(ctx, node.Ms); err != nil {
				return e.out, err
			}
		case "loop":
			e.record(sr) // mark the loop, then run its body per row
			if err := e.runLoop(ctx, fl, node); err != nil {
				e.record(StepResult{NodeID: cur, Type: node.Type, Err: err.Error()})
			}
			cur = next
			continue
		case "parallel":
			e.record(sr)
			e.runParallel(ctx, fl, node)
			cur = next
			continue
		default:
			sr.Err = "unknown node type " + node.Type
		}

		e.record(sr)
		cur = next
	}
	return e.out, nil
}

// runRequest sends one request via a one-element folder run, reusing the runner's
// RunResult building (assert tally, OK/error classification).
func (e *exec) runRequest(ctx context.Context, path string, extra map[string]string) model.RunResult {
	send := e.r.MakeSend(extra)
	rs := runner.RunFolder(ctx, []string{path}, send, nil)
	if len(rs) == 0 {
		return model.RunResult{Path: path, Error: "no result"}
	}
	return rs[0]
}

func (e *exec) evalBranch(node model.FlowNode) (bool, error) {
	if node.Cond == nil {
		return false, fmt.Errorf("branch node missing cond")
	}
	left := e.r.Resolve(node.Cond.Left)
	right := e.r.Resolve(node.Cond.Right)
	return assert.Compare(node.Cond.Op, left, right)
}

// runLoop runs the body node list once per row of the data file, injecting the
// row as extra runtime vars for request nodes.
func (e *exec) runLoop(ctx context.Context, fl model.Flow, node model.FlowNode) error {
	rows, err := e.r.Data(e.r.Resolve(node.Data))
	if err != nil {
		return err
	}
	for _, row := range rows {
		if err := ctx.Err(); err != nil {
			return err
		}
		e.runLinear(ctx, fl, node.Body, row)
	}
	return nil
}

// runParallel runs each branch (a linear node list) concurrently and waits.
func (e *exec) runParallel(ctx context.Context, fl model.Flow, node model.FlowNode) {
	var wg sync.WaitGroup
	for _, branch := range node.Branches {
		wg.Add(1)
		go func(ids []string) {
			defer wg.Done()
			e.runLinear(ctx, fl, ids, nil)
		}(branch)
	}
	wg.Wait()
}

// runLinear runs a list of node ids in order without following their edges —
// used for loop bodies and parallel branches. Only request/setvar/delay nodes
// are meaningful here.
func (e *exec) runLinear(ctx context.Context, fl model.Flow, ids []string, extra map[string]string) {
	for _, id := range ids {
		if ctx.Err() != nil {
			return
		}
		node, ok := fl.Nodes[id]
		if !ok {
			e.record(StepResult{NodeID: id, Err: "node not found"})
			continue
		}
		sr := StepResult{NodeID: id, Type: node.Type}
		switch node.Type {
		case "request":
			rr := e.runRequest(ctx, node.Request, extra)
			sr.Result = &rr
			sr.Err = rr.Error
		case "setvar":
			e.r.SetVar(node.Var, e.r.Resolve(node.From))
		case "delay":
			_ = sleep(ctx, node.Ms)
		default:
			sr.Err = "loop/parallel body supports request/setvar/delay only, got " + node.Type
		}
		e.record(sr)
	}
}

// record appends a step and streams it, guarded so parallel branches are safe.
func (e *exec) record(sr StepResult) {
	e.mu.Lock()
	e.out = append(e.out, sr)
	if e.r.OnStep != nil {
		e.r.OnStep(sr)
	}
	e.mu.Unlock()
}

func sleep(ctx context.Context, ms int) error {
	if ms <= 0 {
		return nil
	}
	t := time.NewTimer(time.Duration(ms) * time.Millisecond)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
