package flow

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"senda/internal/model"
	"senda/internal/runner"
	"senda/internal/vars"
)

// harness builds a Runner backed by an in-memory var map and a send that records
// which request paths ran (and any injected data-row var).
type harness struct {
	vars  map[string]string
	mu    sync.Mutex
	sent  []string
	count int32
}

func newHarness(initial map[string]string) *harness {
	v := map[string]string{}
	for k, val := range initial {
		v[k] = val
	}
	return &harness{vars: v}
}

func (h *harness) runner() Runner {
	return Runner{
		MakeSend: func(extra map[string]string) runner.Send {
			return func(ctx context.Context, path string) (model.Request, model.Response, error) {
				atomic.AddInt32(&h.count, 1)
				h.mu.Lock()
				h.sent = append(h.sent, path)
				if v, ok := extra["row"]; ok {
					h.sent = append(h.sent, "row="+v)
				}
				h.mu.Unlock()
				return model.Request{Name: path}, model.Response{Status: 200}, nil
			}
		},
		Resolve: func(s string) string {
			kvs := make([]model.KV, 0, len(h.vars))
			h.mu.Lock()
			for k, v := range h.vars {
				kvs = append(kvs, model.KV{Key: k, Value: v, Enabled: true})
			}
			h.mu.Unlock()
			return vars.Build(kvs).Apply(s)
		},
		SetVar: func(k, v string) {
			h.mu.Lock()
			h.vars[k] = v
			h.mu.Unlock()
		},
	}
}

func TestBranchTakesTrueEdge(t *testing.T) {
	h := newHarness(map[string]string{"status": "200"})
	fl := model.Flow{Start: "check", Nodes: map[string]model.FlowNode{
		"check": {Type: "branch", Cond: &model.FlowCond{Left: "{{status}}", Op: "eq", Right: "200"}, OnTrue: "ok", OnFalse: "bad"},
		"ok":    {Type: "request", Request: "ok.yaml"},
		"bad":   {Type: "request", Request: "bad.yaml"},
	}}
	steps, err := Run(context.Background(), fl, h.runner())
	if err != nil {
		t.Fatal(err)
	}
	if steps[0].Branch != "true" {
		t.Errorf("branch = %q, want true", steps[0].Branch)
	}
	if len(h.sent) != 1 || h.sent[0] != "ok.yaml" {
		t.Errorf("sent = %v, want [ok.yaml]", h.sent)
	}
}

func TestBranchTakesFalseEdge(t *testing.T) {
	h := newHarness(map[string]string{"status": "500"})
	fl := model.Flow{Start: "check", Nodes: map[string]model.FlowNode{
		"check": {Type: "branch", Cond: &model.FlowCond{Left: "{{status}}", Op: "eq", Right: "200"}, OnTrue: "ok", OnFalse: "bad"},
		"ok":    {Type: "request", Request: "ok.yaml"},
		"bad":   {Type: "request", Request: "bad.yaml"},
	}}
	if _, err := Run(context.Background(), fl, h.runner()); err != nil {
		t.Fatal(err)
	}
	if len(h.sent) != 1 || h.sent[0] != "bad.yaml" {
		t.Errorf("sent = %v, want [bad.yaml]", h.sent)
	}
}

func TestSetVarThenUse(t *testing.T) {
	h := newHarness(map[string]string{"src": "abc"})
	fl := model.Flow{Start: "set", Nodes: map[string]model.FlowNode{
		"set": {Type: "setvar", Var: "token", From: "{{src}}", Next: "use"},
		"use": {Type: "request", Request: "use.yaml"},
	}}
	if _, err := Run(context.Background(), fl, h.runner()); err != nil {
		t.Fatal(err)
	}
	if h.vars["token"] != "abc" {
		t.Errorf("token = %q, want abc", h.vars["token"])
	}
}

func TestLoopRunsBodyPerRow(t *testing.T) {
	h := newHarness(nil)
	r := h.runner()
	r.Data = func(path string) ([]map[string]string, error) {
		return []map[string]string{{"row": "1"}, {"row": "2"}}, nil
	}
	fl := model.Flow{Start: "loop", Nodes: map[string]model.FlowNode{
		"loop": {Type: "loop", Data: "rows.csv", Body: []string{"req"}, Next: "done"},
		"req":  {Type: "request", Request: "req.yaml"},
		"done": {Type: "request", Request: "done.yaml"},
	}}
	if _, err := Run(context.Background(), fl, r); err != nil {
		t.Fatal(err)
	}
	// req runs once per row (2) + done once = 3 sends.
	if got := atomic.LoadInt32(&h.count); got != 3 {
		t.Errorf("sends = %d, want 3", got)
	}
	joined := strings.Join(h.sent, ",")
	if !strings.Contains(joined, "row=1") || !strings.Contains(joined, "row=2") {
		t.Errorf("data row not injected: %v", h.sent)
	}
}

func TestParallelRunsAllBranches(t *testing.T) {
	h := newHarness(nil)
	fl := model.Flow{Start: "fan", Nodes: map[string]model.FlowNode{
		"fan":  {Type: "parallel", Branches: [][]string{{"a"}, {"b"}, {"c"}}, Next: "done"},
		"a":    {Type: "request", Request: "a.yaml"},
		"b":    {Type: "request", Request: "b.yaml"},
		"c":    {Type: "request", Request: "c.yaml"},
		"done": {Type: "request", Request: "done.yaml"},
	}}
	if _, err := Run(context.Background(), fl, h.runner()); err != nil {
		t.Fatal(err)
	}
	if got := atomic.LoadInt32(&h.count); got != 4 {
		t.Errorf("sends = %d, want 4 (3 parallel + done)", got)
	}
}

func TestCycleHitsStepCap(t *testing.T) {
	h := newHarness(nil)
	fl := model.Flow{Start: "a", Nodes: map[string]model.FlowNode{
		"a": {Type: "request", Request: "a.yaml", Next: "a"}, // self-loop forever
	}}
	_, err := Run(context.Background(), fl, h.runner())
	if err == nil || !strings.Contains(err.Error(), "exceeded") {
		t.Errorf("want step-cap error, got %v", err)
	}
}

func TestMissingStartNode(t *testing.T) {
	h := newHarness(nil)
	_, err := Run(context.Background(), model.Flow{Start: "nope", Nodes: map[string]model.FlowNode{}}, h.runner())
	if err == nil {
		t.Error("want error for missing node")
	}
}
