package docgen

import (
	"encoding/json"
	"html/template"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"senda/internal/codegen"
	"senda/internal/model"
	"senda/internal/store"
)

// page is the view model handed to the HTML template. It is a flattened,
// render-ready projection of a Collection + its requests — the template does
// no logic beyond ranging over these slices.
type page struct {
	Title    string // "<name> API" — used for <title> and the H1
	Brand    string // raw collection name — sidebar brand line
	Host     string // e.g. api.senda.travel (empty if first URL has no host)
	Version  string // e.g. v1 (empty if no /vN/ path segment found)
	Intro    template.HTML
	HasAuth  bool
	AuthDesc template.HTML
	AuthCurl string
	Groups   []navGroup
	Items    []endpoint
	Schemas  []schema
}

type navGroup struct {
	Title string
	Links []navLink
}

type navLink struct {
	ID      string
	Label   string
	Method  string // endpoint badge; "" for plain links
	Diamond bool   // schema links render a diamond glyph
}

type endpoint struct {
	ID          string
	Name        string
	Method      string
	Path        string // URL path portion (host stripped)
	URL         string
	Docs        template.HTML
	PathParams  []model.KV
	Params      []model.KV // query params
	BodyParams  []model.KV // json body top-level fields
	Headers     []model.KV
	Asserts     []model.Assert
	Curl        string
	JS          string // fetch snippet (JS tab)
	Python      string // requests snippet (Python tab)
	RespSchema  string // pretty-printed response schema, "" if none
	RespExample string // pretty-printed example response body, "" if none
	RespStatus  string // e.g. "201 Created", derived from the status assertion
	ReqJSON     string // JSON {method,url,headers,body} for the in-page "Try it" sender
}

type schema struct {
	ID   string
	Name string
	Code string
}

var verSegRe = regexp.MustCompile(`/(v\d+)(/|$)`)

// GenerateHTML produces a self-contained three-column API docs site (nav +
// prose + code samples, light/dark) for the collection under collPath.
func GenerateHTML(collPath, subPath string) (string, error) {
	reqs, err := loadRequests(collPath, subPath)
	if err != nil {
		return "", err
	}
	coll, _ := store.OpenCollection(collPath)
	p := buildPage(coll, reqs)

	var b strings.Builder
	if err := siteTmpl.Execute(&b, p); err != nil {
		return "", err
	}
	return b.String(), nil
}

func buildPage(coll model.Collection, reqs []model.Request) page {
	name := coll.Name
	if name == "" {
		name = "API"
	}
	p := page{Title: name + " API", Brand: name}

	if coll.Description != "" {
		p.Intro = template.HTML(RenderFragment(coll.Description))
	}

	// Host + version derived from the first parseable request URL.
	for _, r := range reqs {
		if u, err := url.Parse(r.URL); err == nil && u.Host != "" {
			p.Host = u.Host
			break
		}
	}
	for _, r := range reqs {
		if m := verSegRe.FindStringSubmatch(r.URL); m != nil {
			p.Version = m[1]
			break
		}
	}

	gettingStarted := navGroup{Title: "Getting Started", Links: []navLink{
		{ID: "introduction", Label: "Introduction"},
	}}
	if coll.Auth.Type != "" && coll.Auth.Type != model.AuthNone && coll.Auth.Type != model.AuthInherit {
		p.HasAuth = true
		p.AuthDesc = template.HTML(RenderFragment(authDescription(coll.Auth)))
		gettingStarted.Links = append(gettingStarted.Links, navLink{ID: "authentication", Label: "Authentication"})
	}
	gettingStarted.Links = append(gettingStarted.Links, navLink{ID: "errors", Label: "Errors"})

	endpoints := navGroup{Title: "Endpoints"}
	for i, r := range reqs {
		e := buildEndpoint(i, r)
		if p.HasAuth && p.AuthCurl == "" {
			p.AuthCurl = e.Curl // first endpoint's curl doubles as the auth sample
		}
		p.Items = append(p.Items, e)
		endpoints.Links = append(endpoints.Links, navLink{ID: e.ID, Label: e.Name, Method: e.Method})
	}

	p.Schemas = buildSchemas(reqs)
	schemas := navGroup{Title: "Schemas"}
	for _, s := range p.Schemas {
		schemas.Links = append(schemas.Links, navLink{ID: s.ID, Label: s.Name, Diamond: true})
	}

	p.Groups = append(p.Groups, gettingStarted, endpoints)
	if len(p.Schemas) > 0 {
		p.Groups = append(p.Groups, schemas)
	}
	return p
}

func buildEndpoint(i int, r model.Request) endpoint {
	method := r.Method
	if method == "" {
		method = "GET"
	}
	name := r.Name
	if name == "" {
		name = r.URL
	}
	e := endpoint{
		ID:     "endpoint-" + itoa(i),
		Name:   name,
		Method: strings.ToUpper(method),
		Path:   urlPath(r.URL),
		URL:    r.URL,
	}
	if r.Docs != "" {
		e.Docs = template.HTML(RenderFragment(r.Docs))
	}
	// Param tables show every documented param (required + optional) with a
	// badge; unlike the live sender we don't filter by Enabled here.
	e.PathParams = r.PathParams
	e.Params = r.Params
	e.BodyParams = r.Body.Fields
	for _, kv := range r.Headers {
		if kv.Enabled {
			e.Headers = append(e.Headers, kv)
		}
	}
	for _, a := range r.Asserts {
		if a.Enabled {
			e.Asserts = append(e.Asserts, a)
		}
		if a.Target == "status" && a.Value != "" && e.RespStatus == "" {
			if n, err := strconv.Atoi(a.Value); err == nil {
				e.RespStatus = strings.TrimSpace(a.Value + " " + http.StatusText(n))
			}
		}
	}
	if s, err := codegen.Generate(r, "curl"); err == nil {
		e.Curl = s
	}
	if s, err := codegen.Generate(r, "fetch"); err == nil {
		e.JS = s
	}
	if s, err := codegen.Generate(r, "python"); err == nil {
		e.Python = s
	}
	if r.ResponseSchema != "" {
		e.RespSchema = prettyJSON(r.ResponseSchema, model.BodyJSON)
	}
	if r.ResponseExample != "" {
		e.RespExample = prettyJSON(r.ResponseExample, model.BodyJSON)
	}
	e.ReqJSON = reqJSON(e.Method, r)
	return e
}

// reqJSON serialises the bits the in-page "Try it" sender needs into a compact
// JSON blob embedded in a data attribute. {{vars}} are left intact — the
// browser sender lets the reader edit the URL before sending.
func reqJSON(method string, r model.Request) string {
	d := struct {
		Method  string      `json:"method"`
		URL     string      `json:"url"`
		Headers [][2]string `json:"headers"`
		Body    string      `json:"body,omitempty"`
	}{Method: method, URL: r.URL}
	for _, h := range r.Headers {
		if h.Enabled {
			d.Headers = append(d.Headers, [2]string{h.Key, h.Value})
		}
	}
	if r.Body.Type == model.BodyJSON || r.Body.Type == model.BodyRaw || r.Body.Type == model.BodyGraphQL {
		d.Body = r.Body.Raw
	}
	// JSON/GraphQL bodies need an explicit Content-Type, else the browser's
	// fetch() defaults a string body to text/plain and servers (e.g. Apollo)
	// reject it. curl codegen adds this; mirror it for the live "Try it" send.
	if r.Body.Type == model.BodyJSON || r.Body.Type == model.BodyGraphQL {
		hasCT := false
		for _, h := range d.Headers {
			if strings.EqualFold(h[0], "Content-Type") {
				hasCT = true
				break
			}
		}
		if !hasCT {
			d.Headers = append(d.Headers, [2]string{"Content-Type", "application/json"})
		}
	}
	b, err := json.Marshal(d)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// buildSchemas collects distinct response schemas. Each schema's name comes
// from its JSON Schema "title", falling back to the request name.
func buildSchemas(reqs []model.Request) []schema {
	var out []schema
	seen := map[string]bool{}
	for i, r := range reqs {
		if r.ResponseSchema == "" {
			continue
		}
		name := schemaTitle(r.ResponseSchema)
		if name == "" {
			name = r.Name
		}
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, schema{
			ID:   "schema-" + itoa(i),
			Name: name,
			Code: prettyJSON(r.ResponseSchema, model.BodyJSON),
		})
	}
	return out
}

func schemaTitle(s string) string {
	var m struct {
		Title string `json:"title"`
	}
	_ = json.Unmarshal([]byte(s), &m)
	return m.Title
}

func authDescription(a model.Auth) string {
	switch a.Type {
	case model.AuthBearer, model.AuthOAuth2:
		return "Authenticate by passing your secret key as a Bearer token in the `Authorization` header. Keys are environment-scoped."
	case model.AuthBasic:
		return "Authenticate with HTTP Basic auth: send your username and password in the `Authorization` header."
	case model.AuthAPIKey:
		where := "header"
		if a.Placement == model.APIKeyQuery {
			where = "query parameter"
		}
		return "Authenticate by sending your API key in the `" + a.Key + "` " + where + " with every request."
	default:
		return "This API requires authentication on every request."
	}
}

// urlPath returns the path portion of a URL, preserving literal characters
// like {id} (url.EscapedPath would turn those into %7Bid%7D). Query string is
// dropped — the route display shows the path only.
func urlPath(raw string) string {
	if i := strings.Index(raw, "://"); i >= 0 {
		raw = raw[i+3:]
		if j := strings.IndexByte(raw, '/'); j >= 0 {
			raw = raw[j:]
		} else {
			return "/"
		}
	}
	if q := strings.IndexByte(raw, '?'); q >= 0 {
		raw = raw[:q]
	}
	return raw
}

func itoa(i int) string { return strconv.Itoa(i) }
