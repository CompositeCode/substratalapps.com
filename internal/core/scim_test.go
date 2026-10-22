package core_test

import (
	"net/url"
	"strings"
	"testing"

	"github.com/CompositeCode/substratalapps.com/internal/testenv"
)

// scimCall sends a SCIM request with the Organization's token and checks
// the status. Like an identity provider, it sends no Substratal-Version
// (Conventions → Versioning exempts /scim/v2).
func scimCall(t *testing.T, e *testenv.Env, method, path, token string, body any, status int) map[string]any {
	t.Helper()
	r := e.Do(testenv.Req{Method: method, Path: "/scim/v2" + path, Token: token, Body: body,
		Header: map[string]string{"Substratal-Version": ""}})
	if r.Status != status {
		t.Fatalf("%s %s: got %d, want %d: %s", method, path, r.Status, status, r.Body)
	}
	if status == 204 {
		return nil
	}
	if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/scim+json") {
		t.Fatalf("%s %s: Content-Type %q", method, path, ct)
	}
	return r.JSON()
}

func scimMembership(u map[string]any) map[string]any {
	return u["urn:substratal:params:scim:schemas:extension:2.0:Membership"].(map[string]any)
}

// API Reference → SCIM.
func TestSCIMProvisioning(t *testing.T) {
	e := testenv.New(t)
	admin := e.Signup()
	oid := e.Post("/organizations", admin.Token, map[string]any{"name": "Acme " + testenv.Unique()}).Expect(201).Str("id")
	app := e.App(system, nil)
	e.PostIdem("/organizations/"+oid+"/entitlements", e.Admin, testenv.Unique(), map[string]any{"application_id": app}).Expect(201)

	// Token management: org admins only; the secret is shown once.
	e.Get("/organizations/"+oid+"/scim-token", admin.Token).ExpectErr(404, "scim_token_not_found")
	tok := e.Post("/organizations/"+oid+"/scim-token", admin.Token, nil).Expect(201).JSON()
	secret := tok["token"].(string)
	if !strings.HasPrefix(secret, "scim_") || tok["base_url"] != "http://localhost/scim/v2" {
		t.Fatalf("token = %v", tok)
	}
	if shown := e.Get("/organizations/"+oid+"/scim-token", admin.Token).Expect(200).JSON(); shown["token"] != nil {
		t.Fatal("GET must never show the secret")
	}

	// Without a valid token, SCIM errors in SCIM's own shape.
	if r := scimCall(t, e, "GET", "/Users", "scim_nope", nil, 401); r["schemas"].([]any)[0] != "urn:ietf:params:scim:api:messages:2.0:Error" {
		t.Fatalf("error = %v", r)
	}
	scimCall(t, e, "GET", "/ServiceProviderConfig", secret, nil, 200)
	scimCall(t, e, "GET", "/Groups", secret, nil, 404)

	// Provisioning invites: the membership is pending until the person
	// accepts, whether or not they already had an account.
	existing := e.Signup()
	u := scimCall(t, e, "POST", "/Users", secret, map[string]any{
		"schemas": []string{"urn:ietf:params:scim:schemas:core:2.0:User"}, "userName": existing.Email, "active": true}, 201)
	if u["id"] != existing.ID || u["active"] != true || scimMembership(u)["status"] != "pending" {
		t.Fatalf("provisioned = %v", u)
	}
	if allowed(e, existing.ID, app) {
		t.Fatal("a pending membership must grant nothing")
	}
	scimCall(t, e, "POST", "/Users", secret, map[string]any{"userName": existing.Email}, 409)
	newAddr := "scim+" + testenv.Unique() + "@example.com"
	fresh := scimCall(t, e, "POST", "/Users", secret, map[string]any{"userName": "jdoe",
		"emails": []map[string]any{{"value": newAddr, "primary": true}}}, 201)
	if _, ok := e.Mail.Last(newAddr, "invitation"); !ok {
		t.Fatal("a new address should get an account invitation")
	}
	audit := e.Get("/audit-events?action=organization.member_invited&target_user_id="+existing.ID, e.Admin).Expect(200).JSON()["data"].([]any)
	if actor := audit[0].(map[string]any)["actor"].(map[string]any); actor["type"] != "system" || actor["id"] != tok["id"] {
		t.Fatalf("SCIM writes should be attributed to the token: %v", actor)
	}

	// Filtering and paging.
	list := scimCall(t, e, "GET", "/Users?filter="+url.QueryEscape(`userName eq "`+existing.Email+`"`), secret, nil, 200)
	if list["totalResults"].(float64) != 1 {
		t.Fatalf("filtered list = %v", list)
	}
	all := scimCall(t, e, "GET", "/Users?startIndex=1&count=1", secret, nil, 200)
	if all["totalResults"].(float64) != 3 || len(all["Resources"].([]any)) != 1 {
		t.Fatalf("paged list = %v", all)
	}
	scimCall(t, e, "GET", "/Users?filter=title%20eq%20%22x%22", secret, nil, 400)

	// Once accepted, the member has the Organization's access.
	e.Post("/organizations/"+oid+"/members/me/accept", existing.Token, map[string]any{}).Expect(200)
	if got := scimCall(t, e, "GET", "/Users/"+existing.ID, secret, nil, 200); scimMembership(got)["status"] != "active" {
		t.Fatalf("after accept = %v", got)
	}
	if !allowed(e, existing.ID, app) {
		t.Fatal("an active member should have the org grant")
	}

	// Deactivating removes the membership, and access with it; the
	// person's own account is untouched.
	patched := scimCall(t, e, "PATCH", "/Users/"+existing.ID, secret, map[string]any{
		"schemas":    []string{"urn:ietf:params:scim:api:messages:2.0:PatchOp"},
		"Operations": []map[string]any{{"op": "Replace", "path": "active", "value": "False"}}}, 200)
	if patched["active"] != false {
		t.Fatalf("patched = %v", patched)
	}
	if allowed(e, existing.ID, app) || accessEventsFor(e, "access.revoked", existing.ID, app) != 1 {
		t.Fatal("deprovisioning should revoke the org's access")
	}
	scimCall(t, e, "GET", "/Users/"+existing.ID, secret, nil, 404)
	e.Get("/users/me", existing.Token).Expect(200)

	// DELETE removes a pending invitation; the last org_admin can't be removed.
	scimCall(t, e, "DELETE", "/Users/"+fresh["id"].(string), secret, nil, 204)
	if r := scimCall(t, e, "DELETE", "/Users/"+admin.ID, secret, nil, 409); r["scimType"] != "mutability" {
		t.Fatalf("last admin = %v", r)
	}

	// Only org admins manage the token; revoking it cuts SCIM off at once.
	member := e.Signup()
	e.Post("/organizations/"+oid+"/members", admin.Token, map[string]any{"email": member.Email}).Expect(201)
	e.Post("/organizations/"+oid+"/members/me/accept", member.Token, map[string]any{}).Expect(200)
	e.Post("/organizations/"+oid+"/scim-token", member.Token, nil).ExpectErr(403, "forbidden")
	e.Delete("/organizations/"+oid+"/scim-token", admin.Token).Expect(204)
	scimCall(t, e, "GET", "/Users", secret, nil, 401)
}
