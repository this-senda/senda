package importer

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	yaml "go.yaml.in/yaml/v4"

	"senda/internal/model"
)

// OpenAPI parses an OpenAPI 3 document (JSON or YAML — JSON is valid YAML) into
// one request per path+operation. The first server URL is used as the base.
// $ref references (parameters, request bodies, schemas) are resolved by
// libopenapi before walking the model.
func OpenAPI(data []byte) ([]Imported, error) {
	// SkipExternalRefResolution leaves external file/remote refs (e.g. a docs
	// markdown file referenced from an extension) as-is instead of failing when
	// the sibling files aren't present. Internal #/components refs still resolve.
	doc, err := libopenapi.NewDocumentWithConfiguration(data, &datamodel.DocumentConfiguration{
		SkipExternalRefResolution: true,
	})
	if err != nil {
		return nil, fmt.Errorf("openapi: %w", err)
	}
	v3doc, err := doc.BuildV3Model()
	if err != nil {
		return nil, fmt.Errorf("openapi: %w", err)
	}
	if v3doc == nil || v3doc.Model.Paths == nil || v3doc.Model.Paths.PathItems == nil {
		return nil, fmt.Errorf("openapi: no paths found")
	}

	// Requests reference {{baseUrl}}; OpenAPIMeta turns each server into an
	// environment that supplies it. (Was: literal first-server URL baked in.)
	base := "{{baseUrl}}"

	// Security schemes (components) + the document-level default requirement, so
	// each operation can resolve its auth.
	schemes := map[string]*v3.SecurityScheme{}
	if c := v3doc.Model.Components; c != nil && c.SecuritySchemes != nil {
		for name, s := range c.SecuritySchemes.FromOldest() {
			schemes[name] = s
		}
	}
	globalSec := v3doc.Model.Security

	items := v3doc.Model.Paths.PathItems

	// Walk paths in sorted order for deterministic output.
	paths := make([]string, 0, items.Len())
	for p := range items.KeysFromOldest() {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	var out []Imported
	for _, p := range paths {
		item := items.GetOrZero(p)
		if item == nil {
			continue
		}
		// Path-level parameters apply to every operation under the path.
		for method, op := range item.GetOperations().FromOldest() {
			if op == nil {
				continue
			}
			out = append(out, Imported{
				Dir:     dirFromTags(op.Tags),
				Request: convertOpenAPI(base, p, method, op, item.Parameters, schemes, globalSec),
			})
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("openapi: no operations found")
	}
	return out, nil
}

func convertOpenAPI(base, path, method string, op *v3.Operation, pathParams []*v3.Parameter, schemes map[string]*v3.SecurityScheme, globalSec []*base.SecurityRequirement) model.Request {
	name := op.OperationId
	if name == "" {
		name = strings.ToLower(method) + "-" + strings.Trim(path, "/")
	}
	req := model.Request{
		Name:    sanitize(name),
		Method:  strings.ToUpper(method),
		URL:     base + pathToVars(path),
		Auth:    authForOp(op, schemes, globalSec),
		Body:    model.Body{Type: model.BodyNone},
		Docs:    docsFromOp(op),
		Asserts: assertsForOp(op),
	}
	// Operation parameters override path-level ones with the same name+location.
	seen := map[string]bool{}
	addParam := func(pr *v3.Parameter) {
		if pr == nil || pr.Name == "" {
			return
		}
		key := pr.In + "\x00" + pr.Name
		if seen[key] {
			return
		}
		seen[key] = true
		// Required params are enabled with their example/default prefilled; the
		// path itself already carries path params as {{vars}}.
		kv := model.KV{Key: pr.Name, Value: paramValue(pr), Enabled: paramRequired(pr), Desc: pr.Description}
		switch pr.In {
		case "query":
			req.Params = append(req.Params, kv)
		case "header":
			req.Headers = append(req.Headers, kv)
		}
	}
	for _, pr := range op.Parameters {
		addParam(pr)
	}
	for _, pr := range pathParams {
		addParam(pr)
	}

	if body, ct := bodyForOp(op.RequestBody); body.Type != model.BodyNone {
		req.Body = body
		if ct != "" {
			req.Headers = append(req.Headers, model.KV{Key: "Content-Type", Value: ct, Enabled: true})
		}
	}
	return req
}

// pathToVars rewrites OpenAPI path templating ({bookingId}) to Senda's variable
// syntax ({{bookingId}}) so path params resolve from vars/environment.
func pathToVars(path string) string {
	return braceParam.ReplaceAllString(path, "{{$1}}")
}

func paramRequired(pr *v3.Parameter) bool {
	return pr.Required != nil && *pr.Required
}

// paramValue prefills a param from its example, then schema example/default.
func paramValue(pr *v3.Parameter) string {
	if s := nodeScalar(pr.Example); s != "" {
		return s
	}
	if pr.Schema != nil {
		if sc := pr.Schema.Schema(); sc != nil {
			if s := nodeScalar(sc.Example); s != "" {
				return s
			}
			if s := nodeScalar(sc.Default); s != "" {
				return s
			}
		}
	}
	return ""
}

// nodeScalar decodes a YAML node to a flat string (scalars rendered directly,
// non-scalars left empty so we don't drop structured values into a KV cell).
func nodeScalar(n *yaml.Node) string {
	if n == nil {
		return ""
	}
	var v any
	if err := n.Decode(&v); err != nil || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case bool, int, int64, float64:
		return fmt.Sprint(t)
	default:
		return ""
	}
}

// bodyForOp builds a request body from the documented content, preferring JSON
// (example, else schema skeleton), then form / multipart with rows derived from
// the schema. Returns the body and the Content-Type header value to set.
func bodyForOp(rb *v3.RequestBody) (model.Body, string) {
	if rb == nil || rb.Content == nil {
		return model.Body{Type: model.BodyNone}, ""
	}
	if mt := jsonMediaType(rb.Content); mt != nil {
		if v, ok := mediaTypeExample(mt); ok {
			if raw, err := marshalJSON(v); err == nil {
				return model.Body{Type: model.BodyJSON, Raw: raw}, "application/json"
			}
		}
		if raw, ok := schemaJSON(mt.Schema); ok {
			return model.Body{Type: model.BodyJSON, Raw: raw}, "application/json"
		}
		return model.Body{Type: model.BodyJSON}, "application/json"
	}
	if mt := rb.Content.GetOrZero("application/x-www-form-urlencoded"); mt != nil {
		return model.Body{Type: model.BodyForm, Form: formRows(mt)}, ""
	}
	if mt := rb.Content.GetOrZero("multipart/form-data"); mt != nil {
		return model.Body{Type: model.BodyMultipart, Form: formRows(mt)}, ""
	}
	return model.Body{Type: model.BodyNone}, ""
}

// formRows turns a form body's top-level schema properties into KV rows.
func formRows(mt *v3.MediaType) []model.KV {
	if mt == nil || mt.Schema == nil {
		return nil
	}
	sc := mt.Schema.Schema()
	if sc == nil || sc.Properties == nil {
		return nil
	}
	required := map[string]bool{}
	for _, r := range sc.Required {
		required[r] = true
	}
	var rows []model.KV
	for name := range sc.Properties.KeysFromOldest() {
		rows = append(rows, model.KV{Key: name, Enabled: required[name]})
	}
	return rows
}

// assertsForOp seeds a status-code assertion from the documented success
// response (lowest 2xx, else the bare default mapped to 200).
func assertsForOp(op *v3.Operation) []model.Assert {
	if op.Responses == nil {
		return nil
	}
	best := 0
	if op.Responses.Codes != nil {
		for code := range op.Responses.Codes.FromOldest() {
			st, ok := statusCode(code)
			if !ok || st < 200 || st >= 300 {
				continue
			}
			if best == 0 || st < best {
				best = st
			}
		}
	}
	if best == 0 && op.Responses.Default != nil {
		best = 200
	}
	if best == 0 {
		return nil
	}
	return []model.Assert{{Target: "status", Op: "eq", Value: strconv.Itoa(best), Enabled: true}}
}

// authForOp maps the operation's (or document's default) security requirement
// to a Senda Auth. An explicit empty operation requirement means "no auth".
func authForOp(op *v3.Operation, schemes map[string]*v3.SecurityScheme, globalSec []*base.SecurityRequirement) model.Auth {
	reqs := globalSec
	if op.Security != nil { // operation overrides the document default
		if len(op.Security) == 0 {
			return model.Auth{Type: model.AuthNone}
		}
		reqs = op.Security
	}
	for _, r := range reqs {
		if r == nil || r.Requirements == nil {
			continue
		}
		for name := range r.Requirements.KeysFromOldest() {
			if s := schemes[name]; s != nil {
				return mapScheme(s)
			}
		}
	}
	return model.Auth{Type: model.AuthInherit}
}

// mapScheme converts one OpenAPI security scheme into a Senda Auth with
// {{placeholder}} credentials the user can fill from vars/environment.
func mapScheme(s *v3.SecurityScheme) model.Auth {
	switch strings.ToLower(s.Type) {
	case "http":
		switch strings.ToLower(s.Scheme) {
		case "basic":
			return model.Auth{Type: model.AuthBasic, Username: "{{username}}", Password: "{{password}}"}
		default: // bearer and other http schemes
			return model.Auth{Type: model.AuthBearer, Token: "{{token}}"}
		}
	case "apikey":
		placement := model.APIKeyHeader
		if strings.ToLower(s.In) == "query" {
			placement = model.APIKeyQuery
		}
		return model.Auth{Type: model.AuthAPIKey, Key: s.Name, KeyValue: "{{apiKey}}", Placement: placement}
	case "oauth2":
		a := model.Auth{Type: model.AuthOAuth2, Grant: model.OAuth2ClientCredentials, ClientID: "{{clientId}}", ClientSecret: "{{clientSecret}}"}
		if s.Flows != nil {
			switch {
			case s.Flows.ClientCredentials != nil:
				a.TokenURL, a.Scope = s.Flows.ClientCredentials.TokenUrl, scopeList(s.Flows.ClientCredentials)
			case s.Flows.Password != nil:
				a.Grant = model.OAuth2Password
				a.TokenURL, a.Scope = s.Flows.Password.TokenUrl, scopeList(s.Flows.Password)
				a.OAuthUsername, a.OAuthPassword = "{{username}}", "{{password}}"
			case s.Flows.AuthorizationCode != nil:
				a.TokenURL, a.Scope = s.Flows.AuthorizationCode.TokenUrl, scopeList(s.Flows.AuthorizationCode)
			}
		}
		return a
	case "openidconnect":
		return model.Auth{Type: model.AuthBearer, Token: "{{token}}"}
	}
	return model.Auth{Type: model.AuthInherit}
}

func scopeList(f *v3.OAuthFlow) string {
	if f == nil || f.Scopes == nil {
		return ""
	}
	var keys []string
	for k := range f.Scopes.KeysFromOldest() {
		keys = append(keys, k)
	}
	return strings.Join(keys, " ")
}

// docsFromOp builds the request's markdown doc block from the operation's
// summary and description. Summary becomes a bold lead line; description follows
// as prose. Either may be empty.
func docsFromOp(op *v3.Operation) string {
	var parts []string
	if s := strings.TrimSpace(op.Summary); s != "" {
		parts = append(parts, "**"+s+"**")
	}
	if d := strings.TrimSpace(op.Description); d != "" {
		parts = append(parts, d)
	}
	return strings.Join(parts, "\n\n")
}

// OpenAPIMeta extracts collection-level info from a spec: a markdown
// description (from info.title/description) and one environment per server, each
// supplying {{baseUrl}}. Kept separate from OpenAPI so the shared Imported shape
// stays request-only and OpenAPI's signature/tests don't churn.
// ponytail: re-parses the doc (import is a one-shot user action, not a hot path).
func OpenAPIMeta(data []byte) (description string, envs []model.Environment, err error) {
	doc, err := libopenapi.NewDocumentWithConfiguration(data, &datamodel.DocumentConfiguration{
		SkipExternalRefResolution: true,
	})
	if err != nil {
		return "", nil, fmt.Errorf("openapi: %w", err)
	}
	v3doc, err := doc.BuildV3Model()
	if err != nil {
		return "", nil, fmt.Errorf("openapi: %w", err)
	}

	if info := v3doc.Model.Info; info != nil {
		var parts []string
		if t := strings.TrimSpace(info.Title); t != "" {
			parts = append(parts, "# "+t)
		}
		if d := strings.TrimSpace(info.Description); d != "" {
			parts = append(parts, d)
		}
		description = strings.Join(parts, "\n\n")
	}

	seen := map[string]bool{}
	for i, s := range v3doc.Model.Servers {
		if s == nil || s.URL == "" {
			continue
		}
		name := serverEnvName(s, i)
		if seen[name] {
			name = fmt.Sprintf("%s-%d", name, i+1)
		}
		seen[name] = true
		envs = append(envs, model.Environment{
			Name: name,
			Vars: []model.KV{{Key: "baseUrl", Value: strings.TrimRight(s.URL, "/"), Enabled: true}},
		})
	}
	return description, envs, nil
}

// serverEnvName derives an environment name from a server's description, else
// its URL host, else its position.
func serverEnvName(s *v3.Server, i int) string {
	if d := strings.TrimSpace(s.Description); d != "" {
		return sanitize(d)
	}
	host := s.URL
	if u := strings.SplitN(strings.TrimPrefix(strings.TrimPrefix(s.URL, "https://"), "http://"), "/", 2); len(u) > 0 && u[0] != "" {
		host = u[0]
	}
	if host == "" {
		return fmt.Sprintf("server-%d", i+1)
	}
	return sanitize(host)
}

func dirFromTags(tags []string) []string {
	if len(tags) == 0 {
		return nil
	}
	return []string{sanitize(tags[0])}
}
