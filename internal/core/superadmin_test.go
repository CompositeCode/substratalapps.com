package core_test

import (
	"context"
	"testing"

	"github.com/CompositeCode/substratalapps.com/internal/db"
	"github.com/CompositeCode/substratalapps.com/internal/testenv"
)

// The protected superadmin (migration 0004) can't be locked out through the
// API or directly in the database, and every deploy re-asserts it.
func TestProtectedSuperadmin(t *testing.T) {
	e := testenv.NewWith(t, testenv.Options{OwnDatabase: true})
	ctx := context.Background()
	addr := "owner+" + testenv.Unique() + "@example.com"

	id, created, err := e.Core.EnsureProtectedSuperadmin(ctx, addr)
	if err != nil || !created {
		t.Fatalf("first ensure: %s %v %v", id, created, err)
	}
	again, created, err := e.Core.EnsureProtectedSuperadmin(ctx, addr)
	if err != nil || created || again != id {
		t.Fatalf("second ensure should be a no-op: %s %v %v", again, created, err)
	}
	roles := e.Get("/users/"+id+"/roles", e.Admin).Expect(200).JSON()["data"].([]any)
	hasSuper := false
	for _, r := range roles {
		hasSuper = hasSuper || r.(map[string]any)["role_id"] == "role_platform_superadmin"
	}
	if !hasSuper {
		t.Fatalf("roles = %v", roles)
	}
	if u := e.Get("/users/"+id, e.Admin).JSON(); u["status"] != "active" || u["email_verified"] != true {
		t.Fatalf("user = %v", u)
	}

	// Every API path that could lock the account out is refused.
	up := "/users/" + id
	e.Delete(up, e.Admin).ExpectErr(409, "user_protected")
	e.Post(up+"/suspend", e.Admin, map[string]any{"reason": "x"}).ExpectErr(409, "user_protected")
	e.Patch(up, e.Admin, map[string]any{"status": "suspended"}).ExpectErr(409, "user_protected")
	e.Patch(up, e.Admin, map[string]any{"email": "elsewhere+" + testenv.Unique() + "@example.com"}).ExpectErr(409, "user_protected")
	e.Post(up+"/erasure-requests", e.Admin, map[string]any{}).ExpectErr(409, "user_protected")
	e.Delete(up+"/roles/role_platform_superadmin", e.Admin).ExpectErr(409, "user_protected")
	// Harmless changes still work.
	e.Patch(up, e.Admin, map[string]any{"status": "active"}).Expect(200)

	// The database refuses the same changes from any caller.
	for _, q := range []string{
		`update users set status = 'suspended' where id = :u`,
		`update users set deleted_at = now() where id = :u`,
		`update users set protected = false where id = :u`,
		`delete from user_role_assignments where user_id = :u and role_id = 'role_platform_superadmin'`,
		`insert into erasure_requests (user_id, requested_by_type, requested_by_id, scheduled_for)
			values (:u, 'system', 'system', now())`,
	} {
		err := e.DB.Tx(ctx, db.Settings{}, func(tx db.Querier) error {
			_, err := tx.Exec(ctx, q, db.Args{"u": id})
			return err
		})
		if err == nil {
			t.Fatalf("database allowed: %s", q)
		}
	}
	// Ops can lift the guard for one transaction to hand ownership over.
	err = e.DB.Tx(ctx, db.Settings{}, func(tx db.Querier) error {
		if _, err := tx.Query(ctx, `select to_jsonb(set_config('app.protected_user_maintenance', 'on', true))`, nil); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `update users set protected = false where id = :u`, db.Args{"u": id})
		return err
	})
	if err != nil {
		t.Fatalf("maintenance override: %v", err)
	}
	e.Delete(up+"/roles/role_platform_superadmin", e.Admin).Expect(204)

	// The next deploy's ensure restores the role and the protection.
	if _, _, err := e.Core.EnsureProtectedSuperadmin(ctx, addr); err != nil {
		t.Fatal(err)
	}
	e.Delete(up+"/roles/role_platform_superadmin", e.Admin).ExpectErr(409, "user_protected")
}
