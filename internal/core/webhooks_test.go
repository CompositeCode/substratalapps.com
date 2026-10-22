package core_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/CompositeCode/substratalapps.com/internal/db"
	"github.com/CompositeCode/substratalapps.com/internal/testenv"
)

type receiver struct {
	mu     sync.Mutex
	got    []map[string]any
	sigs   []string
	status int
	srv    *httptest.Server
	raw    [][]byte
}

func newReceiver(t *testing.T) *receiver {
	r := &receiver{status: 200}
	r.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		b, _ := io.ReadAll(req.Body)
		var m map[string]any
		_ = json.Unmarshal(b, &m)
		r.mu.Lock()
		r.got = append(r.got, m)
		r.raw = append(r.raw, b)
		r.sigs = append(r.sigs, req.Header.Get("Substratal-Signature"))
		st := r.status
		r.mu.Unlock()
		w.WriteHeader(st)
	}))
	t.Cleanup(r.srv.Close)
	return r
}

func (r *receiver) types() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []string
	for _, m := range r.got {
		out = append(out, m["type"].(string))
	}
	return out
}

// verify checks the newest delivery's signature as the docs tell receivers to.
func verify(t *testing.T, raw []byte, header string, secrets ...string) bool {
	var ts string
	var sigs []string
	for _, p := range strings.Split(header, ",") {
		k, v, _ := strings.Cut(p, "=")
		if k == "t" {
			ts = v
		} else if k == "v1" {
			sigs = append(sigs, v)
		}
	}
	for _, s := range secrets {
		mac := hmac.New(sha256.New, []byte(s))
		mac.Write([]byte(ts + "."))
		mac.Write(raw)
		want := hex.EncodeToString(mac.Sum(nil))
		for _, got := range sigs {
			if hmac.Equal([]byte(got), []byte(want)) {
				return true
			}
		}
	}
	return false
}

func TestWebhookDelivery(t *testing.T) {
	e := testenv.New(t)
	rcv := newReceiver(t)
	app := e.App(system, nil)
	sub := e.Post("/webhooks", e.Admin, map[string]any{"scope": app, "url": rcv.srv.URL,
		"events": []string{"access.granted", "access.revoked", "role.removed"}}).Expect(201)
	secret := sub.Str("signing_secret")
	if !strings.HasPrefix(secret, "whsec_") {
		t.Fatalf("secret = %s", secret)
	}
	if e.Get("/webhooks/"+sub.Str("id"), e.Admin).Str("signing_secret") != "" {
		t.Fatal("a GET must never return the signing secret")
	}
	e.Post("/webhooks", e.Admin, map[string]any{"scope": app, "url": rcv.srv.URL, "events": []string{"nope"}}).
		ExpectErr(422, "unknown_event_type")

	// A grant fires access.granted to the app's subscription, signed.
	u := e.Signup()
	ent := e.Grant(e.Admin, u.ID, app)
	e.Deliver()
	if got := rcv.types(); len(got) != 1 || got[0] != "access.granted" {
		t.Fatalf("delivered = %v", got)
	}
	if !verify(t, rcv.raw[0], rcv.sigs[0], secret) {
		t.Fatal("signature doesn't verify")
	}
	env := rcv.got[0]
	if env["api_version"] != "2026-10-05" || env["data"].(map[string]any)["reason"] != "entitlement_granted" {
		t.Fatalf("envelope = %v", env)
	}

	// Rotation: both secrets sign for 24h.
	rot := e.Post("/webhooks/"+sub.Str("id")+"/rotate-secret", e.Admin, map[string]any{}).Expect(200)
	e.Patch("/entitlements/"+ent, e.Admin, map[string]any{"status": "disabled", "disabled_reason": "test"}).Expect(200)
	e.Deliver()
	last := len(rcv.raw) - 1
	if rcv.types()[last] != "access.revoked" || !verify(t, rcv.raw[last], rcv.sigs[last], secret) ||
		!verify(t, rcv.raw[last], rcv.sigs[last], rot.Str("signing_secret")) {
		t.Fatalf("rotation signing: %s", rcv.sigs[last])
	}

	// Another app's events never arrive.
	other := e.App(system, nil)
	e.Grant(e.Admin, u.ID, other)
	e.Deliver()
	for _, m := range rcv.got {
		if m["application_id"] == other {
			t.Fatal("received another Application's event")
		}
	}

	// Failure schedules a retry; 410 disables the subscription.
	rcv.mu.Lock()
	rcv.status = 500
	rcv.mu.Unlock()
	e.Patch("/entitlements/"+ent, e.Admin, map[string]any{"status": "active"}).Expect(200)
	e.Deliver()
	dl := e.Get("/webhooks/"+sub.Str("id")+"/deliveries?status=failed", e.Admin).Expect(200).JSON()["data"].([]any)
	if len(dl) == 0 || dl[0].(map[string]any)["next_retry_at"] == nil {
		t.Fatalf("failed delivery should carry next_retry_at: %v", dl)
	}
	// Force the retry due, then answer 410.
	_ = e.DB.Tx(context.Background(), db.Settings{AllModes: true}, func(q db.Querier) error {
		_, err := q.Exec(context.Background(), `update webhook_deliveries set next_retry_at = now() where status = 'pending'`, nil)
		return err
	})
	rcv.mu.Lock()
	rcv.status = 410
	rcv.mu.Unlock()
	e.Deliver()
	if e.Get("/webhooks/"+sub.Str("id"), e.Admin).Str("status") != "disabled" {
		t.Fatal("410 should disable the subscription")
	}
	// A test event still goes to a disabled subscription.
	rcv.mu.Lock()
	rcv.status = 200
	rcv.mu.Unlock()
	n := len(rcv.got)
	e.Post("/webhooks/"+sub.Str("id")+"/test", e.Admin, map[string]any{}).Expect(202)
	e.Deliver()
	if len(rcv.got) != n+1 || rcv.types()[n] != "webhook.test" {
		t.Fatalf("test event = %v", rcv.types())
	}
	e.Patch("/webhooks/"+sub.Str("id"), e.Admin, map[string]any{"status": "unhealthy"}).ExpectErr(409, "invalid_status_transition")
	e.Patch("/webhooks/"+sub.Str("id"), e.Admin, map[string]any{"status": "healthy"}).Expect(200)
	// An owner manages their own Application's subscriptions, with no plan cap.
	owner := e.Signup()
	ownApp := e.App(owner.ID, nil)
	e.Post("/webhooks", owner.Token, map[string]any{"scope": ownApp, "url": rcv.srv.URL, "events": []string{"*"}}).Expect(201)
	e.Post("/webhooks", owner.Token, map[string]any{"scope": ownApp, "url": rcv.srv.URL, "events": []string{"*"}}).Expect(201)
	e.Post("/webhooks", owner.Token, map[string]any{"scope": "platform", "url": rcv.srv.URL, "events": []string{"*"}}).
		ExpectErr(403, "forbidden")
}

func TestUserLifecycle(t *testing.T) {
	e := testenv.New(t)
	u := e.Signup()
	app := e.App(system, nil)
	e.Grant(e.Admin, u.ID, app)
	e.Post("/users/"+u.ID+"/suspend", e.Admin, map[string]any{"reason": "SUP-1"}).Expect(200)
	e.Get("/users/me", u.Token).ExpectErr(401, "session_revoked")
	if accessEventsFor(e, "access.revoked", u.ID, app) != 1 {
		t.Fatal("suspension should fire access.revoked")
	}
	e.Post("/auth/login", "", map[string]any{"email": u.Email, "password": u.Password}).ExpectErr(403, "account_suspended")
	e.Patch("/users/"+u.ID, e.Admin, map[string]any{"status": "active"}).Expect(200)
	if accessEventsFor(e, "access.granted", u.ID, app) != 2 {
		t.Fatal("reactivation should fire access.granted")
	}
	s := e.Login(u.Email, u.Password)
	e.Patch("/users/me", s.Token(), map[string]any{"status": "suspended"}).ExpectErr(403, "status_change_forbidden")
	// Self-service email change is pending until verified.
	newAddr := "changed+" + testenv.Unique() + "@example.com"
	pend := e.Patch("/users/me", s.Token(), map[string]any{"email": newAddr}).Expect(200)
	if pend.Str("email") != u.Email || pend.Str("pending_email") != newAddr {
		t.Fatalf("pending = %s", pend.Body)
	}
	e.Post("/auth/email/verify", "", map[string]any{"token": e.Token(newAddr, "email_verify")}).Expect(204)
	if e.Get("/users/me", s.Token()).Str("email") != newAddr {
		t.Fatal("email change should complete on verify")
	}
	// Export, then soft-delete, then erasure.
	ex := e.Get("/users/me/export", s.Token()).Expect(200).JSON()
	for _, k := range []string{"user", "profile", "settings", "identities", "sessions", "roles", "applications", "audit_events"} {
		if _, ok := ex[k]; !ok {
			t.Fatalf("export missing %s", k)
		}
	}
	e.Post("/users/me/erasure-requests", s.Token(), map[string]any{"reason": "please"}).Expect(202)
	e.Get("/users/"+u.ID, e.Admin).Expect(200) // users.manage still sees it
	other := e.Signup()
	e.Get("/users/"+u.ID, other.Token).ExpectErr(404, "user_not_found")
	e.Patch("/users/"+u.ID, e.Admin, map[string]any{"status": "active"}).ExpectErr(409, "erasure_scheduled")
	// Run the cascade now.
	_ = e.DB.Tx(context.Background(), db.Settings{AllModes: true}, func(q db.Querier) error {
		_, err := q.Exec(context.Background(), `update erasure_requests set scheduled_for = now() where user_id = :u`, db.Args{"u": u.ID})
		return err
	})
	if n, err := e.Core.RunErasures(context.Background()); err != nil || n != 1 {
		t.Fatalf("erasures = %d, %v", n, err)
	}
	gone := e.Get("/users/"+u.ID, e.Admin).Expect(200)
	if !strings.HasPrefix(gone.Str("email"), "erased+") {
		t.Fatalf("erased user = %s", gone.Body)
	}
	ev := e.Get("/audit-events?target_user_id="+u.ID+"&action=entitlement.granted", e.Admin).Expect(200).JSON()["data"].([]any)
	if len(ev) != 1 || ev[0].(map[string]any)["after"] != nil {
		t.Fatalf("snapshots should be redacted, shape kept: %v", ev)
	}
	e.Patch("/users/"+u.ID, e.Admin, map[string]any{"status": "active"}).ExpectErr(404, "user_not_found")
}

func TestAuditVisibility(t *testing.T) {
	e := testenv.New(t)
	owner := e.Signup()
	app := e.App(owner.ID, nil)
	u := e.Signup()
	e.Grant(e.Admin, u.ID, app)
	e.Get("/audit-events", u.Token).ExpectErr(403, "forbidden")
	mine := e.Get("/audit-events?target_user_id=me", u.Token).Expect(200).JSON()["data"].([]any)
	if len(mine) == 0 {
		t.Fatal("a user can read their own trail")
	}
	tenant := e.Get("/applications/"+app, e.Admin).Str("tenant_id")
	own := e.Get("/audit-events?tenant_id="+tenant, owner.Token).Expect(200)
	if len(own.JSON()["data"].([]any)) == 0 || own.Header.Get("Audit-Hot-Window-Start") == "" {
		t.Fatalf("tenant owner view = %s", own.Body)
	}
	e.Get("/audit-events?tenant_id="+tenant, u.Token).ExpectErr(403, "forbidden")
	id := mine[0].(map[string]any)["id"].(string)
	e.Get("/audit-events/"+id, u.Token).Expect(200)
	e.Get("/audit-events/"+id, owner.Token).Expect(200) // the Tenant owner's view of their own app
	stranger := e.Signup()
	e.Get("/audit-events/"+id, stranger.Token).ExpectErr(404, "audit_event_not_found")
}

func TestTenancy(t *testing.T) {
	e := testenv.New(t)
	owner := e.Signup()
	e.App(owner.ID, nil)
	ts := e.Get("/tenants", owner.Token).Expect(200).JSON()["data"].([]any)
	if len(ts) != 1 {
		t.Fatalf("owner tenants = %v", ts)
	}
	tenant := ts[0].(map[string]any)
	tid := tenant["id"].(string)
	if tenant["tier"] != "shared" || tenant["application_count"].(float64) != 1 {
		t.Fatalf("tenant = %v", tenant)
	}
	if _, ok := tenant["plan"]; ok {
		t.Fatalf("Tenants aren't billed and carry no plan: %v", tenant)
	}
	u := e.Signup()
	e.Get("/tenants/"+tid, u.Token).ExpectErr(404, "tenant_not_found")
	// Tier changes: tenants.manage only, up the ladder only. No plan gates them.
	e.Post("/tenants/"+tid+"/tier-change-requests", owner.Token, map[string]any{"requested_tier": "isolated", "reason": "x"}).
		ExpectErr(403, "forbidden")
	e.Post("/tenants/"+tid+"/tier-change-requests", e.Admin, map[string]any{"requested_tier": "isolated", "reason": "x"}).
		Expect(201)
	sys := "tnt_01JAG1SYSTEM00000000000000"
	e.Post("/tenants/"+sys+"/tier-change-requests", e.Admin, map[string]any{"requested_tier": "dedicated_region",
		"requested_region": "mars-1", "reason": "x"}).ExpectErr(422, "unsupported_region")
	req := e.Post("/tenants/"+sys+"/tier-change-requests", e.Admin, map[string]any{"requested_tier": "isolated", "reason": "contract"}).Expect(201)
	rp := "/tenants/" + sys + "/tier-change-requests/" + req.Str("id")
	e.Post("/tenants/"+sys+"/tier-change-requests", e.Admin, map[string]any{"requested_tier": "isolated", "reason": "again"}).
		ExpectErr(409, "tier_change_already_pending")
	e.Patch(rp, e.Admin, map[string]any{"status": "completed"}).ExpectErr(409, "invalid_request_transition")
	e.Patch(rp, e.Admin, map[string]any{"status": "in_progress"}).Expect(200)
	if e.Get("/tenants/"+sys, e.Admin).Str("status") != "migrating" {
		t.Fatal("in_progress should mark the Tenant migrating")
	}
	// Writes to a migrating Tenant's data are paused.
	e.Post("/applications", e.Admin, map[string]any{"slug": "mig-" + testenv.Unique(), "name": "x",
		"launch_url": "https://x.example.com", "owner_user_id": system}).ExpectErr(503, "service_unavailable")
	e.Patch(rp, e.Admin, map[string]any{"status": "cancelled"}).ExpectErr(422, "validation_failed")
	e.Patch(rp, e.Admin, map[string]any{"status": "cancelled", "notes": "rollback"}).Expect(200)
	if e.Get("/tenants/"+sys, e.Admin).Str("status") != "active" {
		t.Fatal("cancel should restore the Tenant")
	}
}

func TestPagination(t *testing.T) {
	e := testenv.New(t)
	for i := 0; i < 5; i++ {
		e.Signup()
	}
	seen := map[string]bool{}
	cursor := ""
	pages := 0
	for {
		path := "/users?limit=2"
		if cursor != "" {
			path += "&cursor=" + cursor
		}
		page := e.Get(path, e.Admin).Expect(200).JSON()
		for _, u := range page["data"].([]any) {
			id := u.(map[string]any)["id"].(string)
			if seen[id] {
				t.Fatalf("duplicate %s across pages", id)
			}
			seen[id] = true
		}
		pages++
		p := page["page"].(map[string]any)
		if p["has_more"] != true {
			if p["next_cursor"] != nil {
				t.Fatal("next_cursor must be null when has_more is false")
			}
			break
		}
		cursor = p["next_cursor"].(string)
	}
	if len(seen) < 6 || pages < 3 {
		t.Fatalf("seen %d users across %d pages", len(seen), pages)
	}
	e.Get("/users?limit=2&cursor=garbage", e.Admin).ExpectErr(400, "invalid_cursor")
	first := e.Get("/users?limit=1", e.Admin).JSON()["page"].(map[string]any)["next_cursor"].(string)
	e.Get("/users?limit=1&status=active&cursor="+first, e.Admin).ExpectErr(400, "invalid_cursor")
}

func TestRateLimits(t *testing.T) {
	e := testenv.NewWith(t, testenv.Options{RateLimits: true})
	var last *testenv.Resp
	for i := 0; i < 11; i++ {
		last = e.Post("/auth/login", "", map[string]any{"email": "rl+" + testenv.Unique() + "@example.com", "password": "whatever passphrase"})
	}
	last.ExpectErr(429, "rate_limited")
	if last.Header.Get("Retry-After") == "" || last.Header.Get("RateLimit-Limit") != "10" {
		t.Fatalf("headers = %v", last.Header)
	}
	ok := e.Get("/users/me", e.Admin).Expect(200)
	if ok.Header.Get("RateLimit-Remaining") == "" {
		t.Fatal("every response carries RateLimit headers")
	}
}

func TestSweepExpiresAndStarts(t *testing.T) {
	e := testenv.New(t)
	u := e.Signup()
	app := e.App(system, nil)
	ends := time.Now().Add(time.Second).UTC().Format(time.RFC3339Nano)
	id := e.PostIdem("/users/"+u.ID+"/entitlements", e.Admin, testenv.Unique(),
		map[string]any{"application_id": app, "source": "trial", "ends_at": ends}).Expect(201).Str("id")
	time.Sleep(1200 * time.Millisecond)
	if n, _, err := e.Core.SweepEntitlements(context.Background()); err != nil || n != 1 {
		t.Fatalf("expired = %d, %v", n, err)
	}
	if e.Get("/entitlements/"+id, e.Admin).Str("status") != "expired" {
		t.Fatal("sweep should expire the trial")
	}
	if accessEventsFor(e, "access.revoked", u.ID, app) != 1 || len(e.Events("entitlement.expired")) != 1 {
		t.Fatal("expiry should fire entitlement.expired and access.revoked")
	}
	// Renewal needs a future ends_at.
	e.Patch("/entitlements/"+id, e.Admin, map[string]any{"status": "active"}).ExpectErr(409, "invalid_status_transition")
	e.Patch("/entitlements/"+id, e.Admin, map[string]any{"status": "active",
		"ends_at": time.Now().Add(240 * time.Hour).UTC().Format(time.RFC3339)}).Expect(200)
	if err := e.Core.Cleanup(context.Background()); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
}
