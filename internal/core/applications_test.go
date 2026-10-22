package core_test

import (
	"testing"

	"github.com/CompositeCode/substratalapps.com/internal/testenv"
)

// verifiedUser signs up, verifies the email, and signs in again so the
// access token carries email_verified.
func verifiedUser(e *testenv.Env) testenv.User {
	e.T.Helper()
	u := e.Signup()
	e.Post("/v1/auth/email/verify", "", map[string]any{"token": e.Token(u.Email, "email_verify")}).Expect(204)
	u.Token = e.Login(u.Email, u.Password).Token()
	return u
}

func appBody(slug string, extra map[string]any) map[string]any {
	b := map[string]any{"slug": slug, "name": "Dev " + slug, "launch_url": "https://" + slug + ".example.com/launch"}
	for k, v := range extra {
		b[k] = v
	}
	return b
}

// Applications → The review lifecycle: self-service registration into
// the review queue.
func TestSelfServiceApplicationRegistration(t *testing.T) {
	e := testenv.New(t)
	slug := func() string { return "dev-" + testenv.Unique() }

	// Unverified Users and keys without applications.manage can't submit.
	unverified := e.Signup()
	e.Post("/v1/applications", unverified.Token, appBody(slug(), nil)).ExpectErr(403, "email_not_verified")
	key := e.Post("/v1/api-keys", e.Admin, map[string]any{"name": "ops", "scope": "platform",
		"permissions": []string{"users.list"}}).Expect(201).Str("secret")
	e.Post("/v1/applications", key, appBody(slug(), nil)).ExpectErr(403, "forbidden")

	dev := verifiedUser(e)
	other := verifiedUser(e)
	// Moderation fields and someone else's ownership are staff-only.
	for _, extra := range []map[string]any{
		{"visibility": "internal"}, {"access_tier": "free"}, {"review_status": "approved"}, {"review_notes": "x"},
		{"owner_user_id": other.ID}, {"receives_platform_notifications": true},
	} {
		e.Post("/v1/applications", dev.Token, appBody(slug(), extra)).ExpectErr(403, "moderation_field_forbidden")
	}

	// A submission is the caller's own, pending review, on a new Tenant.
	s := slug()
	app := e.Post("/v1/applications", dev.Token, appBody(s, map[string]any{
		"available_app_roles": []string{"member"}, "default_app_role": "member"})).Expect(201).JSON()
	appID := app["id"].(string)
	if app["review_status"] != "pending_review" || app["owner_user_id"] != dev.ID || app["visibility"] != "public" ||
		app["access_tier"] != "standard" || app["tenant_id"] == nil || app["self_registered"] != true {
		t.Fatalf("submission = %v", app)
	}
	if e.Get("/v1/tenants/"+app["tenant_id"].(string), dev.Token).Expect(200).Str("owner_user_id") != dev.ID {
		t.Fatal("the submitter should own the new Tenant")
	}

	// It's in the staff queue, hidden from everyone else, and can't be
	// launched or granted yet.
	queue := e.Get("/v1/applications?review_status=pending_review", e.Admin).Expect(200).JSON()["data"].([]any)
	found := false
	for _, a := range queue {
		found = found || a.(map[string]any)["id"] == appID
	}
	if !found {
		t.Fatal("submission missing from the review queue")
	}
	e.Get("/v1/applications/"+appID, dev.Token).Expect(200)
	e.Get("/v1/applications/"+appID, other.Token).ExpectErr(404, "application_not_found")
	e.PostIdem("/v1/users/"+dev.ID+"/entitlements", dev.Token, testenv.Unique(),
		map[string]any{"application_id": appID, "source": "admin_grant"}).ExpectErr(409, "application_not_available")

	// Rejected, then resubmitted by editing, then approved.
	e.Patch("/v1/applications/"+appID, e.Admin, map[string]any{"review_status": "rejected", "review_notes": "no icon"}).Expect(200)
	if rs := e.Patch("/v1/applications/"+appID, dev.Token, map[string]any{"icon_url": "https://" + s + ".example.com/icon.png"}).
		Expect(200).Str("review_status"); rs != "pending_review" {
		t.Fatalf("after resubmit review_status = %s", rs)
	}
	e.Patch("/v1/applications/"+appID, e.Admin, map[string]any{"review_status": "approved"}).Expect(200)
	e.Get("/v1/applications/"+appID, other.Token).Expect(200)
	e.Grant(dev.Token, other.ID, appID)

	// Even approved, a self-registered Application is third-party: no
	// embedded app tokens, and no platform security alerts.
	e.Post("/v1/auth/app-tokens", other.Token, map[string]any{"application_id": appID}).ExpectErr(403, "embedded_login_not_allowed")
	e.Patch("/v1/applications/"+appID, dev.Token, map[string]any{"receives_platform_notifications": true}).
		ExpectErr(403, "moderation_field_forbidden")
	e.Patch("/v1/applications/"+appID, e.Admin, map[string]any{"receives_platform_notifications": true}).Expect(200)
	e.Patch("/v1/applications/"+appID, e.Admin, map[string]any{"self_registered": false}).ExpectErr(422, "read_only_field")

	// An org_admin can register one for their Organization; a member of
	// someone else's can't.
	org := e.Post("/v1/organizations", dev.Token, map[string]any{"name": "Dev Co " + testenv.Unique()}).Expect(201).Str("id")
	orgApp := e.Post("/v1/applications", dev.Token, appBody(slug(), map[string]any{"owner_organization_id": org})).Expect(201).JSON()
	if orgApp["owner_organization_id"] != org || orgApp["owner_user_id"] != nil {
		t.Fatalf("org submission = %v", orgApp)
	}
	e.Post("/v1/applications", other.Token, appBody(slug(), map[string]any{"owner_organization_id": org})).
		ExpectErr(403, "moderation_field_forbidden")

	// Staff creation is unchanged: approved by default, any owner.
	staff := e.Post("/v1/applications", e.Admin, appBody(slug(), map[string]any{"owner_user_id": other.ID})).Expect(201).JSON()
	if staff["review_status"] != "approved" || staff["self_registered"] != false {
		t.Fatalf("staff create review_status = %v", staff["review_status"])
	}
}

// Platform-managed Entitlements only exist for an Application that has
// been approved, so a submission in review never reaches anyone's account.
func TestPlatformAccessWaitsForApproval(t *testing.T) {
	e := testenv.NewWith(t, testenv.Options{OwnDatabase: true})
	u := e.Signup()
	app := e.App(system, map[string]any{"access_tier": "free", "review_status": "pending_review"})
	if n := len(e.Get("/v1/users/"+u.ID+"/entitlements?application_id="+app, e.Admin).JSON()["data"].([]any)); n != 0 {
		t.Fatalf("a pending free Application granted %d Entitlements", n)
	}
	e.Patch("/v1/applications/"+app, e.Admin, map[string]any{"review_status": "approved"}).Expect(200)
	if !allowed(e, u.ID, app) {
		t.Fatal("approval should grant the free Application")
	}
	if n := accessEventsFor(e, "access.granted", u.ID, app); n != 1 {
		t.Fatalf("access.granted fired %d times", n)
	}
	// Suspension doesn't cascade.
	e.Patch("/v1/applications/"+app, e.Admin, map[string]any{"review_status": "suspended", "review_notes": "x"}).Expect(200)
	if n := len(e.Get("/v1/users/"+u.ID+"/entitlements?application_id="+app, e.Admin).JSON()["data"].([]any)); n != 1 {
		t.Fatalf("suspension left %d Entitlements", n)
	}
}
