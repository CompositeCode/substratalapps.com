package core

import (
	"net/http"

	"github.com/CompositeCode/substratalapps.com/internal/api/gen"
	"github.com/CompositeCode/substratalapps.com/internal/httpx"
)

// Push notifications (API Reference → Push Notifications) are specified
// but not built yet. Every operation runs the shared pipeline, so
// authentication, rate limits, and the destructive-operation gate already
// apply, and then returns 501 not_implemented, the same treatment as the
// SSO callback. Idempotency stays off until the real handlers land, so a
// stored 501 can never be replayed after they do.

func notImplemented(c *Call) error {
	return httpx.E(501, "not_implemented", "Push notifications aren't available yet.")
}

// PushCredentialsList is reserved until push notifications ship.
func (s *Server) PushCredentialsList(w http.ResponseWriter, r *http.Request, id string) {
	s.serve(w, r, opts{op: "pushCredentials.list"}, notImplemented)
}

// PushCredentialsCreate is reserved until push notifications ship.
func (s *Server) PushCredentialsCreate(w http.ResponseWriter, r *http.Request, id string, params gen.PushCredentialsCreateParams) {
	s.serve(w, r, opts{op: "pushCredentials.create", userOnly: true}, notImplemented)
}

// PushCredentialsGet is reserved until push notifications ship.
func (s *Server) PushCredentialsGet(w http.ResponseWriter, r *http.Request, id string, provider gen.PushProvider) {
	s.serve(w, r, opts{op: "pushCredentials.get"}, notImplemented)
}

// PushCredentialsUpdate is reserved until push notifications ship.
func (s *Server) PushCredentialsUpdate(w http.ResponseWriter, r *http.Request, id string, provider gen.PushProvider, params gen.PushCredentialsUpdateParams) {
	s.serve(w, r, opts{op: "pushCredentials.update", userOnly: true}, notImplemented)
}

// PushCredentialsDelete is reserved until push notifications ship.
func (s *Server) PushCredentialsDelete(w http.ResponseWriter, r *http.Request, id string, provider gen.PushProvider) {
	s.serve(w, r, opts{op: "pushCredentials.delete", userOnly: true}, func(c *Call) error {
		if err := c.destructiveAlways(); err != nil {
			return err
		}
		return notImplemented(c)
	})
}

// DevicesList is reserved until push notifications ship.
func (s *Server) DevicesList(w http.ResponseWriter, r *http.Request, id gen.UserIdOrMe, params gen.DevicesListParams) {
	s.serve(w, r, opts{op: "devices.list"}, notImplemented)
}

// DevicesRegister is reserved until push notifications ship.
func (s *Server) DevicesRegister(w http.ResponseWriter, r *http.Request, id gen.UserIdOrMe) {
	s.serve(w, r, opts{op: "devices.register"}, notImplemented)
}

// DevicesGet is reserved until push notifications ship.
func (s *Server) DevicesGet(w http.ResponseWriter, r *http.Request, id gen.UserIdOrMe, deviceID string) {
	s.serve(w, r, opts{op: "devices.get"}, notImplemented)
}

// DevicesUpdate is reserved until push notifications ship.
func (s *Server) DevicesUpdate(w http.ResponseWriter, r *http.Request, id gen.UserIdOrMe, deviceID string, params gen.DevicesUpdateParams) {
	s.serve(w, r, opts{op: "devices.update"}, notImplemented)
}

// DevicesDelete is reserved until push notifications ship.
func (s *Server) DevicesDelete(w http.ResponseWriter, r *http.Request, id gen.UserIdOrMe, deviceID string) {
	s.serve(w, r, opts{op: "devices.delete"}, func(c *Call) error {
		if err := c.destructiveAlways(); err != nil {
			return err
		}
		return notImplemented(c)
	})
}

// DevicesSendTest is reserved until push notifications ship.
func (s *Server) DevicesSendTest(w http.ResponseWriter, r *http.Request, id gen.UserIdOrMe, deviceID string) {
	s.serve(w, r, opts{op: "devices.sendTest"}, notImplemented)
}

// NotificationsList is reserved until push notifications ship.
func (s *Server) NotificationsList(w http.ResponseWriter, r *http.Request, id gen.UserIdOrMe, params gen.NotificationsListParams) {
	s.serve(w, r, opts{op: "notifications.list"}, notImplemented)
}

// NotificationsSend is reserved until push notifications ship.
func (s *Server) NotificationsSend(w http.ResponseWriter, r *http.Request, id gen.UserIdOrMe, params gen.NotificationsSendParams) {
	s.serve(w, r, opts{op: "notifications.send"}, notImplemented)
}

// NotificationsUnreadCount is reserved until push notifications ship.
func (s *Server) NotificationsUnreadCount(w http.ResponseWriter, r *http.Request, id gen.UserIdOrMe, params gen.NotificationsUnreadCountParams) {
	s.serve(w, r, opts{op: "notifications.unreadCount"}, notImplemented)
}

// NotificationsMarkRead is reserved until push notifications ship.
func (s *Server) NotificationsMarkRead(w http.ResponseWriter, r *http.Request, id gen.UserIdOrMe) {
	s.serve(w, r, opts{op: "notifications.markRead"}, notImplemented)
}

// NotificationsGet is reserved until push notifications ship.
func (s *Server) NotificationsGet(w http.ResponseWriter, r *http.Request, id gen.UserIdOrMe, notificationID string) {
	s.serve(w, r, opts{op: "notifications.get"}, notImplemented)
}

// NotificationsUpdate is reserved until push notifications ship.
func (s *Server) NotificationsUpdate(w http.ResponseWriter, r *http.Request, id gen.UserIdOrMe, notificationID string, params gen.NotificationsUpdateParams) {
	s.serve(w, r, opts{op: "notifications.update"}, notImplemented)
}
