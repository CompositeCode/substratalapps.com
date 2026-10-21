package core

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/CompositeCode/substratalapps.com/internal/api/gen"
	"github.com/CompositeCode/substratalapps.com/internal/db"
	"github.com/CompositeCode/substratalapps.com/internal/httpx"
	"github.com/CompositeCode/substratalapps.com/internal/ids"
	"github.com/CompositeCode/substratalapps.com/internal/push"
)

// maxActiveDevices is the per-User, per-Application cap; a newer
// registration displaces the one registered longest ago.
const maxActiveDevices = 20

// device is the API view of a Device. The token is never part of it.
type device struct {
	ID               string     `json:"id"`
	UserID           string     `json:"user_id"`
	ApplicationID    string     `json:"application_id"`
	Provider         string     `json:"provider"`
	Environment      *string    `json:"environment"`
	TokenHint        string     `json:"token_hint"`
	Name             *string    `json:"name"`
	AppVersion       *string    `json:"app_version"`
	SessionID        *string    `json:"session_id"`
	Status           string     `json:"status"`
	InactiveReason   *string    `json:"inactive_reason"`
	LastRegisteredAt time.Time  `json:"last_registered_at"`
	LastDeliveryAt   *time.Time `json:"last_delivery_at"`
	TestMode         bool       `json:"test_mode"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	version          int
	tokenCiphertext  string
}

// deviceJSON is a device row as JSON; it works in a select and in an
// update ... returning, wherever the table is aliased d.
const deviceJSON = `jsonb_build_object('id', d.id, 'user_id', d.user_id, 'application_id', d.application_id,
	'provider', d.provider, 'environment', d.environment, 'token_hint', d.token_hint, 'name', d.name,
	'app_version', d.app_version, 'session_id', d.session_id, 'status', d.status, 'inactive_reason', d.inactive_reason,
	'last_registered_at', d.last_registered_at, 'last_delivery_at', d.last_delivery_at, 'test_mode', d.test_mode,
	'created_at', d.created_at, 'updated_at', d.updated_at, 'version_', d.version, 'token_', d.token_ciphertext)`

func decodeDevices(rows []json.RawMessage) ([]device, error) {
	out := make([]device, 0, len(rows))
	for _, raw := range rows {
		var x struct {
			device
			V int    `json:"version_"`
			T string `json:"token_"`
		}
		if err := json.Unmarshal(raw, &x); err != nil {
			return nil, err
		}
		x.device.version, x.device.tokenCiphertext = x.V, x.T
		out = append(out, x.device)
	}
	return out, nil
}

func (c *Call) queryDevices(where string, args db.Args) ([]device, error) {
	rows, err := c.q.Query(c.ctx, `select `+deviceJSON+` from devices d where `+where, args)
	if err != nil {
		return nil, err
	}
	return decodeDevices(rows)
}

// deactivateDevices makes every active device matching where inactive for
// reason, and tells each Application through device.deactivated.
func (c *Call) deactivateDevices(where string, args db.Args, reason string) error {
	a := db.Args{"reason": reason}
	for k, v := range args {
		a[k] = v
	}
	rows, err := c.q.Query(c.ctx, `update devices as d set status = 'inactive', inactive_reason = :reason, deactivated_at = now()
		where d.status = 'active' and (`+where+`) returning `+deviceJSON, a)
	if err != nil {
		return err
	}
	devs, err := decodeDevices(rows)
	if err != nil {
		return err
	}
	for _, d := range devs {
		app := d.ApplicationID
		if err := c.emit("device.deactivated", &app, map[string]any{"device": d}); err != nil {
			return err
		}
	}
	return nil
}

// sessionsEnded deactivates the devices registered under revoked sessions
// (API Reference → Push Notifications → Sessions and device lifecycle).
func (c *Call) sessionsEnded(sessionIDs []string) error {
	if len(sessionIDs) == 0 {
		return nil
	}
	return c.deactivateDevices("d.session_id = any(:sids::text[])", db.Args{"sids": db.TextArray(sessionIDs)}, "session_revoked")
}

// deleteDevice removes a device and its pending deliveries, and tells the
// Application through device.deleted.
func (c *Call) deleteDevice(d device) error {
	notes, err := db.All[string](c.ctx, c.q, `delete from notification_deliveries where device_id = :d and status = 'pending'
		returning to_jsonb(notification_id)`, db.Args{"d": d.ID})
	if err != nil {
		return err
	}
	if _, err := c.q.Exec(c.ctx, `delete from devices where id = :d`, db.Args{"d": d.ID}); err != nil {
		return err
	}
	for _, n := range dedupe(notes) {
		if err := c.settleNotification(n); err != nil {
			return err
		}
	}
	app := d.ApplicationID
	return c.emit("device.deleted", &app, map[string]any{"device": d})
}

// deviceUser authorizes device access for the User in {id}: the User
// themself, the app's own key (for its own Application's devices, and only
// for Users with an access path to it), and support (users.list to read,
// users.manage to delete). It returns the User id and, for an app key, the
// Application the caller is confined to.
func (c *Call) deviceUser(id string, write, del bool) (string, *string, error) {
	uid, err := c.selfID(id)
	if err != nil {
		return "", nil, err
	}
	if _, err := c.visibleUser(uid); err != nil {
		return "", nil, err
	}
	if c.isSelf(uid) {
		return uid, nil, nil
	}
	if app, ok := c.p.AppKey(); ok {
		has, err := c.userHasAccessPath(uid, app)
		if err != nil {
			return "", nil, err
		}
		if !has {
			return "", nil, httpx.NotFoundResource("user")
		}
		return uid, &app, nil
	}
	switch {
	case del && c.can("users.manage"):
		return uid, nil, nil
	case !write && (c.can("users.list") || c.can("users.manage")):
		return uid, nil, nil
	case write || del:
		return "", nil, httpx.Forbidden("users.manage")
	}
	return "", nil, httpx.Forbidden("users.list")
}

func (c *Call) loadDevice(uid, deviceID string, app *string) (device, error) {
	where, args := "d.id = :id and d.user_id = :u", db.Args{"id": deviceID, "u": uid}
	if app != nil {
		where += " and d.application_id = :app"
		args["app"] = *app
	}
	ds, err := c.queryDevices(where, args)
	if err != nil {
		return device{}, err
	}
	if len(ds) == 0 {
		return device{}, httpx.NotFoundResource("device")
	}
	return ds[0], nil
}

// deviceToken is a validated provider token, ready to store.
type deviceToken struct {
	hash, hint, plaintext string // plaintext: the token, or the subscription JSON
}

func invalidDeviceToken(why string) error {
	return httpx.E(422, "invalid_device_token", "The device token isn't valid for its provider: "+why+".")
}

// validateToken checks token or subscription for provider.
func (c *Call) validateToken(provider string, token *string, sub *push.Subscription) (deviceToken, error) {
	switch provider {
	case push.APNs, push.FCM:
		if sub != nil {
			return deviceToken{}, invalidDeviceToken("subscription is only for web_push")
		}
		if token == nil || *token == "" {
			var f httpx.Fields
			f.Add("token", "required")
			return deviceToken{}, f.Err()
		}
		t := strings.TrimSpace(*token)
		if provider == push.APNs {
			t = strings.ToLower(t)
			if len(t) < 64 || len(t) > 200 || len(t)%2 != 0 {
				return deviceToken{}, invalidDeviceToken("an APNs token is 64–200 hex characters")
			}
			if _, err := hex.DecodeString(t); err != nil {
				return deviceToken{}, invalidDeviceToken("an APNs token is hex")
			}
		} else if len(t) > 4096 || strings.ContainsAny(t, " \t\r\n") {
			return deviceToken{}, invalidDeviceToken("an FCM token is 1–4,096 characters with no whitespace")
		}
		return deviceToken{hash: sha256Hex([]byte(t)), hint: "…" + t[len(t)-4:], plaintext: t}, nil
	case push.WebPush:
		if token != nil {
			return deviceToken{}, invalidDeviceToken("web_push takes subscription, not token")
		}
		if sub == nil || sub.Endpoint == "" {
			var f httpx.Fields
			f.Add("subscription", "required")
			return deviceToken{}, f.Err()
		}
		u, err := url.Parse(sub.Endpoint)
		if err != nil || u.Host == "" || u.User != nil || len(sub.Endpoint) > 2048 {
			return deviceToken{}, invalidDeviceToken("subscription.endpoint isn't a URL")
		}
		if u.Scheme != "https" && !(AllowPrivateWebhookTargets && u.Scheme == "http") {
			return deviceToken{}, invalidDeviceToken("subscription.endpoint must be https")
		}
		if !AllowPrivateWebhookTargets {
			if err := CheckPublicHost(c.ctx, u.Hostname()); err != nil {
				return deviceToken{}, invalidDeviceToken("subscription.endpoint must resolve to a public address")
			}
		}
		if err := push.ValidateSubscriptionKeys(sub.Keys.P256dh, sub.Keys.Auth); err != nil {
			return deviceToken{}, invalidDeviceToken(err.Error())
		}
		raw, _ := json.Marshal(sub)
		return deviceToken{hash: sha256Hex([]byte(sub.Endpoint)), hint: "…" + sub.Endpoint[len(sub.Endpoint)-4:],
			plaintext: string(raw)}, nil
	}
	var f httpx.Fields
	f.Enum("provider", push.Providers...)
	return deviceToken{}, f.Err()
}

// enforceDeviceCap deactivates the oldest active devices beyond the cap.
func (c *Call) enforceDeviceCap(uid, appID string) error {
	return c.deactivateDevices(`d.id in (select id from devices where user_id = :u and application_id = :a and status = 'active'
		order by last_registered_at desc, id desc offset :cap)`,
		db.Args{"u": uid, "a": appID, "cap": int64(maxActiveDevices)}, "limit")
}

func (s *Server) DevicesList(w http.ResponseWriter, r *http.Request, id gen.UserIdOrMe, params gen.DevicesListParams) {
	s.serve(w, r, opts{op: "devices.list"}, func(c *Call) error {
		uid, app, err := c.deviceUser(id, false, false)
		if err != nil {
			return err
		}
		where, args := []string{"d.user_id = :u"}, db.Args{"u": uid}
		if app != nil {
			where = append(where, "d.application_id = :app")
			args["app"] = *app
		}
		if params.ApplicationId != nil {
			where = append(where, "d.application_id = :fapp")
			args["fapp"] = *params.ApplicationId
		}
		if params.Provider != nil {
			where = append(where, "d.provider = :prov")
			args["prov"] = string(*params.Provider)
		}
		switch {
		case params.Status != nil:
			where = append(where, "d.status = :st")
			args["st"] = string(*params.Status)
		case params.IncludeInactive == nil || !*params.IncludeInactive:
			where = append(where, "d.status = 'active'")
		}
		lim := httpx.Limit(params.Limit)
		args["lim"] = int64(lim + 1)
		filters := httpx.Filters(c.r)
		pos, err := c.s.Cursors.Decode(deref(params.Cursor), filters)
		if err != nil {
			return err
		}
		if pos != nil {
			where = append(where, "(d.created_at, d.id) < (:ps::timestamptz, :pi)")
			args["ps"], args["pi"] = pos.Sort, pos.ID
		}
		ds, err := c.queryDevices(strings.Join(where, " and ")+" order by d.created_at desc, d.id desc limit :lim", args)
		if err != nil {
			return err
		}
		return c.OK(httpx.Paginate(c.s.Cursors, ds, lim, filters, func(x device) (string, string) {
			return httpx.TimeKey(x.CreatedAt), x.ID
		}))
	})
}

func (s *Server) DevicesGet(w http.ResponseWriter, r *http.Request, id gen.UserIdOrMe, deviceID string) {
	s.serve(w, r, opts{op: "devices.get"}, func(c *Call) error {
		uid, app, err := c.deviceUser(id, false, false)
		if err != nil {
			return err
		}
		d, err := c.loadDevice(uid, deviceID, app)
		if err != nil {
			return err
		}
		c.Versioned(d.version)
		return c.OK(d)
	})
}

func (s *Server) DevicesRegister(w http.ResponseWriter, r *http.Request, id gen.UserIdOrMe) {
	s.serve(w, r, opts{op: "devices.register"}, func(c *Call) error {
		uid, keyApp, err := c.deviceUser(id, true, false)
		if err != nil {
			return err
		}
		var in struct {
			ApplicationID string             `json:"application_id"`
			Provider      string             `json:"provider"`
			Token         *string            `json:"token"`
			Subscription  *push.Subscription `json:"subscription"`
			Environment   *string            `json:"environment"`
			Name          *string            `json:"name"`
			AppVersion    *string            `json:"app_version"`
		}
		if err := c.Decode(&in); err != nil {
			return err
		}
		var f httpx.Fields
		if in.ApplicationID == "" {
			f.Add("application_id", "required")
		}
		if in.Provider == "" {
			f.Add("provider", "required")
		} else if !validProvider(in.Provider) {
			f.Enum("provider", push.Providers...)
		}
		if in.Provider == push.APNs {
			if in.Environment == nil {
				f.Add("environment", "required")
			} else if *in.Environment != "production" && *in.Environment != "sandbox" {
				f.Enum("environment", "production", "sandbox")
			}
		} else if in.Environment != nil {
			f.Add("environment", "undeclared_key").Message = "environment is only for apns"
		}
		labels(&f, in.Name, in.AppVersion)
		if err := f.Err(); err != nil {
			return err
		}
		if keyApp != nil && *keyApp != in.ApplicationID {
			return httpx.Forbidden("")
		}
		a, err := c.loadApp(in.ApplicationID)
		if err == db.ErrNotFound {
			f.Add("application_id", "unknown_value")
			return f.Err()
		}
		if err != nil {
			return err
		}
		t, err := c.loadTenant(a.TenantID)
		if err != nil {
			return err
		}
		if err := tenantWritable(t); err != nil {
			return err
		}
		if err := c.requireAccess(uid, a.ID); err != nil {
			return err
		}
		tok, err := c.validateToken(in.Provider, in.Token, in.Subscription)
		if err != nil {
			return err
		}
		var session any
		if c.isSelf(uid) {
			session = c.p.SessionID
		}
		existing, err := c.queryDevices("d.application_id = :a and d.provider = :p and d.token_hash = :h",
			db.Args{"a": a.ID, "p": in.Provider, "h": tok.hash})
		if err != nil {
			return err
		}
		enc, err := c.s.Encrypter.Encrypt(c.ctx, tok.plaintext)
		if err != nil {
			return err
		}
		if len(existing) == 1 && existing[0].UserID == uid {
			// A refresh of the caller's own registration.
			before := existing[0]
			if _, err := c.q.Exec(c.ctx, `update devices set name = :n, app_version = :v, environment = :env,
					session_id = :s, token_ciphertext = :tok, last_registered_at = now(),
					status = 'active', inactive_reason = null, deactivated_at = null
				where id = :id`, db.Args{"n": in.Name, "v": in.AppVersion, "env": in.Environment, "s": session,
				"tok": enc, "id": before.ID}); err != nil {
				return err
			}
			after, err := c.loadDevice(uid, before.ID, nil)
			if err != nil {
				return err
			}
			if before.Status == "inactive" {
				app := a.ID
				if err := c.emit("device.registered", &app, map[string]any{"device": after}); err != nil {
					return err
				}
				if err := c.enforceDeviceCap(uid, a.ID); err != nil {
					return err
				}
			}
			c.Versioned(after.version)
			return c.OK(after)
		}
		if len(existing) == 1 {
			// Someone else signed in on this install: the token is theirs
			// now, and the previous User's history doesn't come with it.
			if err := c.deleteDevice(existing[0]); err != nil {
				return err
			}
		}
		devID := ids.New(ids.Device)
		if _, err := c.q.Exec(c.ctx, `insert into devices (id, user_id, application_id, tenant_id, session_id, provider,
				environment, token_hash, token_ciphertext, token_hint, name, app_version, test_mode)
			values (:id, :u, :a, :t, :s, :p, :env, :h, :tok, :hint, :n, :v, :tm)`,
			db.Args{"id": devID, "u": uid, "a": a.ID, "t": a.TenantID, "s": session, "p": in.Provider,
				"env": in.Environment, "h": tok.hash, "tok": enc, "hint": tok.hint, "n": in.Name, "v": in.AppVersion,
				"tm": c.testMode()}); err != nil {
			return err
		}
		out, err := c.loadDevice(uid, devID, nil)
		if err != nil {
			return err
		}
		app := a.ID
		if err := c.emit("device.registered", &app, map[string]any{"device": out}); err != nil {
			return err
		}
		if err := c.enforceDeviceCap(uid, a.ID); err != nil {
			return err
		}
		c.Versioned(out.version)
		return c.Created(out)
	})
}

func labels(f *httpx.Fields, name, appVersion *string) {
	if name != nil && len([]rune(*name)) > 100 {
		f.Max("name", "too_long", 100)
	}
	if appVersion != nil && len([]rune(*appVersion)) > 50 {
		f.Max("app_version", "too_long", 50)
	}
}

// requireAccess is 403 entitlement_required unless allow() passes for the
// User and Application right now.
func (c *Call) requireAccess(uid, appID string) error {
	res, err := c.resolveOne(uid, appID)
	if err != nil {
		return err
	}
	if !res.Allowed {
		return httpx.E(403, "entitlement_required", "This user has no active entitlement to "+appID+".").
			With("application_id", appID).With("entitlement_status", res.Res.Status)
	}
	return nil
}

func (s *Server) DevicesUpdate(w http.ResponseWriter, r *http.Request, id gen.UserIdOrMe, deviceID string, params gen.DevicesUpdateParams) {
	s.serve(w, r, opts{op: "devices.update"}, func(c *Call) error {
		uid, app, err := c.deviceUser(id, true, false)
		if err != nil {
			return err
		}
		before, err := c.loadDevice(uid, deviceID, app)
		if err != nil {
			return err
		}
		if err := httpx.CheckIfMatch(c.r, before.version); err != nil {
			return err
		}
		p, err := c.Patch()
		if err != nil {
			return err
		}
		if err := p.RejectReadOnly("id", "user_id", "application_id", "provider", "session_id", "status", "inactive_reason",
			"token_hint", "last_registered_at", "last_delivery_at", "test_mode", "created_at", "updated_at"); err != nil {
			return err
		}
		var in struct {
			Token        *string            `json:"token"`
			Subscription *push.Subscription `json:"subscription"`
			Environment  *string            `json:"environment"`
			Name         *string            `json:"name"`
			AppVersion   *string            `json:"app_version"`
		}
		if err := c.Decode(&in); err != nil {
			return err
		}
		var f httpx.Fields
		next := before
		if p.Has("name") {
			next.Name = in.Name
		}
		if p.Has("app_version") {
			next.AppVersion = in.AppVersion
		}
		labels(&f, next.Name, next.AppVersion)
		if p.Has("environment") {
			switch {
			case before.Provider != push.APNs:
				f.Add("environment", "undeclared_key").Message = "environment is only for apns"
			case in.Environment == nil || (*in.Environment != "production" && *in.Environment != "sandbox"):
				f.Enum("environment", "production", "sandbox")
			default:
				next.Environment = in.Environment
			}
		}
		if err := f.Err(); err != nil {
			return err
		}
		args := db.Args{"id": before.ID, "n": next.Name, "v": next.AppVersion, "env": next.Environment,
			"tok": nil, "h": nil, "hint": nil}
		if p.Has("token") || p.Has("subscription") {
			tok, err := c.validateToken(before.Provider, in.Token, in.Subscription)
			if err != nil {
				return err
			}
			// One token, one Device: another registration of it goes.
			others, err := c.queryDevices("d.application_id = :a and d.provider = :p and d.token_hash = :h and d.id <> :id",
				db.Args{"a": before.ApplicationID, "p": before.Provider, "h": tok.hash, "id": before.ID})
			if err != nil {
				return err
			}
			for _, o := range others {
				if err := c.deleteDevice(o); err != nil {
					return err
				}
			}
			enc, err := c.s.Encrypter.Encrypt(c.ctx, tok.plaintext)
			if err != nil {
				return err
			}
			args["tok"], args["h"], args["hint"] = enc, tok.hash, tok.hint
		}
		if _, err := c.q.Exec(c.ctx, `update devices set name = :n, app_version = :v, environment = :env,
				token_ciphertext = coalesce(:tok, token_ciphertext), token_hash = coalesce(:h, token_hash),
				token_hint = coalesce(:hint, token_hint),
				last_registered_at = case when :tok::text is null then last_registered_at else now() end,
				status = case when :tok::text is null then status else 'active' end,
				inactive_reason = case when :tok::text is null then inactive_reason end,
				deactivated_at = case when :tok::text is null then deactivated_at end
			where id = :id`, args); err != nil {
			return err
		}
		after, err := c.loadDevice(uid, before.ID, nil)
		if err != nil {
			return err
		}
		if before.Status == "inactive" && after.Status == "active" {
			a := after.ApplicationID
			if err := c.emit("device.registered", &a, map[string]any{"device": after}); err != nil {
				return err
			}
			if err := c.enforceDeviceCap(uid, after.ApplicationID); err != nil {
				return err
			}
		}
		c.Versioned(after.version)
		return c.OK(after)
	})
}

func (s *Server) DevicesDelete(w http.ResponseWriter, r *http.Request, id gen.UserIdOrMe, deviceID string) {
	s.serve(w, r, opts{op: "devices.delete"}, func(c *Call) error {
		uid, app, err := c.deviceUser(id, false, true)
		if err != nil {
			return err
		}
		d, err := c.loadDevice(uid, deviceID, app)
		if err != nil {
			return err
		}
		if err := c.deleteDevice(d); err != nil {
			return err
		}
		if app == nil && !c.isSelf(uid) {
			devApp := d.ApplicationID
			if err := c.audit(Audit{Action: "device.deleted", TargetType: "device", TargetID: d.ID, TargetUserID: &uid,
				ApplicationID: &devApp, Before: d}); err != nil {
				return err
			}
		}
		return c.NoContent()
	})
}

// testNotificationTitle is a device test's default title.
const testNotificationTitle = "Test notification from Substratal"

func (s *Server) DevicesSendTest(w http.ResponseWriter, r *http.Request, id gen.UserIdOrMe, deviceID string) {
	s.serve(w, r, opts{op: "devices.sendTest"}, func(c *Call) error {
		uid, app, err := c.deviceUser(id, true, false)
		if err != nil {
			return err
		}
		d, err := c.loadDevice(uid, deviceID, app)
		if err != nil {
			return err
		}
		var in struct {
			Title *string `json:"title"`
			Body  *string `json:"body"`
		}
		if err := c.Decode(&in); err != nil {
			return err
		}
		var f httpx.Fields
		title := testNotificationTitle
		if in.Title != nil {
			title = *in.Title
			if n := len([]rune(title)); n < 1 || n > 120 {
				f.Max("title", "too_long", 120)
			}
		}
		if in.Body != nil && len([]rune(*in.Body)) > 1000 {
			f.Max("body", "too_long", 1000)
		}
		if err := f.Err(); err != nil {
			return err
		}
		if err := c.hit("push-test:"+d.ID, perHour(10), false); err != nil {
			return err
		}
		cred, err := c.loadCredential(d.ApplicationID, d.Provider)
		if err != nil {
			if isAPIError(err) {
				return httpx.Conflict("push_not_configured", "The Application has no "+d.Provider+" credential.")
			}
			return err
		}
		pc, err := c.s.decryptCredential(c.ctx, cred)
		if err != nil {
			return err
		}
		target, err := c.s.decryptTarget(c.ctx, d)
		if err != nil {
			return err
		}
		appID := d.ApplicationID
		msg := push.Message{Credential: pc, Target: target, Notification: push.Notification{
			ApplicationID: &appID, Source: "application", Category: "test", Priority: "normal",
			Title: title, Body: in.Body, TTL: 0}}
		ctx, cancel := context.WithTimeout(c.ctx, 10*time.Second)
		res := c.s.pushSender().Send(ctx, msg)
		cancel()
		if err := c.applyOutcome(d, cred.ID, res); err != nil {
			return err
		}
		status := "failed"
		if res.Outcome == push.Sent {
			status = "sent"
		}
		var errText *string
		if res.Reason != "" && res.Outcome != push.Sent {
			errText = &res.Reason
		}
		var provStatus *int
		if res.Status != 0 {
			provStatus = &res.Status
		}
		return c.OK(map[string]any{"device_id": d.ID, "status": status, "provider_status": provStatus,
			"error": errText, "duration_ms": res.Duration.Milliseconds()})
	})
}

// decryptTarget rebuilds a device's provider target.
func (s *Server) decryptTarget(ctx context.Context, d device) (push.Target, error) {
	plain, err := s.Encrypter.Decrypt(ctx, d.tokenCiphertext)
	if err != nil {
		return push.Target{}, err
	}
	t := push.Target{Provider: d.Provider, Environment: deref(d.Environment)}
	if d.Provider == push.WebPush {
		var sub push.Subscription
		if err := json.Unmarshal([]byte(plain), &sub); err != nil {
			return t, err
		}
		t.Subscription = &sub
	} else {
		t.Token = plain
	}
	return t, nil
}

// applyOutcome records what a provider's answer means for the device and
// the credential: a gone token deactivates the device, a rejected
// credential is marked failed, and a success stamps both.
func (c *Call) applyOutcome(d device, credID string, res push.Result) error {
	switch res.Outcome {
	case push.Sent:
		if _, err := c.q.Exec(c.ctx, `update devices set last_delivery_at = now() where id = :d`, db.Args{"d": d.ID}); err != nil {
			return err
		}
		_, err := c.q.Exec(c.ctx, `update push_credentials set last_used_at = now() where id = :id`, db.Args{"id": credID})
		return err
	case push.TokenGone:
		return c.deactivateDevices("d.id = :id", db.Args{"id": d.ID}, "unregistered")
	case push.CredentialRejected:
		reason := res.Reason
		if reason == "" {
			reason = "rejected"
		}
		return c.credentialRejected(credID, reason)
	}
	return nil
}
