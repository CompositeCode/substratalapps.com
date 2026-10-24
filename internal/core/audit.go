package core

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/CompositeCode/substratalapps.com/internal/api/gen"
	"github.com/CompositeCode/substratalapps.com/internal/db"
	"github.com/CompositeCode/substratalapps.com/internal/httpx"
)

type auditEvent struct {
	ID             string          `json:"id"`
	Action         string          `json:"action"`
	Actor          actorRef        `json:"actor"`
	Target         actorRef        `json:"target"`
	TargetUserID   *string         `json:"target_user_id"`
	ApplicationID  *string         `json:"application_id"`
	OrganizationID *string         `json:"organization_id"`
	TenantID       *string         `json:"tenant_id"`
	Before         json.RawMessage `json:"before"`
	After          json.RawMessage `json:"after"`
	RequestID      *string         `json:"request_id"`
	Timestamp      time.Time       `json:"timestamp"`
}

const auditSQL = `select jsonb_build_object('id', a.id, 'action', a.action,
	'actor', jsonb_build_object('type', a.actor_type, 'id', a.actor_id),
	'target', jsonb_build_object('type', a.target_type, 'id', a.target_id),
	'target_user_id', a.target_user_id, 'application_id', a.application_id, 'organization_id', a.organization_id,
	'tenant_id', a.tenant_id, 'before', a.before, 'after', a.after, 'request_id', a.request_id,
	'timestamp', a.timestamp) from audit_events a`

// ownsTenant: the owner_user_id User, or an org_admin of the owning Organization.
func (c *Call) ownsTenant(t tenantRow) (bool, error) {
	if c.p == nil || !c.p.IsUser() {
		return false, nil
	}
	if t.OwnerUserID != nil {
		return *t.OwnerUserID == c.p.UserID, nil
	}
	return c.isOrgAdmin(*t.OwnerOrganizationID)
}

// auditForced is the scope a caller without audit.view is confined to: at
// most one field is set, and none for audit.view.
type auditForced struct {
	ApplicationID string `json:"application_id,omitempty"`
	TargetUserID  string `json:"target_user_id,omitempty"`
	TenantID      string `json:"tenant_id,omitempty"`
}

// auditScope returns the forced scope for the caller, or 403.
func (c *Call) auditScope(targetUser, tenant *string) (auditForced, error) {
	if c.can("audit.view") {
		return auditForced{}, nil
	}
	if app, ok := c.p.AppKey(); ok {
		if !c.p.Perms["audit.view"] {
			return auditForced{}, httpx.Forbidden("audit.view")
		}
		return auditForced{ApplicationID: app}, nil
	}
	if c.p.IsUser() && targetUser != nil && *targetUser == c.p.UserID {
		return auditForced{TargetUserID: c.p.UserID}, nil
	}
	if tenant != nil {
		t, err := c.loadTenant(*tenant)
		if err == nil {
			if owner, err := c.ownsTenant(t); err != nil {
				return auditForced{}, err
			} else if owner {
				return auditForced{TenantID: t.ID}, nil
			}
		}
	}
	return auditForced{}, httpx.Forbidden("audit.view")
}

// auditFilters are GET /audit-events' filters, and an export's.
type auditFilters struct {
	TargetUserID   *string    `json:"target_user_id,omitempty"`
	TargetType     *string    `json:"target_type,omitempty"`
	TargetID       *string    `json:"target_id,omitempty"`
	ActorID        *string    `json:"actor_id,omitempty"`
	ActorType      *string    `json:"actor_type,omitempty"`
	Action         []string   `json:"action,omitempty"`
	ApplicationID  *string    `json:"application_id,omitempty"`
	OrganizationID *string    `json:"organization_id,omitempty"`
	TenantID       *string    `json:"tenant_id,omitempty"`
	Since          *time.Time `json:"since,omitempty"`
	Until          *time.Time `json:"until,omitempty"`
}

// auditWhere is the where clause for a forced scope and filters.
func auditWhere(forced auditForced, f auditFilters) ([]string, db.Args) {
	var where []string
	args := db.Args{}
	add := func(cond, key string, v *string) {
		if v != nil && *v != "" {
			where = append(where, cond)
			args[key] = *v
		}
	}
	add("a.application_id = :forced_app", "forced_app", &forced.ApplicationID)
	add("a.target_user_id = :forced_user", "forced_user", &forced.TargetUserID)
	add("a.tenant_id = :forced_tenant", "forced_tenant", &forced.TenantID)
	add("a.target_user_id = :tu", "tu", f.TargetUserID)
	add("a.target_type = :tt", "tt", f.TargetType)
	add("a.target_id = :ti", "ti", f.TargetID)
	add("a.actor_id = :ai", "ai", f.ActorID)
	add("a.actor_type = :at", "at", f.ActorType)
	add("a.application_id = :app", "app", f.ApplicationID)
	add("a.organization_id = :org", "org", f.OrganizationID)
	add("a.tenant_id = :tnt", "tnt", f.TenantID)
	if len(f.Action) > 0 {
		where = append(where, "a.action = any(:acts::text[])")
		args["acts"] = db.TextArray(f.Action)
	}
	if f.Since != nil {
		where = append(where, "a.timestamp >= :since::timestamptz")
		args["since"] = db.Time(*f.Since)
	}
	if f.Until != nil {
		where = append(where, "a.timestamp < :until::timestamptz")
		args["until"] = db.Time(*f.Until)
	}
	return where, args
}

func (s *Server) AuditList(w http.ResponseWriter, r *http.Request, params gen.AuditListParams) {
	s.serve(w, r, opts{op: "audit.list"}, func(c *Call) error {
		f := auditFilters{TargetType: enumStr(params.TargetType), TargetID: params.TargetId, ActorID: params.ActorId,
			ActorType: enumStr(params.ActorType), ApplicationID: params.ApplicationId, OrganizationID: params.OrganizationId,
			TenantID: params.TenantId, Since: params.Since, Until: params.Until}
		if params.TargetUserId != nil {
			uid, err := c.selfID(*params.TargetUserId)
			if err != nil {
				return err
			}
			f.TargetUserID = &uid
		}
		if params.Action != nil {
			for _, a := range *params.Action {
				f.Action = append(f.Action, string(a))
			}
		}
		forced, err := c.auditScope(f.TargetUserID, params.TenantId)
		if err != nil {
			return err
		}
		where, args := auditWhere(forced, f)
		lim := httpx.Limit(params.Limit)
		args["lim"] = int64(lim + 1)
		filters := httpx.Filters(c.r)
		pos, err := c.s.Cursors.Decode(deref(params.Cursor), filters)
		if err != nil {
			return err
		}
		total, err := c.listTotal("audit_events a", where, args)
		if err != nil {
			return err
		}
		if pos != nil {
			where = append(where, "(a.timestamp, a.id) < (:ps::timestamptz, :pi)")
			args["ps"], args["pi"] = pos.Sort, pos.ID
		}
		if len(where) == 0 {
			where = []string{"true"}
		}
		rows, err := db.All[auditEvent](c.ctx, c.q, auditSQL+" where "+strings.Join(where, " and ")+
			" order by a.timestamp desc, a.id desc limit :lim", args)
		if err != nil {
			return err
		}
		if params.TenantId != nil {
			c.headers.Set("Audit-Hot-Window-Start", c.now.AddDate(0, 0, -AuditHotWindowDays).Format(time.RFC3339))
		}
		return c.OK(httpx.Paginate(c.s.Cursors, rows, lim, filters, func(x auditEvent) (string, string) {
			return httpx.TimeKey(x.Timestamp), x.ID
		}).WithTotal(total))
	})
}

func (s *Server) AuditGet(w http.ResponseWriter, r *http.Request, id string) {
	s.serve(w, r, opts{op: "audit.get"}, func(c *Call) error {
		var ev auditEvent
		if err := db.One(c.ctx, c.q, &ev, auditSQL+" where a.id = :id", db.Args{"id": id}); err == db.ErrNotFound {
			return httpx.NotFoundResource("audit_event")
		} else if err != nil {
			return err
		}
		forced, err := c.auditScope(ev.TargetUserID, ev.TenantID)
		if err != nil {
			return httpx.NotFoundResource("audit_event")
		}
		if where, args := auditWhere(forced, auditFilters{}); len(where) > 0 {
			args["id"] = id
			rows, err := c.q.Query(c.ctx, `select to_jsonb(a.id) from audit_events a where a.id = :id and `+
				strings.Join(where, " and "), args)
			if err != nil {
				return err
			}
			if len(rows) == 0 {
				return httpx.NotFoundResource("audit_event")
			}
		}
		return c.OK(ev)
	})
}
