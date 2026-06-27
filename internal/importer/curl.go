// Package importer converts external request formats (curl commands, Postman
// collections, OpenAPI specs, HAR captures) into Senda model.Request values. Imports preserve
// {{var}} placeholders verbatim where the source already uses that syntax.
package importer

import (
	"fmt"
	"strings"

	"senda/internal/model"
)

// Curl parses a single curl command line into a Request. It understands the
// common flags (-X, -H, -d/--data*, -F, -u, -G, --url) and ignores the rest.
func Curl(cmd string) (model.Request, error) {
	toks, err := tokenize(cmd)
	if err != nil {
		return model.Request{}, err
	}
	// Drop a leading "curl" word if present.
	if len(toks) > 0 && strings.EqualFold(toks[0], "curl") {
		toks = toks[1:]
	}
	if len(toks) == 0 {
		return model.Request{}, fmt.Errorf("empty curl command")
	}

	req := model.Request{Method: "", Body: model.Body{Type: model.BodyNone}, Auth: model.Auth{Type: model.AuthInherit}}
	var dataParts []string
	var formParts []model.KV
	getWithData := false // -G: send data as query string

	next := func(i *int) string {
		if *i+1 < len(toks) {
			*i++
			return toks[*i]
		}
		return ""
	}

	for i := 0; i < len(toks); i++ {
		t := toks[i]
		switch {
		case t == "-X" || t == "--request":
			req.Method = strings.ToUpper(next(&i))
		case t == "-H" || t == "--header":
			addHeader(&req, next(&i))
		case t == "-d" || t == "--data" || t == "--data-raw" || t == "--data-binary" || t == "--data-ascii":
			dataParts = append(dataParts, next(&i))
		case t == "--data-urlencode":
			dataParts = append(dataParts, next(&i))
		case t == "-F" || t == "--form":
			if k, v, ok := splitKV(next(&i)); ok {
				formParts = append(formParts, model.KV{Key: k, Value: strings.TrimPrefix(v, "@"), Enabled: true})
			}
		case t == "-u" || t == "--user":
			user := next(&i)
			name, pass, _ := strings.Cut(user, ":")
			req.Auth = model.Auth{Type: model.AuthBasic, Username: name, Password: pass}
		case t == "-G" || t == "--get":
			getWithData = true
		case t == "--url":
			req.URL = next(&i)
		case t == "-A" || t == "--user-agent":
			addHeader(&req, "User-Agent: "+next(&i))
		case t == "-e" || t == "--referer":
			addHeader(&req, "Referer: "+next(&i))
		case t == "-b" || t == "--cookie":
			addHeader(&req, "Cookie: "+next(&i))
		case t == "--compressed" || t == "-s" || t == "--silent" || t == "-L" || t == "--location" ||
			t == "-k" || t == "--insecure" || t == "-i" || t == "--include" || t == "-v" || t == "--verbose":
			// no-op flags
		case strings.HasPrefix(t, "-"):
			// Unknown flag: skip it (and a likely value is left for the loop).
		default:
			if req.URL == "" {
				req.URL = t
			}
		}
	}

	// Assemble body / query from collected -d data.
	data := strings.Join(dataParts, "&")
	if getWithData && data != "" {
		req.URL = appendQuery(req.URL, data)
		dataParts = nil
		data = ""
	}

	switch {
	case len(formParts) > 0:
		req.Body = model.Body{Type: model.BodyForm, Form: formParts}
	case data != "":
		if looksJSON(data) {
			req.Body = model.Body{Type: model.BodyJSON, Raw: data}
		} else {
			req.Body = model.Body{Type: model.BodyRaw, Raw: data}
		}
	}

	if req.Method == "" {
		if req.Body.Type != model.BodyNone || data != "" {
			req.Method = "POST"
		} else {
			req.Method = "GET"
		}
	}
	if req.URL == "" {
		return req, fmt.Errorf("no URL found in curl command")
	}
	req.Name = deriveName(req.URL)
	return req, nil
}

func addHeader(req *model.Request, raw string) {
	k, v, ok := strings.Cut(raw, ":")
	if !ok {
		return
	}
	req.Headers = append(req.Headers, model.KV{
		Key:     strings.TrimSpace(k),
		Value:   strings.TrimSpace(v),
		Enabled: true,
	})
}

func splitKV(s string) (string, string, bool) {
	k, v, ok := strings.Cut(s, "=")
	if !ok {
		return "", "", false
	}
	return k, v, true
}

func appendQuery(rawURL, data string) string {
	if strings.Contains(rawURL, "?") {
		return rawURL + "&" + data
	}
	return rawURL + "?" + data
}

func looksJSON(s string) bool {
	t := strings.TrimSpace(s)
	return strings.HasPrefix(t, "{") || strings.HasPrefix(t, "[")
}

// deriveName produces a readable request name from a URL's last path segment.
func deriveName(rawURL string) string {
	u := rawURL
	if i := strings.Index(u, "?"); i >= 0 {
		u = u[:i]
	}
	u = strings.TrimRight(u, "/")
	seg := u
	if i := strings.LastIndex(u, "/"); i >= 0 && i+1 < len(u) {
		seg = u[i+1:]
	}
	seg = strings.TrimSpace(seg)
	if seg == "" || strings.Contains(seg, "://") {
		return "imported"
	}
	return seg
}

// tokenize splits a shell-ish command into argv, honouring single/double
// quotes and backslash line-continuations. It is not a full shell parser but
// covers the forms curl commands are pasted in.
func tokenize(s string) ([]string, error) {
	var toks []string
	var cur strings.Builder
	inTok := false
	flush := func() {
		if inTok {
			toks = append(toks, cur.String())
			cur.Reset()
			inTok = false
		}
	}

	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		c := runes[i]
		switch c {
		case ' ', '\t', '\n', '\r':
			flush()
		case '\\':
			// Line continuation: swallow the newline. Otherwise escape next char.
			if i+1 < len(runes) && (runes[i+1] == '\n' || runes[i+1] == '\r') {
				i++
				continue
			}
			if i+1 < len(runes) {
				i++
				cur.WriteRune(runes[i])
				inTok = true
			}
		case '\'':
			inTok = true
			for i++; i < len(runes) && runes[i] != '\''; i++ {
				cur.WriteRune(runes[i])
			}
		case '"':
			inTok = true
			for i++; i < len(runes) && runes[i] != '"'; i++ {
				if runes[i] == '\\' && i+1 < len(runes) {
					nxt := runes[i+1]
					if nxt == '"' || nxt == '\\' || nxt == '$' || nxt == '`' {
						i++
						cur.WriteRune(runes[i])
						continue
					}
				}
				cur.WriteRune(runes[i])
			}
		case '$':
			// Strip $'...' / $"..." prefix dollar; keep the quote handling.
			inTok = true
		default:
			inTok = true
			cur.WriteRune(c)
		}
	}
	flush()
	return toks, nil
}
