package core_test

import (
	"testing"

	"github.com/CompositeCode/substratalapps.com/internal/testenv"
)

func userIDs(t *testing.T, r *testenv.Resp) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	for _, x := range r.JSON()["data"].([]any) {
		out[x.(map[string]any)["id"].(string)] = true
	}
	return out
}

// GET /users?role_id= lists explicit holders of any of the Roles.
func TestUsersRoleFilter(t *testing.T) {
	e := testenv.New(t)
	app := e.App(system, nil)
	other := e.App(system, nil)
	appAdmin := "role_" + app[len("app_"):] + "_admin"
	otherAdmin := "role_" + other[len("app_"):] + "_admin"

	support, appUser, plain := e.Signup(), e.Signup(), e.Signup()
	e.Post("/users/"+support.ID+"/roles/role_platform_support", e.Admin, map[string]any{}).Expect(201)
	e.Post("/users/"+appUser.ID+"/roles/"+appAdmin, e.Admin, map[string]any{}).Expect(201)

	got := userIDs(t, e.Get("/users?limit=100&role_id=role_platform_support", e.Admin).Expect(200))
	if !got[support.ID] || got[appUser.ID] || got[plain.ID] {
		t.Fatalf("support filter = %v", got)
	}
	got = userIDs(t, e.Get("/users?limit=100&role_id="+appAdmin, e.Admin).Expect(200))
	if !got[appUser.ID] || got[support.ID] {
		t.Fatalf("app role filter = %v", got)
	}
	// Repeatable, any-of.
	got = userIDs(t, e.Get("/users?limit=100&role_id=role_platform_support&role_id="+appAdmin, e.Admin).Expect(200))
	if !got[support.ID] || !got[appUser.ID] || got[plain.ID] {
		t.Fatalf("any-of filter = %v", got)
	}

	// An app-confined key may ask only about its own Application's Roles.
	key := e.Post("/api-keys", e.Admin, map[string]any{"name": "backend", "scope": app,
		"permissions": []string{"users.list", "entitlements.manage"}}).Expect(201).Str("secret")
	e.Grant(key, appUser.ID, app)
	got = userIDs(t, e.Get("/users?role_id="+appAdmin, key).Expect(200))
	if !got[appUser.ID] {
		t.Fatalf("app key filter = %v", got)
	}
	e.Get("/users?role_id="+otherAdmin, key).ExpectErr(403, "role_scope_mismatch")
	e.Get("/users?role_id=role_platform_support", key).ExpectErr(403, "role_scope_mismatch")
	e.Get("/users?role_id=role_nope", key).ExpectErr(403, "role_scope_mismatch")
}
