package core_test

import (
	"testing"

	"github.com/CompositeCode/substratalapps.com/internal/testenv"
)

// Conventions → Versioning: the version is a request header, not a path
// prefix.
func TestAPIVersionHeader(t *testing.T) {
	e := testenv.New(t)
	u := e.Signup()
	noVersion := map[string]string{"Substratal-Version": ""}

	// Served, and echoed on the response.
	r := e.Get("/users/me", u.Token).Expect(200)
	if got := r.Header.Get("Substratal-Version"); got != "1" {
		t.Fatalf("response Substratal-Version = %q, want 1", got)
	}

	// Missing: rejected before authentication or any parameter check.
	r = e.Do(testenv.Req{Method: "GET", Path: "/users/me", Header: noVersion}).ExpectErr(400, "api_version_required")
	if s, _ := r.JSON()["error"].(map[string]any)["details"].(map[string]any)["supported"].([]any); len(s) != 1 || s[0] != "1" {
		t.Fatalf("details.supported = %s", r.Body)
	}
	e.Do(testenv.Req{Method: "POST", Path: "/users/" + u.ID + "/entitlements", Token: e.Admin, Header: noVersion,
		Body: map[string]any{"application_id": "app_nope"}}).ExpectErr(400, "api_version_required")

	// Unknown, or the old path-style name.
	for _, v := range []string{"2", "v1", "2026-10-05"} {
		e.Do(testenv.Req{Method: "GET", Path: "/users/me", Token: u.Token, Header: map[string]string{"Substratal-Version": v}}).
			ExpectErr(400, "unsupported_api_version")
	}

	// Exempt: uptime checks, the OAuth token endpoint, and well-known.
	e.Do(testenv.Req{Method: "GET", Path: "/health", Header: noVersion}).Expect(200)
	e.Do(testenv.Req{Method: "GET", Path: "/.well-known/jwks.json", Header: noVersion}).Expect(200)

	// The rest are deliberately off-spec, so skip contract validation.
	e.Validate = false
	if code := e.Do(testenv.Req{Method: "POST", Path: "/auth/oauth/token", Header: noVersion,
		Body: map[string]any{"grant_type": "refresh_token", "refresh_token": "nope"}}).Code(); code == "api_version_required" {
		t.Fatal("oauth/token must not require Substratal-Version")
	}

	// A known path with the wrong method is 405, naming the methods it has.
	r = e.Do(testenv.Req{Method: "PUT", Path: "/health"}).ExpectErr(405, "method_not_allowed")
	if r.Header.Get("Allow") != "GET" {
		t.Fatalf("Allow = %q, want GET", r.Header.Get("Allow"))
	}
	r = e.Do(testenv.Req{Method: "PUT", Path: "/users/me", Token: u.Token}).ExpectErr(405, "method_not_allowed")
	if r.Header.Get("Allow") != "GET, PATCH, DELETE" {
		t.Fatalf("Allow = %q, want GET, PATCH, DELETE", r.Header.Get("Allow"))
	}

	// The /v1 prefix is gone, and an unknown path is still a 404.
	e.Get("/v1/users/me", u.Token).ExpectErr(404, "not_found")
	e.Do(testenv.Req{Method: "GET", Path: "/nope", Header: noVersion}).ExpectErr(404, "not_found")
}
