package core_test

import (
	"strings"
	"testing"

	"github.com/CompositeCode/substratalapps.com/internal/testenv"
)

// NFR → CORS: an allow-listed origin may call any endpoint with a user's
// own token; API Keys are refused from a browser.
func TestCORS(t *testing.T) {
	e := testenv.New(t)
	const origin = "https://dashboard.test"

	// Preflights aren't operations in openapi.yaml.
	e.Validate = false
	pre := e.Do(testenv.Req{Method: "OPTIONS", Path: "/users/me", Header: map[string]string{
		"Origin": origin, "Access-Control-Request-Method": "PATCH"}}).Expect(204)
	if got := pre.Header.Get("Access-Control-Allow-Origin"); got != origin {
		t.Fatalf("allow-origin = %q", got)
	}
	if got := pre.Header.Get("Access-Control-Max-Age"); got != "7200" {
		t.Fatalf("max-age = %q", got)
	}
	expose := pre.Header.Get("Access-Control-Expose-Headers")
	for _, h := range []string{"Retry-After", "Audit-Hot-Window-Start", "Substratal-Version", "ETag"} {
		if !strings.Contains(expose, h) {
			t.Fatalf("expose-headers %q lacks %s", expose, h)
		}
	}

	// An origin that isn't allow-listed gets no CORS headers at all.
	other := e.Do(testenv.Req{Method: "OPTIONS", Path: "/users/me", Header: map[string]string{
		"Origin": "https://evil.example", "Access-Control-Request-Method": "GET"}})
	if other.Header.Get("Access-Control-Allow-Origin") != "" || other.Header.Get("Access-Control-Max-Age") != "" {
		t.Fatalf("disallowed origin got CORS headers: %v", other.Header)
	}

	e.Validate = true

	// A user's own token works from the browser.
	u := e.Signup()
	me := e.Do(testenv.Req{Method: "GET", Path: "/users/me", Token: u.Token, Header: map[string]string{"Origin": origin}}).Expect(200)
	if me.Header.Get("Access-Control-Allow-Origin") != origin {
		t.Fatalf("simple request allow-origin = %q", me.Header.Get("Access-Control-Allow-Origin"))
	}

	// An API Key with an Origin header is a key in a web page.
	key := e.Post("/api-keys", e.Admin, map[string]any{"name": "backend", "scope": "platform",
		"permissions": []string{"users.list"}}).Expect(201).Str("secret")
	e.Get("/users", key).Expect(200)
	for _, o := range []string{origin, "https://evil.example"} {
		e.Do(testenv.Req{Method: "GET", Path: "/users", Token: key, Header: map[string]string{"Origin": o}}).
			ExpectErr(400, "api_key_in_browser")
	}
}
