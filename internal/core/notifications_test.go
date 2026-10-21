package core_test

import (
	"context"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"strings"
	"testing"

	"github.com/CompositeCode/substratalapps.com/internal/db"
	"github.com/CompositeCode/substratalapps.com/internal/push"
	"github.com/CompositeCode/substratalapps.com/internal/testenv"
)

func p8Key(t *testing.T) string {
	t.Helper()
	k, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	der, _ := x509.MarshalPKCS8PrivateKey(k)
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
}

func serviceAccount(t *testing.T) map[string]any {
	t.Helper()
	k, _ := rsa.GenerateKey(rand.Reader, 2048)
	der, _ := x509.MarshalPKCS8PrivateKey(k)
	return map[string]any{"type": "service_account", "project_id": "timetrack-test", "private_key_id": "k1",
		"private_key":  string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})),
		"client_email": "push@timetrack-test.iam.gserviceaccount.com"}
}

func apnsToken() string { return strings.Repeat("ab", 32) }

// pushApp is an Application owned by a signed-up User, with an APNs
// credential, an app key holding notifications.send, and a second User
// with access to it.
type pushApp struct {
	ID       string
	Owner    testenv.User
	Key      string
	Member   testenv.User
	DeviceID string
}

func newPushApp(t *testing.T, e *testenv.Env) pushApp {
	t.Helper()
	owner := e.Signup()
	app := e.App(owner.ID, nil)
	e.Post("/v1/applications/"+app+"/push-credentials", owner.Token, map[string]any{"provider": "apns",
		"team_id": "ABCDE12345", "key_id": "XYZ987WVU6", "topic": "com.example.app", "private_key": p8Key(t)}).Expect(201)
	key := e.Post("/v1/api-keys", owner.Token, map[string]any{"name": "backend", "scope": app,
		"permissions": []string{"notifications.send"}}).Expect(201).Str("secret")
	member := e.Signup()
	e.Grant(e.Admin, member.ID, app)
	dev := e.Post("/v1/users/me/devices", member.Token, map[string]any{"application_id": app, "provider": "apns",
		"token": apntok(), "environment": "production", "name": "Jordan's iPhone"}).Expect(201).Str("id")
	return pushApp{ID: app, Owner: owner, Key: key, Member: member, DeviceID: dev}
}

func apntok() string { return apnsToken() }

func TestPushCredentials(t *testing.T) {
	e := testenv.New(t)
	owner := e.Signup()
	app := e.App(owner.ID, nil)
	base := "/v1/applications/" + app + "/push-credentials"

	r := e.Post(base, owner.Token, map[string]any{"provider": "apns", "team_id": "ABCDE12345", "key_id": "XYZ987WVU6",
		"topic": "com.example.app", "private_key": p8Key(t)}).Expect(201)
	if r.Str("status") != "active" || r.Str("topic") != "com.example.app" || strings.Contains(string(r.Body), "PRIVATE KEY") {
		t.Fatalf("apns credential = %s", r.Body)
	}
	e.Post(base, owner.Token, map[string]any{"provider": "apns", "team_id": "ABCDE12345", "key_id": "XYZ987WVU6",
		"topic": "com.example.app", "private_key": p8Key(t)}).ExpectErr(409, "push_credential_exists")
	e.Post(base, owner.Token, map[string]any{"provider": "fcm", "service_account": map[string]any{"type": "service_account"}}).
		ExpectErr(422, "invalid_push_credential")
	fcm := e.Post(base, owner.Token, map[string]any{"provider": "fcm", "service_account": serviceAccount(t)}).Expect(201)
	if fcm.Str("project_id") != "timetrack-test" || strings.Contains(string(fcm.Body), "private_key") {
		t.Fatalf("fcm credential = %s", fcm.Body)
	}
	wp := e.Post(base, owner.Token, map[string]any{"provider": "web_push", "subject": "mailto:ops@example.com"}).Expect(201)
	if pub, _ := base64.RawURLEncoding.DecodeString(wp.Str("vapid_public_key")); len(pub) != 65 {
		t.Fatalf("vapid key = %q", wp.Str("vapid_public_key"))
	}
	e.Post(base, owner.Token, map[string]any{"provider": "apns", "team_id": "short"}).ExpectErr(422, "validation_failed")

	list := e.Get(base, owner.Token).Expect(200)
	if n := len(list.JSON()["data"].([]any)); n != 3 {
		t.Fatalf("list = %d credentials", n)
	}
	// The app's own key can read, never write.
	key := e.Post("/v1/api-keys", owner.Token, map[string]any{"name": "k", "scope": app, "permissions": []string{}}).Expect(201).Str("secret")
	e.Get(base+"/web_push", key).Expect(200)
	e.Patch(base+"/web_push", key, map[string]any{"subject": "mailto:x@example.com"}).ExpectErr(400, "user_token_required")
	// Another User can't see or change any of it.
	other := e.Signup()
	e.Get(base, other.Token).ExpectErr(404, "application_not_found")
	e.Post(base, other.Token, map[string]any{"provider": "fcm"}).ExpectErr(403, "forbidden")

	// PATCH: rotate the key; only fields of that provider; status rules.
	e.Patch(base+"/apns", owner.Token, map[string]any{"service_account": map[string]any{}}).ExpectErr(422, "validation_failed")
	e.Patch(base+"/apns", owner.Token, map[string]any{"provider": "fcm"}).ExpectErr(422, "read_only_field")
	e.Patch(base+"/apns", owner.Token, map[string]any{"status": "failed"}).ExpectErr(409, "invalid_status_transition")
	rot := e.Patch(base+"/apns", owner.Token, map[string]any{"key_id": "NEWKEY1234", "private_key": p8Key(t)}).Expect(200)
	if rot.Str("key_id") != "NEWKEY1234" {
		t.Fatalf("rotated = %s", rot.Body)
	}
	e.Get(base+"/nope", owner.Token).ExpectErr(404, "push_credential_not_found")
	e.Delete(base+"/fcm", owner.Token).Expect(204)
	e.Get(base+"/fcm", owner.Token).ExpectErr(404, "push_credential_not_found")

	audit := e.Get("/v1/audit-events?target_type=push_credential", e.Admin).Expect(200)
	actions := map[string]bool{}
	for _, ev := range audit.JSON()["data"].([]any) {
		m := ev.(map[string]any)
		actions[m["action"].(string)] = true
		if strings.Contains(mustJSON(m), "PRIVATE KEY") {
			t.Fatal("key material in an Audit Event")
		}
	}
	for _, a := range []string{"push_credential.created", "push_credential.updated", "push_credential.deleted"} {
		if !actions[a] {
			t.Errorf("missing audit action %s (got %v)", a, actions)
		}
	}
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func TestDevices(t *testing.T) {
	e := testenv.New(t)
	pa := newPushApp(t, e)
	m := pa.Member
	devs := "/v1/users/me/devices"

	// Re-registering the same token refreshes the same Device.
	again := e.Post(devs, m.Token, map[string]any{"application_id": pa.ID, "provider": "apns", "token": apntok(),
		"environment": "sandbox", "app_version": "2.5.0"}).Expect(200)
	if again.Str("id") != pa.DeviceID || again.Str("environment") != "sandbox" || again.Str("token_hint") != "…abab" {
		t.Fatalf("refresh = %s", again.Body)
	}
	if strings.Contains(string(again.Body), apntok()) {
		t.Fatal("the token itself was returned")
	}
	e.Post(devs, m.Token, map[string]any{"application_id": pa.ID, "provider": "apns", "token": "zz", "environment": "production"}).
		ExpectErr(422, "invalid_device_token")
	e.Post(devs, m.Token, map[string]any{"application_id": pa.ID, "provider": "apns", "token": apntok()}).
		ExpectErr(422, "validation_failed")
	// No access, no device.
	stranger := e.Signup()
	e.Post(devs, stranger.Token, map[string]any{"application_id": pa.ID, "provider": "fcm", "token": "fcm-token-1"}).
		ExpectErr(403, "entitlement_required")

	// A browser subscription.
	ua, _ := ecdh.P256().GenerateKey(rand.Reader)
	auth := make([]byte, 16)
	_, _ = rand.Read(auth)
	sub := map[string]any{"endpoint": "https://push.example.com/send/abc123",
		"keys": map[string]any{"p256dh": base64.RawURLEncoding.EncodeToString(ua.PublicKey().Bytes()),
			"auth": base64.RawURLEncoding.EncodeToString(auth)}}
	web := e.Post(devs, m.Token, map[string]any{"application_id": pa.ID, "provider": "web_push", "subscription": sub}).Expect(201)
	sub["keys"].(map[string]any)["auth"] = "short"
	e.Post(devs, m.Token, map[string]any{"application_id": pa.ID, "provider": "web_push", "subscription": sub}).
		ExpectErr(422, "invalid_device_token")

	list := e.Get(devs, m.Token).Expect(200)
	if n := len(list.JSON()["data"].([]any)); n != 2 {
		t.Fatalf("devices = %d", n)
	}
	// The app's own key manages its users' devices; other users' devices are invisible.
	e.Get("/v1/users/"+m.ID+"/devices/"+pa.DeviceID, pa.Key).Expect(200)
	e.Get("/v1/users/"+stranger.ID+"/devices", pa.Key).ExpectErr(404, "user_not_found")
	e.Get("/v1/users/"+m.ID+"/devices/"+pa.DeviceID, stranger.Token).ExpectErr(403, "forbidden")

	// PATCH a rotated token.
	newTok := strings.Repeat("cd", 32)
	p := e.Patch(devs+"/"+pa.DeviceID, m.Token, map[string]any{"token": newTok, "name": "New phone"}).Expect(200)
	if p.Str("token_hint") != "…cdcd" || p.Str("name") != "New phone" {
		t.Fatalf("patch = %s", p.Body)
	}
	e.Patch(devs+"/"+pa.DeviceID, m.Token, map[string]any{"provider": "fcm"}).ExpectErr(422, "read_only_field")

	// Logging out deactivates the devices that session registered.
	e.Post("/v1/auth/logout", m.Token, map[string]any{}).Expect(204)
	login := e.Login(m.Email, m.Password)
	tok := login.Token()
	got := e.Get(devs+"/"+pa.DeviceID, tok).Expect(200)
	if got.Str("status") != "inactive" || got.Str("inactive_reason") != "session_revoked" {
		t.Fatalf("after logout = %s", got.Body)
	}
	if n := len(e.Get(devs, tok).Expect(200).JSON()["data"].([]any)); n != 0 {
		t.Fatalf("active devices after logout = %d", n)
	}
	if n := len(e.Get(devs+"?include_inactive=true", tok).Expect(200).JSON()["data"].([]any)); n != 2 {
		t.Fatalf("all devices = %d", n)
	}
	// Signing back in and re-registering reactivates it.
	re := e.Post(devs, tok, map[string]any{"application_id": pa.ID, "provider": "apns", "token": newTok, "environment": "production"}).Expect(200)
	if re.Str("status") != "active" || re.Str("session_id") != login.Str("session_id") {
		t.Fatalf("re-register = %s", re.Body)
	}

	// Someone else signs in on the same install: the token moves to them.
	other := e.Signup()
	e.Grant(e.Admin, other.ID, pa.ID)
	moved := e.Post(devs, other.Token, map[string]any{"application_id": pa.ID, "provider": "apns", "token": newTok, "environment": "production"}).Expect(201)
	if moved.Str("id") == pa.DeviceID {
		t.Fatal("the previous User's Device was handed over")
	}
	e.Get(devs+"/"+pa.DeviceID, tok).ExpectErr(404, "device_not_found")
	if len(appEvents(e, "device.deleted", pa.ID)) == 0 || len(appEvents(e, "device.registered", pa.ID)) < 3 {
		t.Error("missing device webhook events")
	}

	// Device test: synchronous, and 409 without a credential for the provider.
	tr := e.Post("/v1/users/me/devices/"+moved.Str("id")+"/test", other.Token, map[string]any{}).Expect(200)
	if tr.Str("status") != "sent" {
		t.Fatalf("test = %s", tr.Body)
	}
	if msgs := e.Push.Sent(); msgs[len(msgs)-1].Notification.Title != "Test notification from Substratal" {
		t.Fatalf("test push = %+v", msgs[len(msgs)-1])
	}
	e.Post("/v1/users/me/devices/"+web.Str("id")+"/test", tok, map[string]any{}).ExpectErr(409, "push_not_configured")

	// Suspension deactivates everything; DELETE removes a device.
	e.Post("/v1/users/"+other.ID+"/suspend", e.Admin, map[string]any{}).Expect(200)
	sd := e.Get("/v1/users/"+other.ID+"/devices?include_inactive=true", e.Admin).Expect(200).JSON()["data"].([]any)
	if sd[0].(map[string]any)["inactive_reason"] != "user_suspended" {
		t.Fatalf("after suspend = %v", sd)
	}
	e.Delete(devs+"/"+web.Str("id"), tok).Expect(204)
	e.Delete("/v1/users/"+other.ID+"/devices/"+moved.Str("id"), e.Admin).Expect(204)
	if len(e.Get("/v1/audit-events?action=device.deleted", e.Admin).Expect(200).JSON()["data"].([]any)) != 1 {
		t.Error("an admin device delete should be audited (and only that one)")
	}
}

func TestSendAndInbox(t *testing.T) {
	e := testenv.New(t)
	pa := newPushApp(t, e)
	m := pa.Member
	send := "/v1/users/" + m.ID + "/notifications"
	body := map[string]any{"category": "timesheet.reminder", "title": "Timesheet due tomorrow",
		"body": "Due Friday at 5 pm.", "priority": "high", "data": map[string]any{"timesheet_id": "ts_8841"},
		"action_url": "com.example.app:/timesheets/ts_8841", "badge": 1, "collapse_key": "timesheet-due"}

	n := e.PostIdem(send, pa.Key, testenv.Unique(), body).Expect(201)
	push1 := n.JSON()["push"].(map[string]any)
	if push1["status"] != "queued" || push1["devices_targeted"] != float64(1) || n.Str("state") != "unread" ||
		n.Str("sound") != "default" || n.JSON()["sender"].(map[string]any)["type"] != "api_key" {
		t.Fatalf("send = %s", n.Body)
	}
	e.DeliverPush()
	msgs := e.Push.Sent()
	if len(msgs) != 1 || msgs[0].Target.Token != apntok() || msgs[0].Notification.Title != "Timesheet due tomorrow" ||
		msgs[0].Credential.Topic != "com.example.app" || msgs[0].Notification.Data["timesheet_id"] != "ts_8841" {
		t.Fatalf("pushes = %+v", msgs)
	}
	got := e.Get("/v1/users/me/notifications/"+n.Str("id"), m.Token).Expect(200).JSON()
	if p := got["push"].(map[string]any); p["status"] != "sent" || p["devices_sent"] != float64(1) {
		t.Fatalf("after delivery = %v", got)
	}
	if ds := got["deliveries"].([]any); len(ds) != 1 || ds[0].(map[string]any)["status"] != "sent" {
		t.Fatalf("deliveries = %v", got["deliveries"])
	}

	// Errors.
	noPerm := e.Post("/v1/api-keys", pa.Owner.Token, map[string]any{"name": "k2", "scope": pa.ID, "permissions": []string{}}).Expect(201).Str("secret")
	e.Post(send, noPerm, body).ExpectErr(403, "forbidden")
	stranger := e.Signup()
	e.Post("/v1/users/"+stranger.ID+"/notifications", pa.Key, body).ExpectErr(403, "entitlement_required")
	e.Post(send, pa.Key, map[string]any{"category": "security.alert", "title": "x"}).ExpectErr(422, "reserved_category")
	e.Post(send, pa.Key, map[string]any{"title": "x"}).ExpectErr(422, "validation_failed")
	e.Post(send, pa.Key, map[string]any{"category": "c", "title": "x", "data": map[string]any{"substratal": "x"}}).ExpectErr(422, "validation_failed")
	e.Post(send, pa.Key, map[string]any{"category": "c", "title": "x", "action_url": "http://insecure.example.com"}).ExpectErr(422, "validation_failed")
	// Every field within its own limit, but 4-byte characters push the
	// payload past the providers' shared 4 KB.
	e.Post(send, pa.Key, map[string]any{"category": "c", "title": strings.Repeat("😀", 120), "body": strings.Repeat("😀", 1000)}).
		ExpectErr(422, "notification_too_large")
	// A platform caller names the Application.
	e.Post(send, e.Admin, body).ExpectErr(422, "validation_failed")
	adm := e.Post(send, e.Admin, map[string]any{"category": "ops.notice", "title": "Maintenance tonight", "application_id": pa.ID, "push": false}).Expect(201)
	if adm.JSON()["push"].(map[string]any)["skip_reason"] != "push_not_requested" || adm.JSON()["sender"].(map[string]any)["type"] != "user" {
		t.Fatalf("inbox-only = %s", adm.Body)
	}

	// Preferences: push off for this app → inbox only.
	e.Patch("/v1/users/me/apps/"+pa.ID+"/settings", m.Token, map[string]any{"overrides": map[string]any{"notifications": map[string]any{"push": false}}}).Expect(200)
	off := e.Post(send, pa.Key, map[string]any{"category": "c.d", "title": "muted"}).Expect(201)
	if off.JSON()["push"].(map[string]any)["skip_reason"] != "push_disabled" {
		t.Fatalf("muted = %s", off.Body)
	}
	if len(appEvents(e, "notification.push_failed", pa.ID)) != 0 {
		t.Error("push_disabled is a choice, not a failure")
	}
	e.Patch("/v1/users/me/apps/"+pa.ID+"/settings", m.Token, map[string]any{"overrides": map[string]any{"notifications": map[string]any{"push": nil}}}).Expect(200)

	// No devices: skipped, and the app hears about it.
	other := e.Signup()
	e.Grant(e.Admin, other.ID, pa.ID)
	nd := e.Post("/v1/users/"+other.ID+"/notifications", pa.Key, map[string]any{"category": "c", "title": "hi"}).Expect(201)
	if nd.JSON()["push"].(map[string]any)["skip_reason"] != "no_devices" || len(appEvents(e, "notification.push_failed", pa.ID)) != 1 {
		t.Fatalf("no devices = %s", nd.Body)
	}

	// Inbox.
	inbox := "/v1/users/me/notifications"
	if l := e.Get(inbox, m.Token).Expect(200).JSON()["data"].([]any); len(l) != 3 {
		t.Fatalf("inbox = %d", len(l))
	}
	cnt := e.Get(inbox+"/unread-count", m.Token).Expect(200).JSON()
	if cnt["unread_count"] != float64(3) {
		t.Fatalf("unread = %v", cnt)
	}
	e.Patch(inbox+"/"+n.Str("id"), m.Token, map[string]any{"state": "archived"}).Expect(200)
	e.Patch(inbox+"/"+n.Str("id"), m.Token, map[string]any{"title": "x"}).ExpectErr(422, "read_only_field")
	if l := e.Get(inbox, m.Token).Expect(200).JSON()["data"].([]any); len(l) != 2 {
		t.Fatalf("archived still listed: %d", len(l))
	}
	if l := e.Get(inbox+"?state=archived", m.Token).Expect(200).JSON()["data"].([]any); len(l) != 1 {
		t.Fatalf("archived = %d", len(l))
	}
	mr := e.Post(inbox+"/mark-read", m.Token, map[string]any{"application_id": pa.ID}).Expect(200)
	if mr.JSON()["updated_count"] != float64(2) {
		t.Fatalf("mark-read = %s", mr.Body)
	}
	if e.Get(inbox+"/unread-count", m.Token).Expect(200).JSON()["unread_count"] != float64(0) {
		t.Fatal("still unread")
	}
	// The app's key sees only its own notifications; support reads, never writes.
	e.Get("/v1/users/"+m.ID+"/notifications?state=read", pa.Key).Expect(200)
	e.Get("/v1/users/"+m.ID+"/notifications", e.Admin).Expect(200)
	e.Post("/v1/users/"+m.ID+"/notifications/mark-read", e.Admin, map[string]any{}).ExpectErr(403, "forbidden")

	// Export carries devices (never the token) and notifications.
	ex := e.Get("/v1/users/me/export", m.Token).Expect(200)
	if strings.Contains(string(ex.Body), apntok()) || len(ex.JSON()["devices"].([]any)) != 1 || len(ex.JSON()["notifications"].([]any)) != 3 {
		t.Fatalf("export = %s", ex.Body)
	}
}

func TestPushOutcomes(t *testing.T) {
	e := testenv.New(t)
	pa := newPushApp(t, e)
	send := func() string {
		return e.Post("/v1/users/"+pa.Member.ID+"/notifications", pa.Key, map[string]any{"category": "c", "title": "hi"}).Expect(201).Str("id")
	}
	note := func(id string) map[string]any {
		return e.Get("/v1/users/me/notifications/"+id, pa.Member.Token).Expect(200).JSON()
	}

	// A transient failure stays pending, with the attempt counted.
	e.Push.Script(apntok(), push.Result{Outcome: push.Retry, Status: 503, Reason: "ServiceUnavailable"})
	id := send()
	e.DeliverPush()
	d := note(id)["deliveries"].([]any)[0].(map[string]any)
	if d["status"] != "pending" || d["attempts"] != float64(1) || note(id)["push"].(map[string]any)["status"] != "queued" {
		t.Fatalf("retry = %v", d)
	}

	// A gone token deactivates the device and fails the push.
	e.Push.Script(apntok(), push.Result{Outcome: push.TokenGone, Status: 410, Reason: "Unregistered"})
	expireLease(t, e)
	e.DeliverPush()
	if p := note(id)["push"].(map[string]any); p["status"] != "failed" || p["devices_failed"] != float64(1) {
		t.Fatalf("token gone = %v", p)
	}
	dev := e.Get("/v1/users/me/devices/"+pa.DeviceID, pa.Member.Token).Expect(200)
	if dev.Str("inactive_reason") != "unregistered" || len(appEvents(e, "device.deactivated", pa.ID)) != 1 ||
		len(appEvents(e, "notification.push_failed", pa.ID)) != 1 {
		t.Fatalf("device after 410 = %s", dev.Body)
	}

	// A rejected credential is marked failed, reported, and emailed.
	e.Post("/v1/users/me/devices", pa.Member.Token, map[string]any{"application_id": pa.ID, "provider": "apns",
		"token": apntok(), "environment": "production"}).Expect(200)
	e.Push.Script(apntok(), push.Result{Outcome: push.CredentialRejected, Status: 403, Reason: "InvalidProviderToken"})
	send()
	e.DeliverPush()
	cred := e.Get("/v1/applications/"+pa.ID+"/push-credentials/apns", pa.Owner.Token).Expect(200)
	if cred.Str("status") != "failed" || cred.Str("failure_reason") != "InvalidProviderToken" || len(appEvents(e, "push_credential.failed", pa.ID)) != 1 {
		t.Fatalf("credential = %s", cred.Body)
	}
	if _, ok := e.Mail.Last(pa.Owner.Email, "push_credential_failed"); !ok {
		t.Error("the owner wasn't emailed")
	}
	// Further sends skip until it's fixed.
	n := e.Post("/v1/users/"+pa.Member.ID+"/notifications", pa.Key, map[string]any{"category": "c", "title": "again"}).Expect(201)
	if n.JSON()["push"].(map[string]any)["skip_reason"] != "credential_failed" {
		t.Fatalf("after failure = %s", n.Body)
	}
	e.Patch("/v1/applications/"+pa.ID+"/push-credentials/apns", pa.Owner.Token, map[string]any{"status": "active"}).Expect(200)
}

// expireLease makes leased-but-retrying deliveries due now.
func expireLease(t *testing.T, e *testenv.Env) {
	t.Helper()
	err := e.DB.Tx(context.Background(), db.Settings{AllModes: true}, func(q db.Querier) error {
		_, err := q.Exec(context.Background(), `update notification_deliveries set next_attempt_at = now() where status = 'pending'`, nil)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

func sqlExec(t *testing.T, e *testenv.Env, q string, args db.Args) {
	t.Helper()
	if err := e.DB.Tx(context.Background(), db.Settings{AllModes: true}, func(qq db.Querier) error {
		_, err := qq.Exec(context.Background(), q, args)
		return err
	}); err != nil {
		t.Fatal(err)
	}
}

func TestPlatformAlerts(t *testing.T) {
	e := testenv.New(t)
	pa := newPushApp(t, e)
	m := pa.Member
	alerts := func(token string) map[string]map[string]any {
		out := map[string]map[string]any{}
		for _, x := range e.Get("/v1/users/me/notifications?source=platform", token).Expect(200).JSON()["data"].([]any) {
			n := x.(map[string]any)
			out[n["category"].(string)] = n
		}
		return out
	}

	// Without the opt-in, alerts land in the inbox but push nowhere.
	e.Do(testenv.Req{Method: "POST", Path: "/v1/auth/login", Body: map[string]any{"email": m.Email, "password": m.Password},
		Header: map[string]string{"User-Agent": "TimeTrack/2.4 (iOS 19.0)"}}).Expect(200)
	got := alerts(m.Token)
	si, ok := got["security.new_sign_in"]
	if !ok || si["application_id"] != nil || si["priority"] != "high" || si["push"].(map[string]any)["skip_reason"] != "no_devices" {
		t.Fatalf("new sign-in = %v", got)
	}
	// An app key never sees platform alerts.
	if l := e.Get("/v1/users/"+m.ID+"/notifications?source=platform", pa.Key).Expect(200).JSON()["data"].([]any); len(l) != 0 {
		t.Fatal("an app key saw a platform alert")
	}

	// With the opt-in, a password reset's alert reaches the device even
	// though the reset signs every session out.
	e.Patch("/v1/applications/"+pa.ID, pa.Owner.Token, map[string]any{"receives_platform_notifications": true}).Expect(200)
	e.Push.Reset()
	e.Post("/v1/auth/password/forgot", "", map[string]any{"email": m.Email}).Expect(202)
	e.Post("/v1/auth/password/reset", "", map[string]any{"token": e.Token(m.Email, "password_reset"),
		"new_password": "an entirely new passphrase"}).Expect(204)
	e.DeliverPush()
	msgs := e.Push.Sent()
	if len(msgs) != 1 || msgs[0].Notification.Category != "security.password_changed" || msgs[0].Notification.Source != "platform" {
		t.Fatalf("password alert pushes = %+v", msgs)
	}
	tok := e.Login(m.Email, "an entirely new passphrase").Token()
	if d := e.Get("/v1/users/me/devices/"+pa.DeviceID, tok).Expect(200); d.Str("inactive_reason") != "session_revoked" {
		t.Fatalf("device after reset = %s", d.Body)
	}

	// account.access_granted: developer grants only, never in the first minute.
	newbie := e.Signup()
	e.Grant(e.Admin, newbie.ID, e.App("usr_01JAG0SYSTEM00000000000000", nil))
	if _, ok := alerts(newbie.Token)["account.access_granted"]; ok {
		t.Fatal("a brand-new account got an access alert")
	}
	sqlExec(t, e, `update users set created_at = now() - interval '2 minutes' where id = :u`, db.Args{"u": newbie.ID})
	e.Grant(e.Admin, newbie.ID, e.App("usr_01JAG0SYSTEM00000000000000", nil))
	ag, ok := alerts(newbie.Token)["account.access_granted"]
	if !ok || !strings.HasPrefix(ag["title"].(string), "You now have access to") {
		t.Fatalf("access alert = %v", alerts(newbie.Token))
	}

	// An Organization invitation for an existing User.
	org := e.Post("/v1/organizations", pa.Owner.Token, map[string]any{"name": "Acme"}).Expect(201).Str("id")
	e.Post("/v1/organizations/"+org+"/members", pa.Owner.Token, map[string]any{"email": newbie.Email}).Expect(201)
	if inv, ok := alerts(newbie.Token)["account.organization_invitation"]; !ok || inv["data"].(map[string]any)["organization_id"] != org {
		t.Fatalf("invitation alert = %v", alerts(newbie.Token))
	}
}

func TestPushMaintenance(t *testing.T) {
	e := testenv.New(t)
	pa := newPushApp(t, e)
	sqlExec(t, e, `update devices set last_registered_at = now() - interval '300 days' where id = :d`, db.Args{"d": pa.DeviceID})
	if err := e.Core.PushMaintenance(context.Background()); err != nil {
		t.Fatal(err)
	}
	d := e.Get("/v1/users/me/devices/"+pa.DeviceID, pa.Member.Token).Expect(200)
	if d.Str("inactive_reason") != "stale" {
		t.Fatalf("stale = %s", d.Body)
	}
	sqlExec(t, e, `update devices set deactivated_at = now() - interval '31 days' where id = :d`, db.Args{"d": pa.DeviceID})
	if err := e.Core.PushMaintenance(context.Background()); err != nil {
		t.Fatal(err)
	}
	e.Get("/v1/users/me/devices/"+pa.DeviceID, pa.Member.Token).ExpectErr(404, "device_not_found")
}

// appEvents is the outbox events of a type for one Application (tests in
// a package share a database, so global counts would be meaningless).
func appEvents(e *testenv.Env, typ, appID string) []map[string]any {
	var out []map[string]any
	for _, ev := range e.Events(typ) {
		if ev["application_id"] == appID {
			out = append(out, ev)
		}
	}
	return out
}
