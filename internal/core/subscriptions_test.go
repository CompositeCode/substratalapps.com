package core_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/CompositeCode/substratalapps.com/internal/core"
	"github.com/CompositeCode/substratalapps.com/internal/testenv"
)

// fakeStripe stands in for the billing package: it records Checkout
// requests and returns placeholder URLs.
type fakeStripe struct {
	mu        sync.Mutex
	checkouts []core.CheckoutInput
	live      bool
}

func (f *fakeStripe) EnsureCustomer(_ context.Context, who core.BillingSubscriber, _ string) (string, error) {
	return "cus_" + who.ID, nil
}

func (f *fakeStripe) CheckoutSession(_ context.Context, in core.CheckoutInput) (string, time.Time, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.checkouts = append(f.checkouts, in)
	return "https://checkout.stripe.test/" + in.SubscriptionID, time.Now().Add(time.Hour), nil
}

func (f *fakeStripe) PortalSession(_ context.Context, customerID, _ string) (string, time.Time, error) {
	return "https://billing.stripe.test/" + customerID, time.Now().Add(5 * time.Minute), nil
}

func (f *fakeStripe) LiveMode() bool { return f.live }

func (f *fakeStripe) last() core.CheckoutInput {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.checkouts[len(f.checkouts)-1]
}

var checkoutBody = map[string]any{"plan": "standard", "success_url": "https://app.example.com/ok", "cancel_url": "https://app.example.com/no"}

func allowed(e *testenv.Env, userID, appID string) bool {
	e.T.Helper()
	return e.Get("/users/"+userID+"/apps/"+appID+"/effective-permissions", e.Admin).Expect(200).JSON()["allowed"] == true
}

// activate plays Stripe's part after a completed Checkout.
func activate(t *testing.T, e *testenv.Env, subscriptionID, status string) {
	t.Helper()
	if _, err := e.Core.ApplyStripeSubscription(context.Background(), core.StripeSubscription{
		ID: "sub_" + subscriptionID, SubscriptionID: subscriptionID, Status: status}); err != nil {
		t.Fatalf("apply %s: %v", status, err)
	}
}

func TestFreeAndStandardAccess(t *testing.T) {
	stripe := &fakeStripe{}
	e := testenv.NewWith(t, testenv.Options{Billing: stripe, OwnDatabase: true})
	before := e.Signup()
	free := e.App(system, map[string]any{"access_tier": "free"})
	standard := e.App(system, nil)
	if tier := e.Get("/applications/"+standard, e.Admin).Str("access_tier"); tier != "standard" {
		t.Fatalf("default access_tier = %s", tier)
	}

	// Free: every User, existing and new, gets each free Application.
	after := e.Signup()
	for _, u := range []testenv.User{before, after} {
		if !allowed(e, u.ID, free) || allowed(e, u.ID, standard) {
			t.Fatalf("%s: free=%v standard=%v", u.ID, allowed(e, u.ID, free), allowed(e, u.ID, standard))
		}
	}
	page := e.Get("/applications?access_tier=free", e.Admin).JSON()["data"].([]any)
	for _, a := range page {
		if a.(map[string]any)["access_tier"] != "free" {
			t.Fatalf("access_tier filter returned %v", a)
		}
	}
	me := e.Get("/users/me/subscription", after.Token).Expect(200).JSON()
	if me["plan"] != "free" || me["subscription"] != nil || len(me["plan_sources"].([]any)) != 0 {
		t.Fatalf("free user subscription = %v", me)
	}

	// The platform owns free rows: an admin can only disable or re-enable.
	ents := e.Get("/users/"+after.ID+"/entitlements?application_id="+free, e.Admin).JSON()["data"].([]any)
	if len(ents) != 1 || ents[0].(map[string]any)["source"] != "free" {
		t.Fatalf("free entitlements = %v", ents)
	}
	ep := "/entitlements/" + ents[0].(map[string]any)["id"].(string)
	e.Patch(ep, e.Admin, map[string]any{"ends_at": "2030-01-01T00:00:00Z"}).ExpectErr(409, "entitlement_platform_managed")
	e.Patch(ep, e.Admin, map[string]any{"status": "revoked", "disabled_reason": "x"}).ExpectErr(409, "entitlement_platform_managed")
	e.Delete(ep, e.Admin).ExpectErr(409, "entitlement_platform_managed")
	e.Patch(ep, e.Admin, map[string]any{"status": "disabled", "disabled_reason": "abuse"}).Expect(200)
	if allowed(e, after.ID, free) {
		t.Fatal("a disabled free row should turn access off")
	}
	e.Patch(ep, e.Admin, map[string]any{"status": "active"}).Expect(200)
	e.PostIdem("/users/"+after.ID+"/entitlements", e.Admin, testenv.Unique(),
		map[string]any{"application_id": free, "source": "free"}).ExpectErr(422, "invalid_source")

	// Standard, individually.
	e.PostIdem("/users/me/billing/checkout-sessions", after.Token, testenv.Unique(),
		map[string]any{"plan": "business", "success_url": "https://x.example.com", "cancel_url": "https://x.example.com"}).
		ExpectErr(422, "plan_requires_sales")
	e.Post("/users/me/billing/portal-sessions", after.Token, map[string]any{"return_url": "https://x.example.com"}).
		ExpectErr(409, "no_billing_account")
	co := e.PostIdem("/users/me/billing/checkout-sessions", after.Token, testenv.Unique(), checkoutBody).Expect(201)
	if co.Str("url") == "" || stripe.last().Quantity != 1 || stripe.last().Subscriber.Type != "user" {
		t.Fatalf("checkout = %s, %+v", co.Body, stripe.last())
	}
	pending := e.Get("/users/me/subscription", after.Token).Expect(200).JSON()
	sub := pending["subscription"].(map[string]any)
	if pending["plan"] != "free" || sub["status"] != "incomplete" || sub["unit_amount"].(float64) != 600 {
		t.Fatalf("pending subscription = %v", pending)
	}
	sid := sub["id"].(string)
	activate(t, e, sid, "active")
	if !allowed(e, after.ID, standard) || !allowed(e, after.ID, free) {
		t.Fatal("Standard should unlock standard Applications and keep free ones")
	}
	u := e.Get("/users/"+after.ID, e.Admin).JSON()
	if u["plan"] != "standard" || len(u["plan_sources"].([]any)) != 1 {
		t.Fatalf("user = %v", u)
	}
	if accessEventsFor(e, "access.granted", after.ID, standard) != 1 {
		t.Fatal("subscribing should fire access.granted")
	}
	e.PostIdem("/users/me/billing/checkout-sessions", after.Token, testenv.Unique(), checkoutBody).
		ExpectErr(409, "subscription_exists")
	e.Post("/users/me/billing/portal-sessions", after.Token, map[string]any{"return_url": "https://x.example.com"}).Expect(201)
	e.Get("/users/"+after.ID+"/subscription", before.Token).ExpectErr(403, "forbidden")

	// A developer grant coexists with the subscription row.
	e.Grant(e.Admin, before.ID, standard)
	e.Grant(e.Admin, after.ID, standard)

	// Lapse: Standard access ends, Free stays, the developer grant stays.
	activate(t, e, sid, "canceled")
	if !allowed(e, after.ID, standard) {
		t.Fatal("the developer grant should keep access after the subscription ends")
	}
	other := e.Signup()
	e.PostIdem("/users/me/billing/checkout-sessions", other.Token, testenv.Unique(), checkoutBody).Expect(201)
	osid := e.Get("/users/me/subscription", other.Token).JSON()["subscription"].(map[string]any)["id"].(string)
	activate(t, e, osid, "active")
	activate(t, e, osid, "past_due")
	if !allowed(e, other.ID, standard) {
		t.Fatal("past_due still covers")
	}
	activate(t, e, osid, "unpaid")
	if allowed(e, other.ID, standard) || !allowed(e, other.ID, free) {
		t.Fatal("an ended subscription drops to Free")
	}
	if accessEventsFor(e, "access.revoked", other.ID, standard) != 1 {
		t.Fatal("lapsing should fire access.revoked")
	}
	if e.Get("/users/me", other.Token).Str("plan") != "free" {
		t.Fatal("plan after lapse")
	}
	// Resubscribing (a new Stripe subscription) revives the same row.
	e.PostIdem("/users/me/billing/checkout-sessions", other.Token, testenv.Unique(), checkoutBody).Expect(201)
	if _, err := e.Core.ApplyStripeSubscription(context.Background(), core.StripeSubscription{
		ID: "sub_new_" + osid, SubscriptionID: osid, Status: "active"}); err != nil {
		t.Fatal(err)
	}
	if !allowed(e, other.ID, standard) {
		t.Fatal("resubscribing restores Standard")
	}
	// A late event about the old, canceled Stripe subscription changes nothing.
	activate(t, e, osid, "canceled")
	if !allowed(e, other.ID, standard) {
		t.Fatal("a stale event from the replaced subscription must not end coverage")
	}
	live := e.Get("/users/"+other.ID+"/entitlements?application_id="+standard+"&source=subscription", e.Admin).JSON()["data"].([]any)
	if len(live) != 1 {
		t.Fatalf("one live subscription row, got %v", live)
	}
}

func TestAccessTierChanges(t *testing.T) {
	e := testenv.NewWith(t, testenv.Options{OwnDatabase: true})
	owner := e.Signup()
	u := e.Signup()
	app := e.App(owner.ID, nil)
	if allowed(e, u.ID, app) {
		t.Fatal("standard by default")
	}
	e.Patch("/applications/"+app, owner.Token, map[string]any{"access_tier": "free"}).ExpectErr(403, "moderation_field_forbidden")
	e.Patch("/applications/"+app, e.Admin, map[string]any{"access_tier": "gold"}).ExpectErr(422, "validation_failed")
	e.Patch("/applications/"+app, e.Admin, map[string]any{"access_tier": "free"}).Expect(200)
	if !allowed(e, u.ID, app) || !allowed(e, owner.ID, app) {
		t.Fatal("making an Application free grants it to every User")
	}
	e.Patch("/applications/"+app, e.Admin, map[string]any{"access_tier": "standard"}).Expect(200)
	if allowed(e, u.ID, app) {
		t.Fatal("moving back to standard ends free access")
	}
	// The hourly job is a no-op when everything already matches.
	if err := e.Core.ReconcileAllAccess(context.Background()); err != nil {
		t.Fatal(err)
	}
	if allowed(e, u.ID, app) {
		t.Fatal("reconcile changed nothing")
	}
}

func TestOrganizationSubscription(t *testing.T) {
	stripe := &fakeStripe{}
	e := testenv.NewWith(t, testenv.Options{Billing: stripe, OwnDatabase: true})
	standard := e.App(system, nil)
	admin := e.Signup()
	oid := e.Post("/organizations", admin.Token, map[string]any{"name": "Acme " + testenv.Unique()}).Expect(201).Str("id")
	join := func() testenv.User {
		m := e.Signup()
		e.Post("/organizations/"+oid+"/members", admin.Token, map[string]any{"email": m.Email}).Expect(201)
		e.Post("/organizations/"+oid+"/members/me/accept", m.Token, map[string]any{}).Expect(200)
		return m
	}
	m1, m2 := join(), join()
	stranger := e.Signup()

	sp := "/organizations/" + oid + "/subscription"
	if body := e.Get(sp, admin.Token).Expect(200).JSON(); body["plan"] != "free" || body["subscription"] != nil {
		t.Fatalf("org before = %v", body)
	}
	e.Get(sp, m1.Token).ExpectErr(403, "forbidden")
	e.Get(sp, stranger.Token).ExpectErr(404, "organization_not_found")
	e.Patch(sp, admin.Token, map[string]any{"member_scope": "all_members"}).ExpectErr(404, "subscription_not_found")
	co := "/organizations/" + oid + "/billing/checkout-sessions"
	e.PostIdem(co, m1.Token, testenv.Unique(), checkoutBody).ExpectErr(403, "forbidden")

	body := map[string]any{"member_scope": "allowlist", "member_overrides": []string{m1.ID}}
	for k, v := range checkoutBody {
		body[k] = v
	}
	e.PostIdem(co, admin.Token, testenv.Unique(), body).Expect(201)
	if stripe.last().Quantity != 1 || stripe.last().Subscriber.Type != "organization" {
		t.Fatalf("org checkout = %+v", stripe.last())
	}
	got := e.Get(sp, admin.Token).Expect(200).JSON()["subscription"].(map[string]any)
	activate(t, e, got["id"].(string), "trialing")
	if !allowed(e, m1.ID, standard) || allowed(e, m2.ID, standard) || allowed(e, admin.ID, standard) {
		t.Fatal("the allowlist decides who's covered")
	}
	src := e.Get("/users/me", m1.Token).JSON()["plan_sources"].([]any)
	if len(src) != 1 || src[0].(map[string]any)["type"] != "organization" || src[0].(map[string]any)["id"] != oid {
		t.Fatalf("plan_sources = %v", src)
	}

	// Covering everyone: three members are billed.
	res := e.Patch(sp, admin.Token, map[string]any{"member_scope": "all_members", "member_overrides": nil}).Expect(200).JSON()
	if res["plan"] != "standard" || res["subscription"].(map[string]any)["quantity"].(map[string]any)["current"].(float64) != 3 {
		t.Fatalf("org after = %v", res)
	}
	if !allowed(e, m2.ID, standard) || !allowed(e, admin.ID, standard) {
		t.Fatal("all_members covers every active member")
	}
	qs, err := e.Core.OrganizationQuantities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, q := range qs {
		found = found || (q.SubscriptionID == got["id"] && q.Quantity == 3)
	}
	if !found {
		t.Fatalf("quantities = %+v", qs)
	}

	// Leaving the Organization ends that member's Standard access.
	e.Delete("/organizations/"+oid+"/members/"+m2.ID, admin.Token).Expect(204)
	if allowed(e, m2.ID, standard) {
		t.Fatal("a former member isn't covered")
	}
	activate(t, e, got["id"].(string), "canceled")
	if allowed(e, m1.ID, standard) {
		t.Fatal("a canceled Organization subscription drops its members to Free")
	}
}

// A deployment billing in Stripe live mode can't start a test-mode
// subscription; one on a Stripe test key can.
func TestTestModeCheckoutNeedsStripeTestMode(t *testing.T) {
	signupTest := func(e *testenv.Env) string {
		addr := "tm+" + testenv.Unique() + "@example.com"
		return e.Post("/auth/signup", "", map[string]any{"email": addr, "password": "a long enough test passphrase",
			"test_mode": true}).Expect(201).Token()
	}
	live := testenv.NewWith(t, testenv.Options{Billing: &fakeStripe{live: true}})
	live.PostIdem("/users/me/billing/checkout-sessions", signupTest(live), testenv.Unique(), checkoutBody).
		ExpectErr(409, "test_mode_billing_unavailable")
	live.PostIdem("/users/me/billing/checkout-sessions", live.Signup().Token, testenv.Unique(), checkoutBody).Expect(201)

	test := testenv.NewWith(t, testenv.Options{Billing: &fakeStripe{}})
	tok := signupTest(test)
	test.PostIdem("/users/me/billing/checkout-sessions", tok, testenv.Unique(), checkoutBody).Expect(201)
	if sub := test.Get("/users/me/subscription", tok).JSON()["subscription"].(map[string]any); sub["test_mode"] != true {
		t.Fatalf("test-mode subscription = %v", sub)
	}
}
