package flow

import (
	"strings"
	"testing"

	"senda/internal/model"
)

func TestValidate(t *testing.T) {
	cases := []struct {
		name string
		fl   model.Flow
		want string // substring expected in some message; "" = expect valid
	}{
		{
			name: "valid graph",
			fl: model.Flow{Start: "a", Nodes: map[string]model.FlowNode{
				"a": {Type: "request", Request: "r.yaml", Next: "b"},
				"b": {Type: "branch", Cond: &model.FlowCond{Left: "x", Op: "eq", Right: "1"}, OnTrue: "", OnFalse: ""},
			}},
			want: "",
		},
		{
			name: "missing start",
			fl:   model.Flow{Nodes: map[string]model.FlowNode{"a": {Type: "request", Request: "r.yaml"}}},
			want: "no start node",
		},
		{
			name: "start not found",
			fl:   model.Flow{Start: "ghost", Nodes: map[string]model.FlowNode{"a": {Type: "request", Request: "r.yaml"}}},
			want: "start node \"ghost\" not found",
		},
		{
			name: "dangling edge",
			fl: model.Flow{Start: "a", Nodes: map[string]model.FlowNode{
				"a": {Type: "request", Request: "r.yaml", Next: "typo"},
			}},
			want: "targets missing node \"typo\"",
		},
		{
			name: "branch missing cond",
			fl: model.Flow{Start: "a", Nodes: map[string]model.FlowNode{
				"a": {Type: "branch"},
			}},
			want: "missing cond",
		},
		{
			name: "loop body wrong type",
			fl: model.Flow{Start: "a", Nodes: map[string]model.FlowNode{
				"a": {Type: "loop", Data: "d.csv", Body: []string{"b"}},
				"b": {Type: "branch", Cond: &model.FlowCond{}},
			}},
			want: "support request/setvar/delay only",
		},
		{
			name: "owned id also main-graph target",
			fl: model.Flow{Start: "a", Nodes: map[string]model.FlowNode{
				"a": {Type: "loop", Data: "d.csv", Body: []string{"b"}, Next: "b"},
				"b": {Type: "request", Request: "r.yaml"},
			}},
			want: "owned by",
		},
		{
			name: "request missing path",
			fl: model.Flow{Start: "a", Nodes: map[string]model.FlowNode{
				"a": {Type: "request"},
			}},
			want: "missing request path",
		},
		{
			name: "unknown type",
			fl: model.Flow{Start: "a", Nodes: map[string]model.FlowNode{
				"a": {Type: "wat"},
			}},
			want: "unknown type",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			msgs := Validate(tc.fl)
			if tc.want == "" {
				if len(msgs) != 0 {
					t.Fatalf("expected valid, got %v", msgs)
				}
				return
			}
			if !strings.Contains(strings.Join(msgs, "\n"), tc.want) {
				t.Fatalf("expected a message containing %q, got %v", tc.want, msgs)
			}
		})
	}
}
