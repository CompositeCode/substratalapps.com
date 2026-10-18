package core_test

import (
	"testing"

	"github.com/CompositeCode/substratalapps.com/internal/testenv"
)

// Push notifications are specified but not built: every route exists, runs
// the shared pipeline (so authentication and the user-token rule apply
// first), and answers 501 not_implemented in the standard error envelope.
func TestPushNotificationsReserved(t *testing.T) {
	e := testenv.New(t)
	u := e.Signup()
	app := e.App("usr_01JAG0SYSTEM00000000000000", nil)

	e.Get("/v1/users/me/notifications", "").ExpectErr(401, "unauthenticated")
	e.Get("/v1/users/me/notifications", u.Token).ExpectErr(501, "not_implemented")
	e.Get("/v1/users/me/notifications/unread-count", u.Token).ExpectErr(501, "not_implemented")
	e.Post("/v1/users/me/devices", u.Token, map[string]any{
		"application_id": app, "provider": "fcm", "token": "fcm-registration-token",
	}).ExpectErr(501, "not_implemented")
	e.Delete("/v1/users/me/devices/dvc_01JAH2B3C4D5E6F7G8H9J0K1M2", u.Token).ExpectErr(501, "not_implemented")

	// Push credentials are managed by people only: an API Key is refused
	// before the route's 501.
	key := e.Post("/v1/api-keys", e.Admin, map[string]any{"name": "app", "scope": app, "permissions": []string{}}).
		Expect(201).Str("secret")
	e.Post("/v1/applications/"+app+"/push-credentials", key, map[string]any{"provider": "web_push", "subject": "mailto:ops@example.com"}).
		ExpectErr(400, "user_token_required")
	e.Get("/v1/applications/"+app+"/push-credentials", key).ExpectErr(501, "not_implemented")
}
