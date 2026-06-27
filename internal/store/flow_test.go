package store

import (
	"path/filepath"
	"testing"

	"senda/internal/model"
)

func TestFlowRoundTrip(t *testing.T) {
	root := t.TempDir()
	// .senda must exist for FlowsDir to sit under it; SaveFlow creates flows/.
	fl := model.Flow{
		Name:  "signup",
		Start: "login",
		Nodes: map[string]model.FlowNode{
			"login": {Type: "request", Request: "auth/login.yaml", Next: "check"},
			"check": {Type: "branch", Cond: &model.FlowCond{Left: "{{res.login.status}}", Op: "eq", Right: "200"}, OnTrue: "me", OnFalse: ""},
			"me":    {Type: "request", Request: "users/me.yaml"},
		},
	}
	if err := SaveFlow(root, fl); err != nil {
		t.Fatal(err)
	}

	flows, err := ListFlows(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(flows) != 1 || flows[0].Name != "signup" {
		t.Fatalf("ListFlows = %+v, want one named signup", flows)
	}

	got, err := ReadFlow(flows[0].Path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Start != "login" || len(got.Nodes) != 3 {
		t.Errorf("ReadFlow = %+v", got)
	}
	if got.Nodes["check"].Cond == nil || got.Nodes["check"].Cond.Left != "{{res.login.status}}" {
		t.Errorf("branch cond not round-tripped: %+v", got.Nodes["check"])
	}

	// ResolveFlow by name and by path.
	if p, err := ResolveFlow(root, "signup"); err != nil || p != flows[0].Path {
		t.Errorf("ResolveFlow by name = %q, %v", p, err)
	}
	if p, err := ResolveFlow(root, flows[0].Path); err != nil || p != flows[0].Path {
		t.Errorf("ResolveFlow by path = %q, %v", p, err)
	}

	// A flow file under .senda/ must NOT appear in the request tree.
	reqs, err := ListRequests(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range reqs {
		if filepath.Ext(r) == ".yaml" && isFlowFile(filepath.Base(r)) {
			t.Errorf("flow file leaked into request tree: %s", r)
		}
	}
}
