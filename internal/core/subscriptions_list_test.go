package core_test

import (
	"context"
	"testing"
	"time"

	"github.com/CompositeCode/substratalapps.com/internal/core"
	"github.com/CompositeCode/substratalapps.com/internal/testenv"
)

func subscriptionIDs(t *testing.T, r *testenv.Resp) []string {
	t.Helper()
	var out []string
	for _, x := range r.JSON()["data"].([]any) {
		out = append(out, x.(map[string]any)["id"].(string))
	}
	return out
}

func sameIDs(got []string, want ...string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// GET /subscriptions lists every subscriber's subscription for billing.manage.
func TestListSubscriptions(t *testing.T) {
	stripe := &fakeStripe{}
	e := testenv.NewWith(t, testenv.Options{Billing: stripe, OwnDatabase: true})

	// Three subscribers: two Users and an Organization.
	alice, bob, owner := e.Signup(), e.Signup(), e.Signup()
	orgName := "Acme " + testenv.Unique()
	oid := e.Post("/organizations", owner.Token, map[string]any{"name": orgName}).Expect(201).Str("id")
	member := e.Signup()
	e.Post("/organizations/"+oid+"/members", owner.Token, map[string]any{"email": member.Email}).Expect(201)
	e.Post("/organizations/"+oid+"/members/me/accept", member.Token, map[string]any{}).Expect(200)

	subOf := func(path, token string) string {
		return e.Get(path, token).Expect(200).JSON()["subscription"].(map[string]any)["id"].(string)
	}
	e.PostIdem("/users/me/billing/checkout-sessions", alice.Token, testenv.Unique(), checkoutBody).Expect(201)
	aliceSub := subOf("/users/me/subscription", alice.Token)
	e.PostIdem("/users/me/billing/checkout-sessions", bob.Token, testenv.Unique(), checkoutBody).Expect(201)
	bobSub := subOf("/users/me/subscription", bob.Token)
	e.PostIdem("/organizations/"+oid+"/billing/checkout-sessions", owner.Token, testenv.Unique(), checkoutBody).Expect(201)
	orgSub := subOf("/organizations/"+oid+"/subscription", owner.Token)

	soon := time.Now().Add(72 * time.Hour).UTC()
	later := time.Now().Add(40 * 24 * time.Hour).UTC()
	apply := func(id, status string, end time.Time, cancel bool) {
		t.Helper()
		if _, err := e.Core.ApplyStripeSubscription(context.Background(), core.StripeSubscription{
			ID: "sub_" + id, SubscriptionID: id, Status: status, PeriodEnd: &end, CancelAtPeriodEnd: cancel}); err != nil {
			t.Fatalf("apply %s: %v", status, err)
		}
	}
	// Updated in this order, so the newest-first list is org, bob, alice.
	apply(aliceSub, "past_due", soon, false)
	apply(bobSub, "active", later, true)
	apply(orgSub, "past_due", later, false)

	// Permission: billing.manage only. Subscribers, org owners, and members get 403.
	for _, tok := range []string{alice.Token, owner.Token, member.Token} {
		e.Get("/subscriptions", tok).ExpectErr(403, "forbidden")
	}
	billing := e.Signup()
	e.Post("/users/"+billing.ID+"/roles/role_platform_billing_admin", e.Admin, map[string]any{}).Expect(201)

	all := e.Get("/subscriptions", billing.Token).Expect(200)
	if got := subscriptionIDs(t, all); !sameIDs(got, orgSub, bobSub, aliceSub) {
		t.Fatalf("all = %v", got)
	}
	// Each item is the Subscription object plus the subscriber for display.
	for _, x := range all.JSON()["data"].([]any) {
		item := x.(map[string]any)
		switch item["id"] {
		case aliceSub:
			if item["subscriber_type"] != "user" || item["user_email"] != alice.Email || item["organization_name"] != nil {
				t.Fatalf("alice item = %v", item)
			}
		case orgSub:
			if item["subscriber_type"] != "organization" || item["organization_name"] != orgName || item["user_email"] != nil ||
				item["quantity"].(map[string]any)["current"].(float64) != 2 {
				t.Fatalf("org item = %v", item)
			}
		}
	}

	// Filters.
	if got := subscriptionIDs(t, e.Get("/subscriptions?status=past_due", billing.Token).Expect(200)); !sameIDs(got, orgSub, aliceSub) {
		t.Fatalf("past_due = %v", got)
	}
	if got := subscriptionIDs(t, e.Get("/subscriptions?status=past_due&status=active", billing.Token).Expect(200)); !sameIDs(got, orgSub, bobSub, aliceSub) {
		t.Fatalf("past_due or active = %v", got)
	}
	if got := subscriptionIDs(t, e.Get("/subscriptions?status=canceled", billing.Token).Expect(200)); len(got) != 0 {
		t.Fatalf("canceled = %v", got)
	}
	if got := subscriptionIDs(t, e.Get("/subscriptions?subscriber_type=organization", billing.Token).Expect(200)); !sameIDs(got, orgSub) {
		t.Fatalf("organization = %v", got)
	}
	if got := subscriptionIDs(t, e.Get("/subscriptions?subscriber_type=user&status=past_due", billing.Token).Expect(200)); !sameIDs(got, aliceSub) {
		t.Fatalf("user past_due = %v", got)
	}
	if got := subscriptionIDs(t, e.Get("/subscriptions?cancel_at_period_end=true", billing.Token).Expect(200)); !sameIDs(got, bobSub) {
		t.Fatalf("cancel_at_period_end = %v", got)
	}
	if got := subscriptionIDs(t, e.Get("/subscriptions?cancel_at_period_end=false", billing.Token).Expect(200)); !sameIDs(got, orgSub, aliceSub) {
		t.Fatalf("not cancelling = %v", got)
	}
	before := time.Now().Add(7 * 24 * time.Hour).UTC().Format(time.RFC3339)
	if got := subscriptionIDs(t, e.Get("/subscriptions?current_period_end_before="+before, billing.Token).Expect(200)); !sameIDs(got, aliceSub) {
		t.Fatalf("ending soon = %v", got)
	}
	e.Get("/subscriptions?status=bogus", billing.Token).ExpectErr(422, "validation_failed")

	// Cursor pagination, newest update first.
	p1 := e.Get("/subscriptions?limit=2", billing.Token).Expect(200)
	if got := subscriptionIDs(t, p1); !sameIDs(got, orgSub, bobSub) {
		t.Fatalf("page 1 = %v", got)
	}
	page := p1.JSON()["page"].(map[string]any)
	if page["has_more"] != true {
		t.Fatalf("page 1 info = %v", page)
	}
	p2 := e.Get("/subscriptions?limit=2&cursor="+page["next_cursor"].(string), billing.Token).Expect(200)
	if got := subscriptionIDs(t, p2); !sameIDs(got, aliceSub) || p2.JSON()["page"].(map[string]any)["has_more"] != false {
		t.Fatalf("page 2 = %v", p2.Body)
	}
	e.Get("/subscriptions?subscriber_type=bogus", billing.Token).ExpectErr(422, "validation_failed")

	// ?include=total counts every match, on every page.
	expectTotal(t, e.Get("/subscriptions?include=total", billing.Token).Expect(200), 3)
	expectTotal(t, e.Get("/subscriptions?include=total&status=past_due", billing.Token).Expect(200), 2)
	expectTotal(t, e.Get("/subscriptions?include=total&limit=2&cursor="+page["next_cursor"].(string), billing.Token).Expect(200), 3)
	nine := "/subscriptions?status=active"
	for range 8 {
		nine += "&status=active"
	}
	e.Get(nine, billing.Token).ExpectErr(422, "validation_failed")

	// An app-scoped key never lists subscriptions.
	app := e.App(system, nil)
	appKey := e.Post("/api-keys", e.Admin, map[string]any{"name": "backend", "scope": app,
		"permissions": []string{"users.list"}}).Expect(201).Str("secret")
	e.Get("/subscriptions", appKey).ExpectErr(403, "forbidden")

	// A deleted User's subscription is listed without their email, unless
	// the caller also holds users.manage.
	e.Delete("/users/"+alice.ID, e.Admin).Expect(204)
	emailOf := func(token string) any {
		for _, x := range e.Get("/subscriptions?subscriber_type=user", token).Expect(200).JSON()["data"].([]any) {
			if item := x.(map[string]any); item["id"] == aliceSub {
				return item["user_email"]
			}
		}
		t.Fatal("deleted User's subscription isn't listed")
		return nil
	}
	if got := emailOf(billing.Token); got != nil {
		t.Fatalf("billing.manage sees a deleted User's email: %v", got)
	}
	if got := emailOf(e.Admin); got != alice.Email {
		t.Fatalf("users.manage email = %v", got)
	}

	// A cursor is bound to its filters.
	e.Get("/subscriptions?limit=2&status=active&cursor="+page["next_cursor"].(string), billing.Token).ExpectErr(400, "invalid_cursor")
}
