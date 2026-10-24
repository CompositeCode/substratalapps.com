package core_test

import (
	"context"
	"net/url"
	"testing"

	"github.com/CompositeCode/substratalapps.com/internal/db"
	"github.com/CompositeCode/substratalapps.com/internal/testenv"
)

// pageTotal returns page.total and page.total_capped, failing if either is
// missing.
func pageTotal(t *testing.T, r *testenv.Resp) (int, bool) {
	t.Helper()
	page := r.JSON()["page"].(map[string]any)
	n, ok := page["total"].(float64)
	capped, ok2 := page["total_capped"].(bool)
	if !ok || !ok2 {
		t.Fatalf("page has no total: %v", page)
	}
	return int(n), capped
}

func expectTotal(t *testing.T, r *testenv.Resp, want int) {
	t.Helper()
	if n, capped := pageTotal(t, r); n != want || capped {
		t.Fatalf("total = %d (capped %v), want %d", n, capped, want)
	}
}

// ?include=total counts the rows matching the filters, on every page.
func TestListTotals(t *testing.T) {
	e := testenv.New(t)
	rcv := newReceiver(t)
	owner := e.Signup()
	app := e.App(owner.ID, nil)
	e.Post("/webhooks", e.Admin, map[string]any{"scope": app, "url": rcv.srv.URL, "events": []string{"access.granted"}}).Expect(201)
	hook := e.Post("/webhooks", e.Admin, map[string]any{"scope": app, "url": rcv.srv.URL, "events": []string{"*"}}).Expect(201).Str("id")
	a, b, c := e.Signup(), e.Signup(), e.Signup()
	e.Grant(e.Admin, a.ID, app)
	e.Grant(e.Admin, b.ID, app)
	e.Grant(e.Admin, c.ID, app)
	e.Deliver()

	// Users: counts match the filter, and every page reports the same total.
	first := e.Get("/users?application_id="+app+"&limit=2&include=total", e.Admin).Expect(200)
	expectTotal(t, first, 3)
	if len(first.JSON()["data"].([]any)) != 2 {
		t.Fatal("limit still applies")
	}
	cur := url.QueryEscape(first.JSON()["page"].(map[string]any)["next_cursor"].(string))
	expectTotal(t, e.Get("/users?application_id="+app+"&limit=2&include=total&cursor="+cur, e.Admin).Expect(200), 3)

	// Omitted unless requested, and a cursor works either way.
	plain := e.Get("/users?application_id="+app+"&limit=2&cursor="+cur, e.Admin).Expect(200)
	page := plain.JSON()["page"].(map[string]any)
	if _, ok := page["total"]; ok {
		t.Fatalf("total without include: %v", page)
	}
	if _, ok := page["total_capped"]; ok {
		t.Fatalf("total_capped without include: %v", page)
	}
	if len(plain.JSON()["data"].([]any)) != 1 {
		t.Fatal("second page has the third user")
	}
	e.Get("/users?application_id="+app+"&include=everything", e.Admin).ExpectErr(422, "validation_failed")

	// Filters narrow the count.
	expectTotal(t, e.Get("/users?application_id="+app+"&email="+url.QueryEscape(a.Email)+"&include=total", e.Admin).Expect(200), 1)
	expectTotal(t, e.Get("/entitlements?application_id="+app+"&include=total", e.Admin).Expect(200), 3)
	expectTotal(t, e.Get("/entitlements?application_id="+app+"&status=revoked&include=total", e.Admin).Expect(200), 0)
	expectTotal(t, e.Get("/users/"+a.ID+"/entitlements?include=total", e.Admin).Expect(200), 1)
	expectTotal(t, e.Get("/applications?owner_user_id="+owner.ID+"&include=total", e.Admin).Expect(200), 1)
	expectTotal(t, e.Get("/webhooks?application_id="+app+"&include=total", e.Admin).Expect(200), 2)
	deliveries := e.Get("/webhooks/"+hook+"/deliveries?limit=100&include=total", e.Admin).Expect(200)
	expectTotal(t, deliveries, len(deliveries.JSON()["data"].([]any)))
	e.Post("/api-keys", e.Admin, map[string]any{"name": "backend", "scope": app, "permissions": []string{"users.list"}}).Expect(201)
	expectTotal(t, e.Get("/api-keys?scope="+app+"&include=total", e.Admin).Expect(200), 1)
	roles := e.Get("/roles?scope="+app+"&limit=100&include=total", e.Admin).Expect(200)
	expectTotal(t, roles, len(roles.JSON()["data"].([]any)))
	audit := e.Get("/audit-events?target_user_id="+a.ID+"&limit=100&include=total", e.Admin).Expect(200)
	expectTotal(t, audit, len(audit.JSON()["data"].([]any)))

	// Organizations and members, as an org admin who isn't staff.
	org := e.Post("/organizations", owner.Token, map[string]any{"name": "Count Co " + testenv.Unique()}).Expect(201).Str("id")
	e.Post("/organizations/"+org+"/members", owner.Token, map[string]any{"email": a.Email, "role": "member"}).Expect(201)
	expectTotal(t, e.Get("/organizations?include=total", owner.Token).Expect(200), 1)
	expectTotal(t, e.Get("/organizations/"+org+"/members?include=total", owner.Token).Expect(200), 2)
	expectTotal(t, e.Get("/organizations/"+org+"/members?role=org_admin&include=total", owner.Token).Expect(200), 1)

	// A caller who can see nothing gets a total of 0.
	key := e.Post("/api-keys", e.Admin, map[string]any{"name": "lister", "scope": "platform", "permissions": []string{"users.list"}}).Expect(201).Str("secret")
	expectTotal(t, e.Get("/organizations?include=total", key).Expect(200), 0)
	expectTotal(t, e.Get("/users?status=deleted&include=total", key).Expect(200), 0)
}

// Above 10,000 matching rows the count stops and says so.
func TestListTotalCap(t *testing.T) {
	e := testenv.New(t)
	u := e.Signup()
	err := e.DB.Tx(context.Background(), db.Settings{}, func(q db.Querier) error {
		_, err := q.Exec(context.Background(), `insert into audit_events (id, action, actor_type, actor_id, target_type, target_id, target_user_id)
			select 'aud_cap' || g || :u, 'profile.updated', 'system', 'system', 'user', :u, :u from generate_series(1, 10050) g`, db.Args{"u": u.ID})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	r := e.Get("/audit-events?target_user_id="+u.ID+"&include=total", e.Admin).Expect(200)
	if n, capped := pageTotal(t, r); n != 10000 || !capped {
		t.Fatalf("total = %d (capped %v), want 10000 capped", n, capped)
	}
}
