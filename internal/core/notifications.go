package core

import (
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/CompositeCode/substratalapps.com/internal/api/gen"
	"github.com/CompositeCode/substratalapps.com/internal/db"
	"github.com/CompositeCode/substratalapps.com/internal/httpx"
	"github.com/CompositeCode/substratalapps.com/internal/ids"
	"github.com/CompositeCode/substratalapps.com/internal/push"
)

// recipientHourlyLimit caps what one Application can send one User in a
// rolling hour (NFR → Rate limiting). Platform alerts don't count.
const recipientHourlyLimit = 60

// notification is the API view of a Notification.
type notification struct {
	ID            string                  `json:"id"`
	UserID        string                  `json:"user_id"`
	ApplicationID *string                 `json:"application_id"`
	Source        string                  `json:"source"`
	Category      string                  `json:"category"`
	Priority      string                  `json:"priority"`
	Title         string                  `json:"title"`
	Body          *string                 `json:"body"`
	Data          map[string]string       `json:"data"`
	ActionURL     *string                 `json:"action_url"`
	Badge         *int                    `json:"badge"`
	Sound         *string                 `json:"sound"`
	CollapseKey   *string                 `json:"collapse_key"`
	TTLSeconds    int                     `json:"ttl_seconds"`
	Push          json.RawMessage         `json:"push"`
	State         string                  `json:"state"`
	ReadAt        *time.Time              `json:"read_at"`
	ArchivedAt    *time.Time              `json:"archived_at"`
	Sender        json.RawMessage         `json:"sender"`
	TestMode      bool                    `json:"test_mode"`
	CreatedAt     time.Time               `json:"created_at"`
	UpdatedAt     time.Time               `json:"updated_at"`
	Deliveries    *[]notificationDelivery `json:"deliveries,omitempty"`
	version       int
}

type notificationDelivery struct {
	ID       string     `json:"id"`
	DeviceID *string    `json:"device_id"`
	Provider string     `json:"provider"`
	Status   string     `json:"status"`
	Attempts int        `json:"attempts"`
	Error    *string    `json:"error"`
	SentAt   *time.Time `json:"sent_at"`
}

const notificationSQL = `select jsonb_build_object('id', n.id, 'user_id', n.user_id, 'application_id', n.application_id,
	'source', n.source, 'category', n.category, 'priority', n.priority, 'title', n.title, 'body', n.body, 'data', n.data,
	'action_url', n.action_url, 'badge', n.badge, 'sound', n.sound, 'collapse_key', n.collapse_key,
	'ttl_seconds', n.ttl_seconds, 'push', ` + notificationPushJSON + `,
	'state', case when n.archived_at is not null then 'archived' when n.read_at is not null then 'read' else 'unread' end,
	'read_at', n.read_at, 'archived_at', n.archived_at,
	'sender', jsonb_build_object('type', n.sender_type, 'id', n.sender_id),
	'test_mode', n.test_mode, 'created_at', n.created_at, 'updated_at', n.updated_at, 'version_', n.version)
	from notifications n`

func (c *Call) queryNotifications(where string, args db.Args) ([]notification, error) {
	rows, err := c.q.Query(c.ctx, notificationSQL+" where "+where, args)
	if err != nil {
		return nil, err
	}
	out := make([]notification, 0, len(rows))
	for _, raw := range rows {
		var x struct {
			notification
			V int `json:"version_"`
		}
		if err := json.Unmarshal(raw, &x); err != nil {
			return nil, err
		}
		x.notification.version = x.V
		if x.notification.Data == nil {
			x.notification.Data = map[string]string{}
		}
		out = append(out, x.notification)
	}
	return out, nil
}

// inboxUser authorizes inbox access for the User in {id}: the User
// themself (everything), the app's own key (its own Application's
// notifications, never platform alerts, for Users with an access path to
// it), and support with users.list to read.
func (c *Call) inboxUser(id string, write bool) (string, *string, error) {
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
	if !write && (c.can("users.list") || c.can("users.manage")) {
		return uid, nil, nil
	}
	if write {
		return "", nil, httpx.Forbidden("")
	}
	return "", nil, httpx.Forbidden("users.list")
}

func (c *Call) loadNotification(uid, notificationID string, app *string) (notification, error) {
	where, args := "n.id = :id and n.user_id = :u", db.Args{"id": notificationID, "u": uid}
	if app != nil {
		where += " and n.application_id = :app"
		args["app"] = *app
	}
	ns, err := c.queryNotifications(where, args)
	if err != nil {
		return notification{}, err
	}
	if len(ns) == 0 {
		return notification{}, httpx.NotFoundResource("notification")
	}
	return ns[0], nil
}

var (
	categoryRe    = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z0-9_]+)*$`)
	collapseKeyRe = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
	// A private-use URI scheme for native apps is reverse-DNS
	// (com.example.app:/path), the same rule as redirect_uris.
	privateSchemeRe = regexp.MustCompile(`^[a-z][a-z0-9+-]*(\.[a-z0-9+-]+)+$`)
)

func reservedCategory(cat string) bool {
	return strings.HasPrefix(cat, "security.") || strings.HasPrefix(cat, "account.") || strings.HasPrefix(cat, "billing.")
}

func validActionURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	if u.Scheme == "https" {
		return u.Host != ""
	}
	return privateSchemeRe.MatchString(u.Scheme)
}

func (s *Server) NotificationsSend(w http.ResponseWriter, r *http.Request, id gen.UserIdOrMe, params gen.NotificationsSendParams) {
	s.serve(w, r, opts{op: "notifications.send", idem: idemOptional}, func(c *Call) error {
		uid, err := c.selfID(id)
		if err != nil {
			return err
		}
		raw, err := c.Patch()
		if err != nil {
			return err
		}
		var in struct {
			Title         *string           `json:"title"`
			Body          *string           `json:"body"`
			Category      *string           `json:"category"`
			Priority      *string           `json:"priority"`
			Push          *bool             `json:"push"`
			Data          map[string]string `json:"data"`
			ActionURL     *string           `json:"action_url"`
			Badge         *int              `json:"badge"`
			Sound         *string           `json:"sound"`
			CollapseKey   *string           `json:"collapse_key"`
			TTLSeconds    *int              `json:"ttl_seconds"`
			ApplicationID *string           `json:"application_id"`
		}
		if err := c.Decode(&in); err != nil {
			return err
		}
		// Who's sending, for which Application.
		var appID string
		var f httpx.Fields
		if keyApp, ok := c.p.AppKey(); ok {
			if !c.appKeyCan("notifications.send", keyApp) {
				return httpx.Forbidden("notifications.send")
			}
			if in.ApplicationID != nil && *in.ApplicationID != keyApp {
				return httpx.Forbidden("")
			}
			appID = keyApp
		} else if c.can("notifications.send") {
			if in.ApplicationID == nil || *in.ApplicationID == "" {
				f.Add("application_id", "required")
			} else {
				appID = *in.ApplicationID
			}
		} else {
			return httpx.Forbidden("notifications.send")
		}
		if _, err := c.visibleUser(uid); err != nil {
			return err
		}
		sp := notificationSpec{UserID: uid, Priority: "normal", TTL: 86400, Push: true, Data: in.Data,
			Body: in.Body, ActionURL: in.ActionURL, Badge: in.Badge, CollapseKey: in.CollapseKey,
			SenderType: c.p.Type, SenderID: c.p.ID}
		switch {
		case in.Title == nil || *in.Title == "":
			f.Add("title", "required")
		case utf8.RuneCountInString(*in.Title) > 120:
			f.Max("title", "too_long", 120)
		default:
			sp.Title = *in.Title
		}
		if in.Body != nil && utf8.RuneCountInString(*in.Body) > 1000 {
			f.Max("body", "too_long", 1000)
		}
		reserved := false
		switch {
		case in.Category == nil || *in.Category == "":
			f.Add("category", "required")
		case len(*in.Category) > 64:
			f.Max("category", "too_long", 64)
		case !categoryRe.MatchString(*in.Category):
			f.Add("category", "invalid_format").Pattern = categoryRe.String()
		case reservedCategory(*in.Category):
			reserved = true
		default:
			sp.Category = *in.Category
		}
		if in.Priority != nil {
			if *in.Priority != "normal" && *in.Priority != "high" {
				f.Enum("priority", "normal", "high")
			} else {
				sp.Priority = *in.Priority
			}
		}
		if in.Push != nil {
			sp.Push = *in.Push
		}
		if len(in.Data) > 20 {
			f.Max("data", "too_long", 20)
		}
		if _, ok := in.Data["substratal"]; ok {
			f.Add("data.substratal", "unknown_value").Message = "the key substratal is reserved"
		}
		if b, _ := json.Marshal(in.Data); len(b) > 2048 {
			f.Max("data", "too_long", 2048)
		}
		if in.ActionURL != nil && (len(*in.ActionURL) > 2048 || !validActionURL(*in.ActionURL)) {
			f.Add("action_url", "invalid_format").Allowed = []string{"https", "reverse-DNS private-use scheme"}
		}
		if in.Badge != nil && (*in.Badge < 0 || *in.Badge > 99999) {
			f.Add("badge", "out_of_range").Max = ip(99999)
		}
		if raw.Has("sound") {
			if in.Sound != nil && *in.Sound != "default" {
				f.Enum("sound", "default")
			}
			sp.Sound = in.Sound
		} else if sp.Priority == "high" {
			sp.Sound = strp("default")
		}
		if in.CollapseKey != nil && (len(*in.CollapseKey) > 64 || !collapseKeyRe.MatchString(*in.CollapseKey)) {
			f.Add("collapse_key", "invalid_format").Pattern = collapseKeyRe.String()
		}
		if in.TTLSeconds != nil {
			if *in.TTLSeconds < 0 || *in.TTLSeconds > 2419200 {
				f.Add("ttl_seconds", "out_of_range").Max = ip(2419200)
			} else {
				sp.TTL = *in.TTLSeconds
			}
		}
		if err := f.Err(); err != nil {
			return err
		}
		if reserved {
			return httpx.E(422, "reserved_category", "Categories under security., account., and billing. are reserved for platform alerts.").
				With("category", *in.Category)
		}
		a, err := c.loadApp(appID)
		if err == db.ErrNotFound {
			f.Add("application_id", "unknown_value")
			return f.Err()
		}
		if err != nil {
			return err
		}
		sp.AppID = &a.ID
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
		if !c.s.DisableRateLimits {
			var n int
			if err := db.One(c.ctx, c.q, &n, `select to_jsonb(count(*)) from notifications
				where user_id = :u and application_id = :a and created_at > now() - interval '1 hour'`,
				db.Args{"u": uid, "a": a.ID}); err != nil {
				return err
			}
			if n >= recipientHourlyLimit {
				return httpx.E(429, "rate_limited", "This User has received the hourly limit of notifications from this Application.").
					With("scope", "recipient").RetryAfter(60)
			}
		}
		nid := ids.New(ids.Notification)
		if size := push.Size(sp.pushNotification(nid)); size > push.MaxPayload {
			return httpx.E(422, "notification_too_large", "The push payload would exceed 4 KB.").
				With("bytes", size).With("max", push.MaxPayload)
		}
		if err := c.createNotification(nid, sp); err != nil {
			return err
		}
		out, err := c.loadNotification(uid, nid, nil)
		if err != nil {
			return err
		}
		c.Versioned(out.version)
		return c.Created(out)
	})
}

func (s *Server) NotificationsList(w http.ResponseWriter, r *http.Request, id gen.UserIdOrMe, params gen.NotificationsListParams) {
	s.serve(w, r, opts{op: "notifications.list"}, func(c *Call) error {
		uid, app, err := c.inboxUser(id, false)
		if err != nil {
			return err
		}
		where, args := []string{"n.user_id = :u"}, db.Args{"u": uid}
		if app != nil {
			where = append(where, "n.application_id = :app")
			args["app"] = *app
		}
		if params.State != nil {
			switch *params.State {
			case gen.Unread:
				where = append(where, "n.read_at is null and n.archived_at is null")
			case gen.Read:
				where = append(where, "n.read_at is not null and n.archived_at is null")
			case gen.Archived:
				where = append(where, "n.archived_at is not null")
			}
		} else if params.IncludeInactive == nil || !*params.IncludeInactive {
			where = append(where, "n.archived_at is null")
		}
		if params.ApplicationId != nil {
			where = append(where, "n.application_id = :fapp")
			args["fapp"] = *params.ApplicationId
		}
		if params.Source != nil {
			where = append(where, "n.source = :src")
			args["src"] = string(*params.Source)
		}
		if params.Category != nil {
			where = append(where, "n.category = :cat")
			args["cat"] = *params.Category
		}
		if params.Priority != nil {
			where = append(where, "n.priority = :pri")
			args["pri"] = string(*params.Priority)
		}
		if params.CreatedAfter != nil {
			where = append(where, "n.created_at > :after::timestamptz")
			args["after"] = db.Time(*params.CreatedAfter)
		}
		if params.CreatedBefore != nil {
			where = append(where, "n.created_at < :before::timestamptz")
			args["before"] = db.Time(*params.CreatedBefore)
		}
		lim := httpx.Limit(params.Limit)
		args["lim"] = int64(lim + 1)
		filters := httpx.Filters(c.r)
		pos, err := c.s.Cursors.Decode(deref(params.Cursor), filters)
		if err != nil {
			return err
		}
		if pos != nil {
			where = append(where, "(n.created_at, n.id) < (:ps::timestamptz, :pi)")
			args["ps"], args["pi"] = pos.Sort, pos.ID
		}
		ns, err := c.queryNotifications(strings.Join(where, " and ")+" order by n.created_at desc, n.id desc limit :lim", args)
		if err != nil {
			return err
		}
		return c.OK(httpx.Paginate(c.s.Cursors, ns, lim, filters, func(x notification) (string, string) {
			return httpx.TimeKey(x.CreatedAt), x.ID
		}))
	})
}

func (s *Server) NotificationsGet(w http.ResponseWriter, r *http.Request, id gen.UserIdOrMe, notificationID string) {
	s.serve(w, r, opts{op: "notifications.get"}, func(c *Call) error {
		uid, app, err := c.inboxUser(id, false)
		if err != nil {
			return err
		}
		n, err := c.loadNotification(uid, notificationID, app)
		if err != nil {
			return err
		}
		ds, err := db.All[notificationDelivery](c.ctx, c.q, `select jsonb_build_object('id', id, 'device_id', device_id,
				'provider', provider, 'status', status, 'attempts', attempts, 'error', error, 'sent_at', sent_at)
			from notification_deliveries where notification_id = :n order by created_at, id`, db.Args{"n": n.ID})
		if err != nil {
			return err
		}
		n.Deliveries = &ds
		c.Versioned(n.version)
		return c.OK(n)
	})
}

func (s *Server) NotificationsUpdate(w http.ResponseWriter, r *http.Request, id gen.UserIdOrMe, notificationID string, params gen.NotificationsUpdateParams) {
	s.serve(w, r, opts{op: "notifications.update"}, func(c *Call) error {
		uid, app, err := c.inboxUser(id, true)
		if err != nil {
			return err
		}
		before, err := c.loadNotification(uid, notificationID, app)
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
		for k := range p {
			if k != "state" {
				return httpx.ReadOnly(k)
			}
		}
		var state string
		var f httpx.Fields
		if !p.Get("state", &state, &f) {
			if err := f.Err(); err != nil {
				return err
			}
			f.Add("state", "required")
			return f.Err()
		}
		var set string
		switch state {
		case "read":
			set = "read_at = coalesce(read_at, now()), archived_at = null"
		case "unread":
			set = "read_at = null, archived_at = null"
		case "archived":
			set = "archived_at = coalesce(archived_at, now()), read_at = coalesce(read_at, now())"
		default:
			f.Enum("state", "unread", "read", "archived")
			return f.Err()
		}
		if _, err := c.q.Exec(c.ctx, `update notifications set `+set+` where id = :id`, db.Args{"id": before.ID}); err != nil {
			return err
		}
		after, err := c.loadNotification(uid, before.ID, nil)
		if err != nil {
			return err
		}
		c.Versioned(after.version)
		return c.OK(after)
	})
}

func (s *Server) NotificationsMarkRead(w http.ResponseWriter, r *http.Request, id gen.UserIdOrMe) {
	s.serve(w, r, opts{op: "notifications.markRead"}, func(c *Call) error {
		uid, app, err := c.inboxUser(id, true)
		if err != nil {
			return err
		}
		var in struct {
			ApplicationID *string    `json:"application_id"`
			Source        *string    `json:"source"`
			Category      *string    `json:"category"`
			Before        *time.Time `json:"before"`
		}
		if err := c.Decode(&in); err != nil {
			return err
		}
		where, args := []string{"user_id = :u", "read_at is null", "archived_at is null"}, db.Args{"u": uid}
		if app != nil {
			where = append(where, "application_id = :app")
			args["app"] = *app
		}
		if in.ApplicationID != nil {
			where = append(where, "application_id = :fapp")
			args["fapp"] = *in.ApplicationID
		}
		if in.Source != nil {
			if *in.Source != "application" && *in.Source != "platform" {
				var f httpx.Fields
				f.Enum("source", "application", "platform")
				return f.Err()
			}
			where = append(where, "source = :src")
			args["src"] = *in.Source
		}
		if in.Category != nil {
			where = append(where, "category = :cat")
			args["cat"] = *in.Category
		}
		if in.Before != nil {
			where = append(where, "created_at <= :before::timestamptz")
			args["before"] = db.Time(*in.Before)
		}
		n, err := c.q.Exec(c.ctx, `update notifications set read_at = now() where `+strings.Join(where, " and "), args)
		if err != nil {
			return err
		}
		return c.OK(map[string]any{"updated_count": n})
	})
}

func (s *Server) NotificationsUnreadCount(w http.ResponseWriter, r *http.Request, id gen.UserIdOrMe, params gen.NotificationsUnreadCountParams) {
	s.serve(w, r, opts{op: "notifications.unreadCount"}, func(c *Call) error {
		uid, app, err := c.inboxUser(id, true)
		if err != nil {
			return err
		}
		where, args := []string{"user_id = :u", "read_at is null", "archived_at is null"}, db.Args{"u": uid}
		if app != nil {
			where = append(where, "application_id = :app")
			args["app"] = *app
		}
		if params.ApplicationId != nil {
			where = append(where, "application_id = :fapp")
			args["fapp"] = *params.ApplicationId
		}
		if params.Source != nil {
			where = append(where, "source = :src")
			args["src"] = string(*params.Source)
		}
		type group struct {
			ApplicationID *string `json:"application_id"`
			UnreadCount   int     `json:"unread_count"`
		}
		groups, err := db.All[group](c.ctx, c.q, `select jsonb_build_object('application_id', application_id, 'unread_count', count(*))
			from notifications where `+strings.Join(where, " and ")+` group by application_id order by application_id nulls last`, args)
		if err != nil {
			return err
		}
		total := 0
		for _, g := range groups {
			total += g.UnreadCount
		}
		return c.OK(map[string]any{"unread_count": total, "applications": groups})
	})
}
