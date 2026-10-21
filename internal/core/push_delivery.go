package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/CompositeCode/substratalapps.com/internal/db"
	"github.com/CompositeCode/substratalapps.com/internal/ids"
	"github.com/CompositeCode/substratalapps.com/internal/push"
)

// pushSender is the configured provider client, or a log-only one.
func (s *Server) pushSender() push.Sender {
	if s.Push == nil {
		return push.Log{Logger: s.Log}
	}
	return s.Push
}

// ── Creating a notification and its deliveries ─────────────────────────

// notificationSpec is one notification to create, from an Application's
// send or from a platform alert.
type notificationSpec struct {
	UserID      string
	AppID       *string // nil for a platform alert
	Category    string
	Priority    string
	Title       string
	Body        *string
	Data        map[string]string
	ActionURL   *string
	Badge       *int
	Sound       *string
	CollapseKey *string
	TTL         int
	Push        bool
	SenderType  string
	SenderID    string
}

func (sp notificationSpec) source() string {
	if sp.AppID == nil {
		return "platform"
	}
	return "application"
}

// pushNotification is the provider-facing view of a spec.
func (sp notificationSpec) pushNotification(id string) push.Notification {
	return push.Notification{ID: id, ApplicationID: sp.AppID, Source: sp.source(), Category: sp.Category,
		Priority: sp.Priority, Title: sp.Title, Body: sp.Body, Data: sp.Data, ActionURL: sp.ActionURL,
		Badge: sp.Badge, Sound: sp.Sound, CollapseKey: sp.CollapseKey, TTL: time.Duration(sp.TTL) * time.Second}
}

type pushTarget struct {
	DeviceID string `json:"id"`
	AppID    string `json:"application_id"`
	Provider string `json:"provider"`
	CredOK   *bool  `json:"cred_ok"` // nil: no credential for the provider
	PushOn   bool   `json:"push_on"`
}

// pushEnabledSQL resolves notifications.push for (d.user_id, d.application_id):
// the per-app override, then global Settings, and a missing key is on.
const pushEnabledSQL = `coalesce(
	(select case jsonb_typeof(o.overrides->'notifications'->'push') when 'boolean' then (o.overrides->'notifications'->>'push')::boolean end
	   from app_settings o where o.user_id = d.user_id and o.application_id = d.application_id),
	(select case jsonb_typeof(s.notifications->'push') when 'boolean' then (s.notifications->>'push')::boolean end
	   from settings s where s.user_id = d.user_id),
	true)`

// planDeliveries picks the devices a notification is pushed to, or why it
// isn't pushed at all (API Reference → Push Notifications → skip_reason).
func (c *Call) planDeliveries(sp notificationSpec) ([]pushTarget, string, error) {
	if !sp.Push {
		return nil, "push_not_requested", nil
	}
	if sp.AppID != nil {
		var on bool
		if err := db.One(c.ctx, c.q, &on, `select to_jsonb(`+pushEnabledSQL+`) from (select :u::text as user_id, :a::text as application_id) d`,
			db.Args{"u": sp.UserID, "a": *sp.AppID}); err != nil {
			return nil, "", err
		}
		if !on {
			return nil, "push_disabled", nil
		}
	}
	appFilter := "d.application_id = :a"
	args := db.Args{"u": sp.UserID}
	if sp.AppID != nil {
		args["a"] = *sp.AppID
	} else {
		// A device belongs to one Application and only its credentials
		// reach it, so platform alerts go through apps that opt in.
		appFilter = "d.application_id in (select id from applications where receives_platform_notifications)"
	}
	devs, err := db.All[pushTarget](c.ctx, c.q, `select jsonb_build_object('id', d.id, 'application_id', d.application_id,
			'provider', d.provider, 'push_on', `+pushEnabledSQL+`,
			'cred_ok', (select p.status = 'active' from push_credentials p where p.application_id = d.application_id and p.provider = d.provider))
		from devices d where d.user_id = :u and d.status = 'active' and `+appFilter+`
		order by d.last_registered_at desc, d.id`, args)
	if err != nil {
		return nil, "", err
	}
	if len(devs) == 0 {
		return nil, "no_devices", nil
	}
	var on []pushTarget
	for _, d := range devs {
		if d.PushOn {
			on = append(on, d)
		}
	}
	if len(on) == 0 {
		return nil, "push_disabled", nil
	}
	var ok []pushTarget
	anyCred := false
	for _, d := range on {
		if d.CredOK != nil {
			anyCred = true
			if *d.CredOK {
				ok = append(ok, d)
			}
		}
	}
	switch {
	case len(ok) > 0:
		return ok, "", nil
	case anyCred:
		return nil, "credential_failed", nil
	}
	return nil, "no_credentials", nil
}

// createNotification writes a notification and one pending delivery per
// target device in the request's transaction, the push outbox, and nudges
// the worker once it commits.
func (c *Call) createNotification(id string, sp notificationSpec) error {
	targets, skip, err := c.planDeliveries(sp)
	if err != nil {
		return err
	}
	status := "queued"
	var skipArg any
	if skip != "" {
		status, skipArg = "skipped", skip
	}
	var tenant any
	if sp.AppID != nil {
		var t string
		if err := db.One(c.ctx, c.q, &t, `select to_jsonb(tenant_id) from applications where id = :a`, db.Args{"a": *sp.AppID}); err != nil {
			return err
		}
		tenant = t
	}
	data := sp.Data
	if data == nil {
		data = map[string]string{}
	}
	var badge any
	if sp.Badge != nil {
		badge = int64(*sp.Badge)
	}
	if _, err := c.q.Exec(c.ctx, `insert into notifications (id, user_id, application_id, tenant_id, source, category, priority,
			title, body, data, action_url, badge, sound, collapse_key, ttl_seconds, push_requested, push_status, skip_reason,
			push_devices_targeted, sender_type, sender_id, test_mode)
		values (:id, :u, :a, :t, :src, :cat, :pri, :title, :body, :data::jsonb, :url, :badge, :sound, :ck, :ttl, :push,
			:st, :skip, :n, :stype, :sid, :tm)`,
		db.Args{"id": id, "u": sp.UserID, "a": sp.AppID, "t": tenant, "src": sp.source(), "cat": sp.Category,
			"pri": sp.Priority, "title": sp.Title, "body": sp.Body, "data": db.JSON(data), "url": sp.ActionURL,
			"badge": badge, "sound": sp.Sound, "ck": sp.CollapseKey, "ttl": int64(sp.TTL), "push": sp.Push,
			"st": status, "skip": skipArg, "n": int64(len(targets)), "stype": sp.SenderType, "sid": sp.SenderID,
			"tm": c.testMode()}); err != nil {
		return err
	}
	for _, t := range targets {
		if _, err := c.q.Exec(c.ctx, `insert into notification_deliveries (id, notification_id, device_id, provider, next_attempt_at, test_mode)
			values (:id, :n, :d, :p, now(), :tm)`,
			db.Args{"id": ids.New(ids.NotificationDelivery), "n": id, "d": t.DeviceID, "p": t.Provider, "tm": c.testMode()}); err != nil {
			return err
		}
	}
	if len(targets) > 0 {
		c.After(func(ctx context.Context) { c.s.Notifier.EventCommitted(ctx, id) })
	} else if skip != "" && sp.AppID != nil && pushFailureSkip(skip) {
		return c.emitPushFailed(id)
	}
	return nil
}

// pushFailureSkip reports whether a skip is a failure an Application
// should hear about, rather than a choice (push_disabled, push_not_requested).
func pushFailureSkip(reason string) bool {
	return reason == "no_devices" || reason == "no_credentials" || reason == "credential_failed"
}

func (c *Call) emitPushFailed(notificationID string) error {
	var n struct {
		UserID   string          `json:"user_id"`
		AppID    *string         `json:"application_id"`
		Category string          `json:"category"`
		Push     json.RawMessage `json:"push"`
	}
	if err := db.One(c.ctx, c.q, &n, `select jsonb_build_object('user_id', n.user_id, 'application_id', n.application_id,
			'category', n.category, 'push', `+notificationPushJSON+`) from notifications n where n.id = :id`,
		db.Args{"id": notificationID}); err != nil {
		return err
	}
	if n.AppID == nil {
		return nil // platform alerts are between the platform and the User
	}
	return c.emit("notification.push_failed", n.AppID, map[string]any{"notification_id": notificationID,
		"user_id": n.UserID, "category": n.Category, "push": n.Push})
}

// ── Platform alerts (API Reference → Push Notifications → Platform alerts) ──

// platformAlertPriority is the catalog: each category and its priority.
var platformAlertPriority = map[string]string{
	"security.new_sign_in":            "high",
	"security.password_changed":       "high",
	"security.email_changed":          "high",
	"security.mfa_disabled":           "high",
	"security.session_reuse_detected": "high",
	"account.organization_invitation": "normal",
	"account.access_granted":          "normal",
}

// platformAlert raises one platform alert for an active User, in the
// current transaction. An action that also revokes sessions raises it
// first, so its deliveries target the devices that were active.
func (c *Call) platformAlert(userID, category, title string, body *string, data map[string]string) error {
	pri, ok := platformAlertPriority[category]
	if !ok {
		return fmt.Errorf("unknown platform alert %q", category)
	}
	var status string
	if err := db.One(c.ctx, c.q, &status, `select to_jsonb(status) from users where id = :u`, db.Args{"u": userID}); err != nil {
		if err == db.ErrNotFound {
			return nil
		}
		return err
	}
	if status != "active" {
		return nil
	}
	var sound *string
	if pri == "high" {
		sound = strp("default")
	}
	return c.createNotification(ids.New(ids.Notification), notificationSpec{UserID: userID, Category: category,
		Priority: pri, Title: title, Body: body, Data: data, Sound: sound, TTL: 86400, Push: true,
		SenderType: "system", SenderID: "system"})
}

// alertNewSignIn raises security.new_sign_in when a login's user agent
// hasn't been seen in the User's sessions for 90 days. A User's very first
// session (signup) raises nothing.
func (c *Call) alertNewSignIn(userID, sessionID, userAgent, ip string) error {
	var seen struct {
		Prior bool `json:"prior"`
		Known bool `json:"known"`
	}
	if err := db.One(c.ctx, c.q, &seen, `select jsonb_build_object(
			'prior', exists (select 1 from sessions where user_id = :u and id <> :s),
			'known', exists (select 1 from sessions where user_id = :u and id <> :s
			                  and user_agent is not distinct from :ua and created_at > now() - interval '90 days'))`,
		db.Args{"u": userID, "s": sessionID, "ua": nullIfEmpty(userAgent)}); err != nil {
		return err
	}
	if !seen.Prior || seen.Known {
		return nil
	}
	where := "an unrecognized device"
	if userAgent != "" {
		where = userAgent
	}
	data := map[string]string{"session_id": sessionID, "user_agent": userAgent}
	if ip != "" {
		data["ip_address"] = ip
	}
	return c.platformAlert(userID, "security.new_sign_in", "New sign-in to your Substratal account",
		strp("Signed in from "+where+". If this wasn't you, change your password."), data)
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// developerSources are the Entitlement sources a developer or admin
// grants. Platform-managed free and subscription rows never raise
// account.access_granted: signing up, subscribing to Standard, or a new
// free Application would otherwise alert once per app.
var developerSources = map[string]bool{"purchase": true, "trial": true, "admin_grant": true, "org_seat": true}

// alertAccessGranted raises account.access_granted when access a developer
// or admin granted becomes the User's resolved access, except in the first
// 60 seconds after signup.
func (c *Call) alertAccessGranted(p pair, r resolved) error {
	if r.Res.ActiveVia == nil || !developerSources[r.Res.ActiveVia.Source] {
		return nil
	}
	var info struct {
		New  bool   `json:"new"`
		Name string `json:"name"`
	}
	if err := db.One(c.ctx, c.q, &info, `select jsonb_build_object(
			'new', (select created_at > now() - interval '60 seconds' from users where id = :u),
			'name', (select name from applications where id = :a))`, db.Args{"u": p.User, "a": p.App}); err != nil {
		return err
	}
	if info.New {
		return nil
	}
	return c.platformAlert(p.User, "account.access_granted", "You now have access to "+info.Name, nil,
		map[string]string{"application_id": p.App})
}

// ── Settling a notification's push summary ─────────────────────────────

// notificationPushJSON is the push summary object of a notification n.
const notificationPushJSON = `jsonb_build_object('requested', n.push_requested, 'status', n.push_status,
	'devices_targeted', n.push_devices_targeted, 'devices_sent', n.push_devices_sent,
	'devices_failed', n.push_devices_failed, 'skip_reason', n.skip_reason)`

// settleNotification recomputes a notification's push summary from its
// deliveries and, when the push ends without reaching any device, tells
// the Application through notification.push_failed.
func (c *Call) settleNotification(notificationID string) error {
	var st struct {
		Status  string   `json:"push_status"`
		Pending int      `json:"pending"`
		Sent    int      `json:"sent"`
		Failed  int      `json:"failed"`
		Skipped []string `json:"skipped"`
	}
	err := db.One(c.ctx, c.q, &st, `select jsonb_build_object('push_status', n.push_status,
			'pending', count(d.id) filter (where d.status = 'pending'),
			'sent', count(d.id) filter (where d.status = 'sent'),
			'failed', count(d.id) filter (where d.status = 'failed'),
			'skipped', coalesce(array_agg(d.error) filter (where d.status = 'skipped'), '{}'))
		from notifications n left join notification_deliveries d on d.notification_id = n.id
		where n.id = :n group by n.id`, db.Args{"n": notificationID})
	if err == db.ErrNotFound {
		return nil
	}
	if err != nil {
		return err
	}
	if st.Status != "queued" {
		return nil // already settled (or never queued)
	}
	status, skip := "queued", ""
	switch {
	case st.Pending > 0:
	case st.Sent > 0:
		status = "sent"
	case st.Failed > 0:
		status = "failed"
	default:
		status, skip = "skipped", skipFromDeliveries(st.Skipped)
	}
	var skipArg any
	if skip != "" {
		skipArg = skip
	}
	if _, err := c.q.Exec(c.ctx, `update notifications set push_status = :st, skip_reason = :skip,
			push_devices_sent = :sent, push_devices_failed = :failed where id = :n`,
		db.Args{"st": status, "skip": skipArg, "sent": int64(st.Sent), "failed": int64(st.Failed), "n": notificationID}); err != nil {
		return err
	}
	if status == "failed" || (status == "skipped" && pushFailureSkip(skip)) {
		return c.emitPushFailed(notificationID)
	}
	return nil
}

// skipFromDeliveries names why a push whose every delivery was skipped
// never went out.
func skipFromDeliveries(errs []string) string {
	has := func(e string) bool {
		for _, x := range errs {
			if x == e {
				return true
			}
		}
		return false
	}
	switch {
	case has("expired"):
		return "expired"
	case has("credential_failed"):
		return "credential_failed"
	case has("credential_removed"):
		return "no_credentials"
	}
	return "no_devices"
}

// ── The delivery worker ────────────────────────────────────────────────

// PushRetrySchedule is the delay before attempts 2 through 4 (API
// Reference → Push Notifications → Delivery).
var PushRetrySchedule = []time.Duration{30 * time.Second, 2 * time.Minute, 10 * time.Minute}

const (
	pushMaxAttempts = 4
	pushLease       = 2 * time.Minute
)

// RunPush delivers every due push until none are left. The API nudges it
// after a send commits; the scheduled sweep runs it too, so the outbox is
// the only source of truth.
func (s *Server) RunPush(ctx context.Context) error {
	for i := 0; i < 50; i++ {
		n, err := s.DeliverPush(ctx, 50)
		if err != nil || n == 0 {
			return err
		}
	}
	return nil
}

// DeliverPush claims up to limit due deliveries with a short lease, so no
// transaction stays open across a provider call, and sends each.
func (s *Server) DeliverPush(ctx context.Context, limit int) (int, error) {
	type claim struct {
		ID       string `json:"id"`
		TestMode bool   `json:"test_mode"`
	}
	var batch []claim
	err := s.DB.Tx(ctx, db.Settings{AllModes: true}, func(q db.Querier) error {
		var err error
		batch, err = db.All[claim](ctx, q, `update notification_deliveries set next_attempt_at = now() + :lease::interval
			where id in (select id from notification_deliveries where status = 'pending' and next_attempt_at <= now()
			             order by next_attempt_at limit :lim for update skip locked)
			returning jsonb_build_object('id', id, 'test_mode', test_mode)`,
			db.Args{"lease": fmt.Sprintf("%d seconds", int(pushLease.Seconds())), "lim": int64(limit)})
		return err
	})
	if err != nil {
		return 0, err
	}
	for _, cl := range batch {
		if err := s.deliverOne(ctx, cl.ID, cl.TestMode); err != nil {
			s.Log.Error("push delivery", "delivery", cl.ID, "err", err)
		}
	}
	return len(batch), nil
}

// notificationRow is the stored notification, as the worker reads it.
type notificationRow struct {
	ID            string            `json:"id"`
	ApplicationID *string           `json:"application_id"`
	Source        string            `json:"source"`
	Category      string            `json:"category"`
	Priority      string            `json:"priority"`
	Title         string            `json:"title"`
	Body          *string           `json:"body"`
	Data          map[string]string `json:"data"`
	ActionURL     *string           `json:"action_url"`
	Badge         *int              `json:"badge"`
	Sound         *string           `json:"sound"`
	CollapseKey   *string           `json:"collapse_key"`
	TTL           int               `json:"ttl_seconds"`
	CreatedAt     time.Time         `json:"created_at"`
}

func (n notificationRow) push() push.Notification {
	return push.Notification{ID: n.ID, ApplicationID: n.ApplicationID, Source: n.Source, Category: n.Category,
		Priority: n.Priority, Title: n.Title, Body: n.Body, Data: n.Data, ActionURL: n.ActionURL, Badge: n.Badge,
		Sound: n.Sound, CollapseKey: n.CollapseKey, TTL: time.Duration(n.TTL) * time.Second}
}

// deliverOne sends one delivery in three steps: decide in a transaction,
// send outside any transaction, and record the outcome in another.
func (s *Server) deliverOne(ctx context.Context, id string, testMode bool) error {
	var x struct {
		Attempts int             `json:"attempts"`
		Provider string          `json:"provider"`
		DeviceID *string         `json:"device_id"`
		Note     notificationRow `json:"notification"`
	}
	var msg push.Message
	var dev device
	var credID string
	ready := false
	err := s.system(ctx, testMode, db.Settings{}, func(c *Call) error {
		if err := db.One(ctx, c.q, &x, `select jsonb_build_object('attempts', d.attempts, 'provider', d.provider,
				'device_id', d.device_id, 'notification', to_jsonb(n))
			from notification_deliveries d join notifications n on n.id = d.notification_id
			where d.id = :id and d.status = 'pending' for update of d`, db.Args{"id": id}); err != nil {
			if err == db.ErrNotFound {
				return nil // settled meanwhile
			}
			return err
		}
		finish := func(status, why string) error {
			if _, err := c.q.Exec(ctx, `update notification_deliveries set status = :st, error = :e, next_attempt_at = null
				where id = :id`, db.Args{"st": status, "e": why, "id": id}); err != nil {
				return err
			}
			return c.settleNotification(x.Note.ID)
		}
		// ttl_seconds is how long a provider may keep trying an offline
		// device; 0 means now or never.
		deadline := x.Note.CreatedAt.Add(time.Duration(x.Note.TTL) * time.Second)
		if (x.Note.TTL == 0 && x.Attempts > 0) || (x.Note.TTL > 0 && c.now.After(deadline)) {
			if x.Attempts == 0 {
				return finish("skipped", "expired")
			}
			return finish("failed", "expired")
		}
		if x.DeviceID == nil {
			return finish("skipped", "device_deleted")
		}
		devs, err := c.queryDevices("d.id = :id", db.Args{"id": *x.DeviceID})
		if err != nil {
			return err
		}
		if len(devs) == 0 {
			return finish("skipped", "device_deleted")
		}
		dev = devs[0]
		// A security alert raised by an action that also signs the User out
		// still reaches the devices that were active when it was raised.
		if dev.Status != "active" && !strings.HasPrefix(x.Note.Category, "security.") {
			return finish("skipped", "device_inactive")
		}
		cred, err := c.loadCredential(dev.ApplicationID, x.Provider)
		if err != nil {
			if isAPIError(err) {
				return finish("skipped", "credential_removed")
			}
			return err
		}
		if cred.Status != "active" {
			return finish("skipped", "credential_failed")
		}
		pc, err := s.decryptCredential(ctx, cred)
		if err != nil {
			return err
		}
		target, err := s.decryptTarget(ctx, dev)
		if err != nil {
			return err
		}
		msg, credID, ready = push.Message{Credential: pc, Target: target, Notification: x.Note.push()}, cred.ID, true
		return nil
	})
	if err != nil || !ready {
		return err
	}
	res := s.pushSender().Send(ctx, msg)
	return s.system(ctx, testMode, db.Settings{}, func(c *Call) error {
		attempts := x.Attempts + 1
		var provStatus any
		if res.Status != 0 {
			provStatus = int64(res.Status)
		}
		var why any
		if res.Reason != "" {
			why = res.Reason
		}
		record := func(status string, next *time.Time) error {
			var nextArg any
			if next != nil {
				nextArg = db.Time(*next)
			}
			_, err := c.q.Exec(ctx, `update notification_deliveries set status = :st, attempts = :a, provider_status = :ps,
					error = :e, next_attempt_at = :next::timestamptz,
					sent_at = case when :st = 'sent' then now() else sent_at end
				where id = :id`, db.Args{"st": status, "a": int64(attempts), "ps": provStatus, "e": why, "next": nextArg, "id": id})
			return err
		}
		if res.Outcome == push.Retry {
			next := c.now.Add(PushRetrySchedule[min(attempts, len(PushRetrySchedule))-1])
			if ra := c.now.Add(res.RetryAfter); ra.After(next) {
				next = ra
			}
			deadline := x.Note.CreatedAt.Add(time.Duration(x.Note.TTL) * time.Second)
			if attempts < pushMaxAttempts && x.Note.TTL > 0 && !next.After(deadline) {
				return record("pending", &next)
			}
			if err := record("failed", nil); err != nil {
				return err
			}
			return c.settleNotification(x.Note.ID)
		}
		status := "failed"
		if res.Outcome == push.Sent {
			status, why = "sent", nil
		}
		if err := record(status, nil); err != nil {
			return err
		}
		if err := c.applyOutcome(dev, credID, res); err != nil {
			return err
		}
		return c.settleNotification(x.Note.ID)
	})
}

// ── Maintenance (daily) ────────────────────────────────────────────────

// PushMaintenance runs the push lifecycle rules: devices not re-registered
// for 270 days go inactive (stale), inactive devices are deleted 30 days
// after deactivation, and notifications and delivery results age out of
// the inbox after 90 and 30 days.
func (s *Server) PushMaintenance(ctx context.Context) error {
	return eachMode(func(tm bool) error {
		return s.system(ctx, tm, db.Settings{}, func(c *Call) error {
			if err := c.deactivateDevices("d.last_registered_at < now() - interval '270 days'", nil, "stale"); err != nil {
				return err
			}
			old, err := c.queryDevices("d.status = 'inactive' and d.deactivated_at < now() - interval '30 days' limit 1000", nil)
			if err != nil {
				return err
			}
			for _, d := range old {
				if err := c.deleteDevice(d); err != nil {
					return err
				}
			}
			for _, q := range []string{
				`delete from notification_deliveries where created_at < now() - interval '30 days'`,
				`delete from notifications where created_at < now() - interval '90 days'`,
			} {
				if _, err := c.q.Exec(ctx, q, nil); err != nil {
					return err
				}
			}
			return nil
		})
	})
}
