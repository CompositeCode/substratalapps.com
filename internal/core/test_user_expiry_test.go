package core_test

import (
	"context"
	"testing"
	"time"

	"github.com/CompositeCode/substratalapps.com/internal/db"
	"github.com/CompositeCode/substratalapps.com/internal/testenv"
)

const testUserPass = "a long enough test passphrase"

// fieldCode is the code of a 422's first field error.
func fieldCode(t *testing.T, r *testenv.Resp) string {
	t.Helper()
	fields := r.JSON()["error"].(map[string]any)["details"].(map[string]any)["fields"].([]any)
	return fields[0].(map[string]any)["code"].(string)
}

// setExpiry moves a User's expires_at directly, so a test can put it in
// the past without waiting.
func setExpiry(t *testing.T, e *testenv.Env, userID string, at time.Time) {
	t.Helper()
	err := e.DB.Tx(context.Background(), db.Settings{AllModes: true}, func(q db.Querier) error {
		_, err := q.Exec(context.Background(), `update users set expires_at = :t::timestamptz where id = :u`,
			db.Args{"u": userID, "t": db.Time(at)})
		return err
	})
	if err != nil {
		t.Fatalf("set expiry: %v", err)
	}
}

// userEvents is the outbox events of a type about one User.
func userEvents(e *testenv.Env, eventType, userID string) []map[string]any {
	var out []map[string]any
	for _, ev := range e.Events(eventType) {
		if ev["payload"].(map[string]any)["user_id"] == userID {
			out = append(out, ev)
		}
	}
	return out
}

func expiresAt(t *testing.T, u map[string]any) time.Time {
	t.Helper()
	s, ok := u["expires_at"].(string)
	if !ok {
		t.Fatalf("expires_at = %v", u["expires_at"])
	}
	at, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		t.Fatalf("expires_at: %v", err)
	}
	return at
}

// Auth → Test-mode Users → Lifetime: creation defaults and range checks.
func TestTestUserExpiryCreation(t *testing.T) {
	e := testenv.New(t)
	testKey := e.Post("/v1/api-keys", e.Admin, map[string]any{"name": "qa", "scope": "platform", "mode": "test",
		"permissions": []string{"users.manage", "users.list"}}).Expect(201).Str("secret")
	liveKey := e.Post("/v1/api-keys", e.Admin, map[string]any{"name": "live", "scope": "platform",
		"permissions": []string{"users.manage", "users.list"}}).Expect(201).Str("secret")
	addr := func() string { return "qa+" + testenv.Unique() + "@example.com" }

	// Default: 24 hours, through signup and through POST /v1/users.
	s := e.Post("/v1/auth/signup", "", map[string]any{"email": addr(), "password": testUserPass, "test_mode": true}).Expect(201)
	if d := time.Until(expiresAt(t, e.Get("/v1/users/me", s.Token()).Expect(200).JSON())); d < 23*time.Hour || d > 24*time.Hour {
		t.Fatalf("signup default lifetime = %v", d)
	}
	u := e.Post("/v1/users", testKey, map[string]any{"email": addr(), "status": "active", "send_invitation": false}).Expect(201)
	if d := time.Until(expiresAt(t, u.JSON())); d < 23*time.Hour || d > 24*time.Hour {
		t.Fatalf("create default lifetime = %v", d)
	}
	// Live Users never expire.
	if live := e.Signup(); e.Get("/v1/users/me", live.Token).JSON()["expires_at"] != nil {
		t.Fatal("a live User has expires_at")
	}

	// A chosen lifetime is kept.
	want := time.Now().Add(2 * time.Hour).UTC().Truncate(time.Second)
	u = e.Post("/v1/users", testKey, map[string]any{"email": addr(), "status": "active", "send_invitation": false,
		"expires_at": want.Format(time.RFC3339)}).Expect(201)
	if got := expiresAt(t, u.JSON()); !got.Equal(want) {
		t.Fatalf("expires_at = %v, want %v", got, want)
	}

	// Out of range, either way, and on a live User.
	soon := time.Now().Add(5 * time.Minute).Format(time.RFC3339)
	late := time.Now().Add(31 * 24 * time.Hour).Format(time.RFC3339)
	r := e.Post("/v1/auth/signup", "", map[string]any{"email": addr(), "password": testUserPass, "test_mode": true,
		"expires_at": soon}).ExpectErr(422, "validation_failed")
	if c := fieldCode(t, r); c != "expires_at_out_of_range" {
		t.Fatalf("too soon: %s", c)
	}
	r = e.Post("/v1/users", testKey, map[string]any{"email": addr(), "expires_at": late}).ExpectErr(422, "validation_failed")
	if c := fieldCode(t, r); c != "expires_at_out_of_range" {
		t.Fatalf("too late: %s", c)
	}
	r = e.Post("/v1/users", liveKey, map[string]any{"email": addr(), "expires_at": want.Format(time.RFC3339)}).
		ExpectErr(422, "validation_failed")
	if c := fieldCode(t, r); c != "live_user" {
		t.Fatalf("live create: %s", c)
	}
	r = e.Post("/v1/auth/signup", "", map[string]any{"email": addr(), "password": testUserPass,
		"expires_at": want.Format(time.RFC3339)}).ExpectErr(422, "validation_failed")
	if c := fieldCode(t, r); c != "live_user" {
		t.Fatalf("live signup: %s", c)
	}
}

// Expiry: tokens capped and then rejected, the sweep's effects exactly
// once, and extension bringing the User back.
func TestTestUserExpiry(t *testing.T) {
	e := testenv.New(t)
	ctx := context.Background()
	app := e.App("usr_01JAG0SYSTEM00000000000000", nil)
	testKey := e.Post("/v1/api-keys", e.Admin, map[string]any{"name": "qa", "scope": "platform", "mode": "test",
		"permissions": []string{"users.manage", "users.list", "entitlements.manage"}}).Expect(201).Str("secret")
	addr := "qa+" + testenv.Unique() + "@example.com"
	s := e.Post("/v1/auth/signup", "", map[string]any{"email": addr, "password": testUserPass, "test_mode": true}).Expect(201)
	uid := s.Str("user_id")
	e.PostIdem("/v1/users/"+uid+"/entitlements", testKey, testenv.Unique(),
		map[string]any{"application_id": app, "source": "admin_grant"}).Expect(201)
	e.Post("/v1/auth/app-tokens", s.Token(), map[string]any{"application_id": app}).Expect(200)

	// Close to expiry, no token outlives the User.
	near := time.Now().Add(2 * time.Minute).UTC().Truncate(time.Second)
	setExpiry(t, e, uid, near)
	r := e.Post("/v1/auth/token/refresh", "", map[string]any{"refresh_token": s.Str("refresh_token")}).Expect(200)
	if in := int(r.JSON()["expires_in"].(float64)); in > 120 {
		t.Fatalf("access expires_in = %d", in)
	}
	at, refresh := r.Token(), r.Str("refresh_token")
	appTok := e.Post("/v1/auth/app-tokens", at, map[string]any{"application_id": app}).Expect(200)
	if exp := int64(decodeJWT(t, e, appTok.Str("app_token"))["exp"].(float64)); exp > near.Unix() {
		t.Fatalf("app token exp %d is after expires_at %d", exp, near.Unix())
	}

	// At expiry every token stops working, before any sweep.
	setExpiry(t, e, uid, time.Now())
	e.Get("/v1/users/me", at).ExpectErr(401, "session_revoked")
	e.Post("/v1/auth/login", "", map[string]any{"email": addr, "password": testUserPass, "test_mode": true}).
		ExpectErr(403, "test_user_expired")
	e.Post("/v1/auth/token/refresh", "", map[string]any{"refresh_token": refresh}).ExpectErr(403, "test_user_expired")
	if eff := e.Get("/v1/users/"+uid+"/apps/"+app+"/effective-permissions", testKey).Expect(200).JSON(); eff["allowed"] != false {
		t.Fatalf("introspection after expiry = %v", eff)
	}

	// The sweep's effects happen exactly once.
	for range 2 {
		if _, err := e.Core.ExpireTestUsers(ctx); err != nil {
			t.Fatalf("sweep: %v", err)
		}
	}
	revoked := userEvents(e, "access.revoked", uid)
	if len(revoked) != 1 || revoked[0]["payload"].(map[string]any)["reason"] != "test_user_expired" {
		t.Fatalf("access.revoked = %v", revoked)
	}
	if ev := userEvents(e, "user.expired", uid); len(ev) != 1 || ev[0]["test_mode"] != true {
		t.Fatalf("user.expired = %v", ev)
	}
	var state struct {
		Live    int `json:"live_sessions"`
		Expired int `json:"expired_sessions"`
		Audit   int `json:"audit"`
	}
	err := e.DB.Tx(ctx, db.Settings{AllModes: true}, func(q db.Querier) error {
		return db.One(ctx, q, &state, `select jsonb_build_object(
			'live_sessions', (select count(*) from sessions where user_id = :u and revoked_at is null),
			'expired_sessions', (select count(*) from sessions where user_id = :u and revoked_reason = 'test_user_expired'),
			'audit', (select count(*) from audit_events where target_id = :u and action = 'user.expired' and actor_type = 'system'))`,
			db.Args{"u": uid})
	})
	if err != nil || state.Live != 0 || state.Expired == 0 || state.Audit != 1 {
		t.Fatalf("after sweep: %+v %v", state, err)
	}

	// A self-service caller can't move expires_at; users.manage can, and
	// moving it later brings the User back.
	later := time.Now().Add(time.Hour).UTC().Truncate(time.Second).Format(time.RFC3339)
	u := e.Patch("/v1/users/"+uid, testKey, map[string]any{"expires_at": later}).Expect(200).JSON()
	if expiresAt(t, u).Format(time.RFC3339) != later {
		t.Fatalf("expires_at = %v", u["expires_at"])
	}
	granted := userEvents(e, "access.granted", uid)
	if len(granted) == 0 || granted[0]["payload"].(map[string]any)["reason"] != "test_user_extended" {
		t.Fatalf("access.granted = %v", granted)
	}
	back := e.Post("/v1/auth/login", "", map[string]any{"email": addr, "password": testUserPass, "test_mode": true}).Expect(200)
	e.Post("/v1/auth/app-tokens", back.Token(), map[string]any{"application_id": app}).Expect(200)
	e.Patch("/v1/users/me", back.Token(), map[string]any{"expires_at": later}).ExpectErr(403, "status_change_forbidden")

	// The sweep leaves an extended User alone.
	if _, err := e.Core.ExpireTestUsers(ctx); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if n := len(userEvents(e, "user.expired", uid)); n != 1 {
		t.Fatalf("user.expired fired %d times", n)
	}
}

// Migration 0005's backfill: existing test Users expire when the 30-day
// purge would have removed them, and live Users never do.
func TestTestUserExpiryBackfill(t *testing.T) {
	d := testenv.DatabaseThrough(t, "0004")
	ctx := context.Background()
	created := time.Now().Add(-48 * time.Hour).UTC().Truncate(time.Second)
	err := d.Tx(ctx, db.Settings{AllModes: true}, func(q db.Querier) error {
		_, err := q.Exec(ctx, `insert into users (id, email, status, test_mode, created_at) values
			('usr_backfill_test', 'backfill-test@example.com', 'active', true, :c::timestamptz),
			('usr_backfill_live', 'backfill-live@example.com', 'active', false, :c::timestamptz)`,
			db.Args{"c": db.Time(created)})
		return err
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	testenv.MigrateRest(t, d)
	var got map[string]*time.Time
	err = d.Tx(ctx, db.Settings{AllModes: true}, func(q db.Querier) error {
		return db.One(ctx, q, &got, `select jsonb_object_agg(id, expires_at) from users
			where id in ('usr_backfill_test', 'usr_backfill_live')`, nil)
	})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if want := created.Add(30 * 24 * time.Hour); got["usr_backfill_test"] == nil || !got["usr_backfill_test"].Equal(want) {
		t.Fatalf("test User expires_at = %v, want %v", got["usr_backfill_test"], want)
	}
	if got["usr_backfill_live"] != nil {
		t.Fatalf("live User expires_at = %v", got["usr_backfill_live"])
	}
}
