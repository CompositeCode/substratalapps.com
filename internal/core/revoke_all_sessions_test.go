package core_test

import (
	"testing"

	"github.com/CompositeCode/substratalapps.com/internal/testenv"
)

// Auth → Sessions: POST /users/{id}/sessions/revoke-all signs a User out
// everywhere.
func TestRevokeAllSessions(t *testing.T) {
	e := testenv.New(t)
	u := e.Signup()
	second := e.Login(u.Email, u.Password)
	third := e.Login(u.Email, u.Password)

	// Self, keeping the calling session.
	r := e.Post("/users/me/sessions/revoke-all", u.Token, map[string]any{"keep_current": true}).Expect(200).JSON()
	if r["revoked"] != float64(2) {
		t.Fatalf("revoked = %v", r["revoked"])
	}
	e.Get("/users/me", u.Token).Expect(200)
	e.Get("/users/me", second.Token()).Expect(401)
	e.Post("/auth/token/refresh", "", map[string]any{"refresh_token": third.Str("refresh_token")}).Expect(401)

	// Someone else can't, without users.manage.
	other := e.Signup()
	e.Post("/users/"+u.ID+"/sessions/revoke-all", other.Token, nil).ExpectErr(403, "forbidden")

	// An admin can, but keep_current is self-only.
	e.Post("/users/"+u.ID+"/sessions/revoke-all", e.Admin, map[string]any{"keep_current": true}).ExpectErr(422, "validation_failed")
	r = e.Post("/users/"+u.ID+"/sessions/revoke-all", e.Admin, nil).Expect(200).JSON()
	if r["revoked"] != float64(1) {
		t.Fatalf("admin revoked = %v", r["revoked"])
	}
	e.Get("/users/me", u.Token).Expect(401)
	e.Post("/auth/token/refresh", "", map[string]any{"refresh_token": u.Refresh}).Expect(401)

	// Nothing left to revoke is still a success.
	r = e.Post("/users/"+u.ID+"/sessions/revoke-all", e.Admin, nil).Expect(200).JSON()
	if r["revoked"] != float64(0) {
		t.Fatalf("second revoked = %v", r["revoked"])
	}
	e.Post("/users/usr_01JAG0NOSUCHUSER0000000000/sessions/revoke-all", e.Admin, nil).ExpectErr(404, "user_not_found")

	// Each call is audited with its count.
	ev := e.Get("/audit-events?action=user.sessions_revoked&target_id="+u.ID, e.Admin).Expect(200).JSON()["data"].([]any)
	if len(ev) != 3 {
		t.Fatalf("audit events = %v", ev)
	}

	// Destructive: an agent key with restrict_destructive can't call it.
	agent := e.Post("/api-keys", e.Admin, map[string]any{"name": "agent", "scope": "platform", "intended_use": "agent",
		"permissions": []string{"users.manage"}}).Expect(201).Str("secret")
	e.Post("/users/"+other.ID+"/sessions/revoke-all", agent, nil).Expect(403)
}
