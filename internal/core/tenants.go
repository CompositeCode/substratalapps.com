package core

import (
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/CompositeCode/substratalapps.com/internal/api/gen"
	"github.com/CompositeCode/substratalapps.com/internal/db"
	"github.com/CompositeCode/substratalapps.com/internal/httpx"
	"github.com/CompositeCode/substratalapps.com/internal/ids"
)

type tenant struct {
	ID                  string    `json:"id"`
	OwnerType           string    `json:"owner_type"`
	OwnerUserID         *string   `json:"owner_user_id"`
	OwnerOrganizationID *string   `json:"owner_organization_id"`
	Tier                string    `json:"tier"`
	Region              string    `json:"region"`
	Status              string    `json:"status"`
	ApplicationCount    int       `json:"application_count"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

func (c *Call) tenantObject(t tenantRow) (tenant, error) {
	var n int
	if err := db.One(c.ctx, c.q, &n, `select to_jsonb(count(*)) from applications where tenant_id = :t`, db.Args{"t": t.ID}); err != nil {
		return tenant{}, err
	}
	return tenant{ID: t.ID, OwnerType: t.OwnerType, OwnerUserID: t.OwnerUserID, OwnerOrganizationID: t.OwnerOrganizationID,
		Tier: t.Tier, Region: t.Region, Status: t.Status, ApplicationCount: n, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt}, nil
}

// visibleTenant: owners, tenants.manage, and any extra permission given.
func (c *Call) visibleTenant(id string, extra ...string) (tenantRow, bool, error) {
	t, err := c.loadTenant(id)
	if err != nil {
		return t, false, err
	}
	owner, err := c.ownsTenant(t)
	if err != nil {
		return t, false, err
	}
	if owner || c.can("tenants.manage") {
		return t, owner, nil
	}
	for _, p := range extra {
		if c.can(p) {
			return t, false, nil
		}
	}
	return t, false, httpx.NotFoundResource("tenant")
}

func (s *Server) TenantsList(w http.ResponseWriter, r *http.Request, params gen.TenantsListParams) {
	s.serve(w, r, opts{op: "tenants.list"}, func(c *Call) error {
		where, args := []string{"true"}, db.Args{}
		if !c.can("tenants.manage") {
			if !c.p.IsUser() {
				return c.OK(httpx.List[tenant]{Data: []tenant{}})
			}
			where = append(where, `(t.owner_user_id = :me or t.owner_organization_id in (select organization_id
				from organization_memberships where user_id = :me and role = 'org_admin' and status = 'active'))`)
			args["me"] = c.p.UserID
		} else {
			add := func(cond, key string, v *string) {
				if v != nil {
					where = append(where, cond)
					args[key] = *v
				}
			}
			add("t.tier = :tier", "tier", enumStr(params.Tier))
			add("t.status = :st", "st", enumStr(params.Status))
			add("t.owner_user_id = :ou", "ou", params.OwnerUserId)
			add("t.owner_organization_id = :oo", "oo", params.OwnerOrganizationId)
		}
		lim := httpx.Limit(params.Limit)
		args["lim"] = int64(lim + 1)
		filters := httpx.Filters(c.r)
		pos, err := c.s.Cursors.Decode(deref(params.Cursor), filters)
		if err != nil {
			return err
		}
		if pos != nil {
			where = append(where, "(t.created_at, t.id) < (:ps::timestamptz, :pi)")
			args["ps"], args["pi"] = pos.Sort, pos.ID
		}
		rows, err := db.All[tenantRow](c.ctx, c.q, `select to_jsonb(t) from tenants t where `+strings.Join(where, " and ")+
			` order by t.created_at desc, t.id desc limit :lim`, args)
		if err != nil {
			return err
		}
		out := make([]tenant, 0, len(rows))
		for _, t := range rows {
			obj, err := c.tenantObject(t)
			if err != nil {
				return err
			}
			out = append(out, obj)
		}
		return c.OK(httpx.Paginate(c.s.Cursors, out, lim, filters, func(t tenant) (string, string) {
			return httpx.TimeKey(t.CreatedAt), t.ID
		}))
	})
}

func (s *Server) TenantsGet(w http.ResponseWriter, r *http.Request, id string) {
	s.serve(w, r, opts{op: "tenants.get"}, func(c *Call) error {
		t, _, err := c.visibleTenant(id)
		if err != nil {
			return err
		}
		obj, err := c.tenantObject(t)
		if err != nil {
			return err
		}
		return c.OK(obj)
	})
}

// ── Tier-change requests ───────────────────────────────────────────────

type tierChange struct {
	ID              string     `json:"id"`
	TenantID        string     `json:"tenant_id"`
	FromTier        string     `json:"from_tier"`
	FromRegion      string     `json:"from_region"`
	RequestedTier   string     `json:"requested_tier"`
	RequestedRegion *string    `json:"requested_region"`
	Status          string     `json:"status"`
	Reason          string     `json:"reason"`
	Notes           *string    `json:"notes"`
	ScheduledFor    *time.Time `json:"scheduled_for"`
	RequestedBy     string     `json:"requested_by"`
	CreatedAt       time.Time  `json:"created_at"`
	StartedAt       *time.Time `json:"started_at"`
	CompletedAt     *time.Time `json:"completed_at"`
	Version         int        `json:"-"`
}

var supportedRegions = []string{"us-east-1", "us-west-2", "ca-central-1", "eu-west-1", "eu-central-1", "ap-southeast-2"}

var tierRank = map[string]int{"shared": 0, "isolated": 1, "dedicated_region": 2}

func (c *Call) loadTierChange(tenantID, id string) (tierChange, error) {
	var x struct {
		tierChange
		V int `json:"version"`
	}
	err := db.One(c.ctx, c.q, &x, `select to_jsonb(r) from tier_change_requests r where id = :id and tenant_id = :t`,
		db.Args{"id": id, "t": tenantID})
	if err == db.ErrNotFound {
		return tierChange{}, httpx.NotFoundResource("tier_change_request")
	}
	x.tierChange.Version = x.V
	return x.tierChange, err
}

func (s *Server) TenantsListTierChangeRequests(w http.ResponseWriter, r *http.Request, id string, params gen.TenantsListTierChangeRequestsParams) {
	s.serve(w, r, opts{op: "tenants.listTierChangeRequests"}, func(c *Call) error {
		if _, _, err := c.visibleTenant(id); err != nil {
			return err
		}
		where, args := []string{"r.tenant_id = :t"}, db.Args{"t": id}
		lim := httpx.Limit(params.Limit)
		args["lim"] = int64(lim + 1)
		filters := httpx.Filters(c.r)
		pos, err := c.s.Cursors.Decode(deref(params.Cursor), filters)
		if err != nil {
			return err
		}
		if pos != nil {
			where = append(where, "(r.created_at, r.id) < (:ps::timestamptz, :pi)")
			args["ps"], args["pi"] = pos.Sort, pos.ID
		}
		rows, err := db.All[tierChange](c.ctx, c.q, `select to_jsonb(r) from tier_change_requests r where `+
			strings.Join(where, " and ")+` order by r.created_at desc, r.id desc limit :lim`, args)
		if err != nil {
			return err
		}
		return c.OK(httpx.Paginate(c.s.Cursors, rows, lim, filters, func(x tierChange) (string, string) {
			return httpx.TimeKey(x.CreatedAt), x.ID
		}))
	})
}

func (s *Server) TenantsGetTierChangeRequest(w http.ResponseWriter, r *http.Request, id string, requestID string) {
	s.serve(w, r, opts{op: "tenants.getTierChangeRequest"}, func(c *Call) error {
		if _, _, err := c.visibleTenant(id); err != nil {
			return err
		}
		x, err := c.loadTierChange(id, requestID)
		if err != nil {
			return err
		}
		c.Versioned(x.Version)
		return c.OK(x)
	})
}

func (s *Server) TenantsRequestTierChange(w http.ResponseWriter, r *http.Request, id string) {
	s.serve(w, r, opts{op: "tenants.requestTierChange", userOnly: true, idem: idemOptional}, func(c *Call) error {
		if err := c.require("tenants.manage"); err != nil {
			return err
		}
		t, err := c.loadTenant(id)
		if err != nil {
			return err
		}
		var in struct {
			RequestedTier   string  `json:"requested_tier"`
			RequestedRegion *string `json:"requested_region"`
			Reason          string  `json:"reason"`
		}
		if err := c.Decode(&in); err != nil {
			return err
		}
		var f httpx.Fields
		if in.Reason == "" {
			f.Add("reason", "required")
		} else if len([]rune(in.Reason)) > 2000 {
			f.Max("reason", "too_long", 2000)
		}
		if in.RequestedTier != "isolated" && in.RequestedTier != "dedicated_region" {
			f.Enum("requested_tier", "isolated", "dedicated_region")
		}
		if err := f.Err(); err != nil {
			return err
		}
		upgrade := tierRank[in.RequestedTier] > tierRank[t.Tier] ||
			(in.RequestedTier == "dedicated_region" && t.Tier == "dedicated_region" && in.RequestedRegion != nil && *in.RequestedRegion != t.Region)
		if !upgrade {
			return httpx.Conflict("invalid_tier_transition", "A tier change must move up the ladder shared < isolated < dedicated_region.")
		}
		switch in.RequestedTier {
		case "dedicated_region":
			if in.RequestedRegion == nil || !slices.Contains(supportedRegions, *in.RequestedRegion) {
				return httpx.E(422, "unsupported_region", "requested_region must be a supported region.").With("allowed", supportedRegions)
			}
		case "isolated":
			if in.RequestedRegion != nil {
				f.Add("requested_region", "unknown_value").Message = "not allowed for isolated"
				return f.Err()
			}
		}
		rid := ids.New(ids.TierChange)
		if _, err := c.q.Exec(c.ctx, `insert into tier_change_requests (id, tenant_id, from_tier, from_region, requested_tier,
				requested_region, reason, requested_by)
			values (:id, :t, :ft, :fr, :rt, :rr, :reason, :by)`,
			db.Args{"id": rid, "t": t.ID, "ft": t.Tier, "fr": t.Region, "rt": in.RequestedTier, "rr": in.RequestedRegion,
				"reason": in.Reason, "by": c.p.UserID}); err != nil {
			if _, ok := db.UniqueViolation(err); ok {
				return httpx.Conflict("tier_change_already_pending", "This Tenant already has an open tier-change request.")
			}
			return err
		}
		x, err := c.loadTierChange(t.ID, rid)
		if err != nil {
			return err
		}
		if err := c.audit(Audit{Action: "tenant.tier_change_requested", TargetType: "tier_change_request", TargetID: rid,
			TenantID: &t.ID, After: x}); err != nil {
			return err
		}
		c.Versioned(x.Version)
		return c.Created(x)
	})
}

func (s *Server) TenantsUpdateTierChangeRequest(w http.ResponseWriter, r *http.Request, id string, requestID string, params gen.TenantsUpdateTierChangeRequestParams) {
	s.serve(w, r, opts{op: "tenants.updateTierChangeRequest", userOnly: true}, func(c *Call) error {
		if err := c.require("tenants.manage"); err != nil {
			return err
		}
		t, err := c.loadTenant(id)
		if err != nil {
			return err
		}
		x, err := c.loadTierChange(id, requestID)
		if err != nil {
			return err
		}
		if err := httpx.CheckIfMatch(c.r, x.Version); err != nil {
			return err
		}
		var in struct {
			Status       string     `json:"status"`
			ScheduledFor *time.Time `json:"scheduled_for"`
			Notes        *string    `json:"notes"`
		}
		if err := c.Decode(&in); err != nil {
			return err
		}
		if in.Notes != nil && len([]rune(*in.Notes)) > 2000 {
			var f httpx.Fields
			f.Max("notes", "too_long", 2000)
			return f.Err()
		}
		bad := httpx.Conflict("invalid_request_transition", "The request status change from "+x.Status+" to "+in.Status+" isn't allowed.")
		notes := x.Notes
		if in.Notes != nil {
			notes = in.Notes
		}
		needNotes := func() error {
			if in.Notes == nil || *in.Notes == "" {
				var f httpx.Fields
				f.Add("notes", "required")
				return f.Err()
			}
			return nil
		}
		set := map[string]string{"status": in.Status}
		switch {
		case x.Status == "pending" && in.Status == "scheduled":
			if in.ScheduledFor == nil || !in.ScheduledFor.After(c.now) {
				var f httpx.Fields
				f.Add("scheduled_for", "required").Message = "must be in the future"
				return f.Err()
			}
			if _, err := c.q.Exec(c.ctx, `update tier_change_requests set status = 'scheduled', scheduled_for = :sf::timestamptz,
				notes = :n where id = :id`, db.Args{"sf": db.Time(*in.ScheduledFor), "n": notes, "id": x.ID}); err != nil {
				return err
			}
			c.notifyTenantOwner(t, "Your Substratal maintenance window is scheduled",
				"Your Tenant's tier change is scheduled for "+in.ScheduledFor.UTC().Format(time.RFC1123)+".")
		case (x.Status == "pending" || x.Status == "scheduled") && in.Status == "in_progress":
			if _, err := c.q.Exec(c.ctx, `update tier_change_requests set status = 'in_progress', started_at = now(), notes = :n
				where id = :id`, db.Args{"n": notes, "id": x.ID}); err != nil {
				return err
			}
			if _, err := c.q.Exec(c.ctx, `update tenants set status = 'migrating' where id = :t`, db.Args{"t": t.ID}); err != nil {
				return err
			}
		case x.Status == "in_progress" && in.Status == "completed":
			region := "us-east-1"
			if x.RequestedRegion != nil {
				region = *x.RequestedRegion
			}
			if _, err := c.q.Exec(c.ctx, `update tier_change_requests set status = 'completed', completed_at = now(), notes = :n
				where id = :id`, db.Args{"n": notes, "id": x.ID}); err != nil {
				return err
			}
			if _, err := c.q.Exec(c.ctx, `update tenants set tier = :tier, region = :region, status = 'active' where id = :t`,
				db.Args{"tier": x.RequestedTier, "region": region, "t": t.ID}); err != nil {
				return err
			}
			if err := c.audit(Audit{Action: "tenant.tier_changed", TargetType: "tenant", TargetID: t.ID, TenantID: &t.ID,
				Before: map[string]any{"tier": t.Tier, "region": t.Region},
				After:  map[string]any{"tier": x.RequestedTier, "region": region}}); err != nil {
				return err
			}
		case (x.Status == "pending" || x.Status == "scheduled") && in.Status == "cancelled":
			if err := needNotes(); err != nil {
				return err
			}
			if _, err := c.q.Exec(c.ctx, `update tier_change_requests set status = 'cancelled', notes = :n where id = :id`,
				db.Args{"n": notes, "id": x.ID}); err != nil {
				return err
			}
		case x.Status == "in_progress" && in.Status == "cancelled":
			if err := needNotes(); err != nil {
				return err
			}
			if _, err := c.q.Exec(c.ctx, `update tier_change_requests set status = 'cancelled', notes = :n where id = :id`,
				db.Args{"n": notes, "id": x.ID}); err != nil {
				return err
			}
			if _, err := c.q.Exec(c.ctx, `update tenants set status = 'active' where id = :t`, db.Args{"t": t.ID}); err != nil {
				return err
			}
		default:
			return bad
		}
		after, err := c.loadTierChange(id, requestID)
		if err != nil {
			return err
		}
		_ = set
		if err := c.audit(Audit{Action: "tenant.tier_change_request_updated", TargetType: "tier_change_request", TargetID: x.ID,
			TenantID: &t.ID, Before: x, After: after}); err != nil {
			return err
		}
		c.Versioned(after.Version)
		return c.OK(after)
	})
}

// notifyTenantOwner emails a Tenant's owner (or its Organization's admins).
func (c *Call) notifyTenantOwner(t tenantRow, subject, text string) {
	var emails []string
	if t.OwnerUserID != nil {
		emails, _ = db.All[string](c.ctx, c.q, `select to_jsonb(email::text) from users where id = :u`, db.Args{"u": *t.OwnerUserID})
	} else {
		emails, _ = db.All[string](c.ctx, c.q, `select to_jsonb(u.email::text) from organization_memberships m
			join users u on u.id = m.user_id where m.organization_id = :o and m.role = 'org_admin' and m.status = 'active'`,
			db.Args{"o": *t.OwnerOrganizationID})
	}
	for _, e := range emails {
		c.notice(e, "tenant_notice", subject, text)
	}
}
