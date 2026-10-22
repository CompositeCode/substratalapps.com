package core

import (
	"context"
	"net/http"
	"slices"
	"time"

	"github.com/CompositeCode/substratalapps.com/internal/api/gen"
	"github.com/CompositeCode/substratalapps.com/internal/db"
	"github.com/CompositeCode/substratalapps.com/internal/httpx"
	"github.com/CompositeCode/substratalapps.com/internal/ids"
)

// Subscriber billing (Pricing; API Reference → Billing). A User subscribes
// to Standard for themselves, or an Organization subscribes for the members
// it covers. Access still flows through Entitlements: the platform keeps a
// source "free" row for every User on each free Application, and a source
// "subscription" row (personal or org-wide) for each standard Application
// while a subscription covers its holder.

// SubscriptionPrefix is the subscription id prefix.
const SubscriptionPrefix = "sbs_"

// StandardUnitAmount is Standard's price, in cents per user per month.
const StandardUnitAmount = 600

// coveringStatusesSQL are the Stripe statuses that cover users with
// Standard. past_due still covers: Stripe is retrying the payment.
const coveringStatusesSQL = `('active','trialing','past_due')`

func covering(status string) bool {
	return status == "active" || status == "trialing" || status == "past_due"
}

// checkoutAllowed: a new Checkout may start when there's no subscription
// yet, or the last one never completed or has ended.
func checkoutAllowed(status string) bool {
	return slices.Contains([]string{"incomplete", "incomplete_expired", "canceled", "unpaid"}, status)
}

type subscriptionRow struct {
	ID                   string     `json:"id"`
	UserID               *string    `json:"user_id"`
	OrganizationID       *string    `json:"organization_id"`
	Plan                 string     `json:"plan"`
	Status               string     `json:"status"`
	StripeSubscriptionID *string    `json:"stripe_subscription_id"`
	PeriodStart          *time.Time `json:"current_period_start"`
	PeriodEnd            *time.Time `json:"current_period_end"`
	CancelAtPeriodEnd    bool       `json:"cancel_at_period_end"`
	MemberScope          *string    `json:"member_scope"`
	MemberOverrides      []string   `json:"member_overrides"`
	QuantitySynced       *int       `json:"quantity_synced"`
	QuantitySyncedAt     *time.Time `json:"quantity_synced_at"`
	TestMode             bool       `json:"test_mode"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
	Version              int        `json:"version"`
}

func (s subscriptionRow) subscriberType() string {
	if s.OrganizationID != nil {
		return "organization"
	}
	return "user"
}

type subscriptionQuantity struct {
	Current      int        `json:"current"`
	LastSynced   *int       `json:"last_synced"`
	LastSyncedAt *time.Time `json:"last_synced_at"`
}

// subscription is the API Subscription object.
type subscription struct {
	ID                string               `json:"id"`
	SubscriberType    string               `json:"subscriber_type"`
	UserID            *string              `json:"user_id"`
	OrganizationID    *string              `json:"organization_id"`
	Plan              string               `json:"plan"`
	Status            string               `json:"status"`
	PeriodStart       *time.Time           `json:"current_period_start"`
	PeriodEnd         *time.Time           `json:"current_period_end"`
	CancelAtPeriodEnd bool                 `json:"cancel_at_period_end"`
	MemberScope       *string              `json:"member_scope"`
	MemberOverrides   []string             `json:"member_overrides"`
	Quantity          subscriptionQuantity `json:"quantity"`
	UnitAmount        int                  `json:"unit_amount"`
	Currency          string               `json:"currency"`
	TestMode          bool                 `json:"test_mode"`
	CreatedAt         time.Time            `json:"created_at"`
	UpdatedAt         time.Time            `json:"updated_at"`
}

type planSource struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

// userPlanSQL computes a User's plan and plan_sources; it's appended to
// userRowSQL, so it reads the outer users row as u.
const userPlanSQL = `(select coalesce(jsonb_agg(jsonb_build_object('type', src.type, 'id', src.id) order by src.type desc, src.id), '[]'::jsonb)
	from (
		select 'user' as type, s.user_id as id from subscriptions s
		 where s.user_id = u.id and s.status in ` + coveringStatusesSQL + `
		union all
		select 'organization', s.organization_id from subscriptions s
		  join organization_memberships m on m.organization_id = s.organization_id and m.user_id = u.id and m.status = 'active'
		 where s.organization_id is not null and s.status in ` + coveringStatusesSQL + `
		   and (s.member_scope = 'all_members'
		        or (s.member_scope = 'allowlist' and u.id = any(s.member_overrides))
		        or (s.member_scope = 'denylist' and not (u.id = any(coalesce(s.member_overrides, '{}')))))
	) src)`

// coveredMembersSQL counts an Organization subscription's covered members:
// active, unexpired Users with an active membership, included by the
// member selection, in the subscription's own mode.
const coveredMembersSQL = `select to_jsonb(count(*)) from subscriptions s
	join organization_memberships m on m.organization_id = s.organization_id and m.status = 'active'
	join users u on u.id = m.user_id and u.status = 'active' and u.test_mode = s.test_mode
	  and (u.expires_at is null or u.expires_at > :now::timestamptz)
	where s.id = :s
	  and (s.member_scope = 'all_members'
	       or (s.member_scope = 'allowlist' and m.user_id = any(s.member_overrides))
	       or (s.member_scope = 'denylist' and not (m.user_id = any(coalesce(s.member_overrides, '{}')))))`

func (c *Call) coveredMembers(subID string) (int, error) {
	var n int
	err := db.One(c.ctx, c.q, &n, coveredMembersSQL, db.Args{"s": subID, "now": db.Time(c.now)})
	return n, err
}

func (c *Call) subscriptionObject(s subscriptionRow) (subscription, error) {
	qty := 1
	if s.OrganizationID != nil {
		n, err := c.coveredMembers(s.ID)
		if err != nil {
			return subscription{}, err
		}
		qty = n
	}
	return subscription{ID: s.ID, SubscriberType: s.subscriberType(), UserID: s.UserID, OrganizationID: s.OrganizationID,
		Plan: s.Plan, Status: s.Status, PeriodStart: s.PeriodStart, PeriodEnd: s.PeriodEnd, CancelAtPeriodEnd: s.CancelAtPeriodEnd,
		MemberScope: s.MemberScope, MemberOverrides: s.MemberOverrides,
		Quantity:   subscriptionQuantity{Current: qty, LastSynced: s.QuantitySynced, LastSyncedAt: s.QuantitySyncedAt},
		UnitAmount: StandardUnitAmount, Currency: "usd", TestMode: s.TestMode, CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt}, nil
}

// loadSubscriptionFor returns the subscriber's subscription row, if any.
func (c *Call) loadSubscriptionFor(subscriberType, id string) (*subscriptionRow, error) {
	col := "user_id"
	if subscriberType == "organization" {
		col = "organization_id"
	}
	var s subscriptionRow
	err := db.One(c.ctx, c.q, &s, `select to_jsonb(s) from subscriptions s where `+col+` = :id`, db.Args{"id": id})
	if err == db.ErrNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (c *Call) loadSubscription(id string) (subscriptionRow, error) {
	var s subscriptionRow
	err := db.One(c.ctx, c.q, &s, `select to_jsonb(s) from subscriptions s where id = :id`, db.Args{"id": id})
	return s, err
}

func (c *Call) optionalSubscriptionObject(s *subscriptionRow) (*subscription, error) {
	if s == nil {
		return nil, nil
	}
	obj, err := c.subscriptionObject(*s)
	return &obj, err
}

// ── Platform-managed Entitlements ──────────────────────────────────────

// accessScope narrows a reconcile to one User's personal rows, one
// Organization's rows, or one Application; the zero value reconciles
// everything visible in the transaction's mode.
type accessScope struct{ UserID, OrgID, AppID string }

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// platformAccessSQL diffs the platform-managed Entitlements that should
// exist against the live ones that do. Rows match on holder, Application,
// and source; an ended row that's wanted again is revived rather than
// replaced, so the one-live-row index never conflicts.
const platformAccessSQL = `with desired as (
		select u.id as user_id, null::text as organization_id, a.id as application_id, a.tenant_id,
		       'free'::text as source, null::text as subscription_id, null::text as member_scope,
		       null::text[] as member_overrides, u.test_mode
		  from users u join applications a on a.access_tier = 'free' and (not a.test_mode or a.test_mode = u.test_mode)
		 where u.status in ('active','invited','suspended') and :o::text is null
		   and (:u::text is null or u.id = :u) and (:a::text is null or a.id = :a)
		union all
		select s.user_id, null, a.id, a.tenant_id, 'subscription', s.id, null, null, s.test_mode
		  from subscriptions s join applications a on a.access_tier = 'standard' and (not a.test_mode or a.test_mode = s.test_mode)
		  join users u on u.id = s.user_id and u.status <> 'deleted'
		 where s.status in ` + coveringStatusesSQL + ` and :o::text is null
		   and (:u::text is null or s.user_id = :u) and (:a::text is null or a.id = :a)
		union all
		select null, s.organization_id, a.id, a.tenant_id, 'subscription', s.id, s.member_scope, s.member_overrides, s.test_mode
		  from subscriptions s join applications a on a.access_tier = 'standard' and (not a.test_mode or a.test_mode = s.test_mode)
		 where s.organization_id is not null and s.status in ` + coveringStatusesSQL + ` and :u::text is null
		   and (:o::text is null or s.organization_id = :o) and (:a::text is null or a.id = :a)),
	existing as (
		select e.id, e.user_id, e.organization_id, e.application_id, e.source, e.subscription_id,
		       e.member_scope, e.member_overrides, e.ends_at
		  from entitlements e
		 where e.source in ('free','subscription') and e.status in ('active','disabled')
		   and (:u::text is null or e.user_id = :u) and (:o::text is null or e.organization_id = :o)
		   and (:a::text is null or e.application_id = :a)),
	matched as (
		select e.id, e.user_id, e.organization_id, e.application_id, e.ends_at,
		       e.member_scope as old_scope, e.member_overrides as old_overrides, e.subscription_id as old_sub,
		       d.member_scope, d.member_overrides, d.subscription_id
		  from existing e join desired d on d.application_id = e.application_id and d.source = e.source
		   and d.user_id is not distinct from e.user_id and d.organization_id is not distinct from e.organization_id)
	select jsonb_build_object(
		'insert', coalesce((select jsonb_agg(to_jsonb(d)) from desired d where not exists (
			select 1 from existing e where e.application_id = d.application_id and e.source = d.source
			   and e.user_id is not distinct from d.user_id and e.organization_id is not distinct from d.organization_id)), '[]'::jsonb),
		'revive', coalesce((select jsonb_agg(jsonb_build_object('id', m.id, 'user_id', m.user_id,
			'organization_id', m.organization_id, 'application_id', m.application_id, 'subscription_id', m.subscription_id))
			from matched m where m.ends_at is not null or m.old_sub is distinct from m.subscription_id), '[]'::jsonb),
		'end', coalesce((select jsonb_agg(jsonb_build_object('id', e.id, 'user_id', e.user_id,
			'organization_id', e.organization_id, 'application_id', e.application_id))
			from existing e where (e.ends_at is null or e.ends_at > now())
			  and not exists (select 1 from matched m where m.id = e.id)), '[]'::jsonb),
		'rescope', coalesce((select jsonb_agg(jsonb_build_object('id', m.id, 'organization_id', m.organization_id,
			'application_id', m.application_id, 'member_scope', m.member_scope, 'member_overrides', m.member_overrides))
			from matched m where m.organization_id is not null
			  and (m.old_scope is distinct from m.member_scope or m.old_overrides is distinct from m.member_overrides)), '[]'::jsonb))`

type platformRow struct {
	ID              string   `json:"id"`
	UserID          *string  `json:"user_id"`
	OrganizationID  *string  `json:"organization_id"`
	ApplicationID   string   `json:"application_id"`
	TenantID        string   `json:"tenant_id"`
	Source          string   `json:"source"`
	SubscriptionID  *string  `json:"subscription_id"`
	MemberScope     *string  `json:"member_scope"`
	MemberOverrides []string `json:"member_overrides"`
	TestMode        bool     `json:"test_mode"`
}

// reconcileAccess brings platform-managed Entitlements in line with the
// Applications' access tiers and the subscriptions that cover their
// holders, in the current transaction, with the same audit events,
// resource webhooks, and access.* events as any other Entitlement change.
func (c *Call) reconcileAccess(sc accessScope) error {
	var diff struct {
		Insert  []platformRow `json:"insert"`
		Revive  []platformRow `json:"revive"`
		End     []platformRow `json:"end"`
		Rescope []platformRow `json:"rescope"`
	}
	if err := db.One(c.ctx, c.q, &diff, platformAccessSQL,
		db.Args{"u": nullable(sc.UserID), "o": nullable(sc.OrgID), "a": nullable(sc.AppID)}); err != nil {
		return err
	}
	if len(diff.Insert)+len(diff.Revive)+len(diff.End)+len(diff.Rescope) == 0 {
		return nil
	}
	// Snapshot resolved access for every pair these writes can affect.
	var personal, org []pair
	seen := map[pair]bool{}
	add := func(r platformRow) error {
		if r.UserID != nil {
			p := pair{*r.UserID, r.ApplicationID}
			if !seen[p] {
				seen[p] = true
				personal = append(personal, p)
			}
			return nil
		}
		ps, err := c.orgMemberPairs(*r.OrganizationID, r.ApplicationID)
		if err != nil {
			return err
		}
		for _, p := range ps {
			if !seen[p] {
				seen[p] = true
				org = append(org, p)
			}
		}
		return nil
	}
	for _, list := range [][]platformRow{diff.Insert, diff.Revive, diff.End, diff.Rescope} {
		for _, r := range list {
			if err := add(r); err != nil {
				return err
			}
		}
	}
	watchPersonal, err := c.watchAccess(personal)
	if err != nil {
		return err
	}
	watchOrg, err := c.watchAccess(org)
	if err != nil {
		return err
	}

	system := func(action string, before *entitlement, after entitlement, extra map[string]any) error {
		return c.platformEntitlementEffects(action, before, after, extra)
	}
	for _, r := range diff.End {
		before, err := c.loadEnt(r.ID)
		if err != nil {
			return err
		}
		if _, err := c.q.Exec(c.ctx, `update entitlements set ends_at = :now::timestamptz where id = :id`,
			db.Args{"now": db.Time(c.now), "id": r.ID}); err != nil {
			return err
		}
		after, err := c.loadEnt(r.ID)
		if err != nil {
			return err
		}
		if err := system("entitlement.updated", &before, after, map[string]any{"changed_fields": []string{"ends_at"}}); err != nil {
			return err
		}
	}
	for _, r := range diff.Revive {
		before, err := c.loadEnt(r.ID)
		if err != nil {
			return err
		}
		if _, err := c.q.Exec(c.ctx, `update entitlements set ends_at = null, subscription_id = :s where id = :id`,
			db.Args{"s": db.NullString(r.SubscriptionID), "id": r.ID}); err != nil {
			return err
		}
		after, err := c.loadEnt(r.ID)
		if err != nil {
			return err
		}
		if err := system("entitlement.updated", &before, after, map[string]any{"changed_fields": []string{"ends_at"}}); err != nil {
			return err
		}
	}
	for _, r := range diff.Rescope {
		before, err := c.loadEnt(r.ID)
		if err != nil {
			return err
		}
		if _, err := c.q.Exec(c.ctx, `update entitlements set member_scope = :ms, member_overrides = :mo::text[] where id = :id`,
			db.Args{"ms": r.MemberScope, "mo": db.NullTextArray(r.MemberOverrides), "id": r.ID}); err != nil {
			return err
		}
		after, err := c.loadEnt(r.ID)
		if err != nil {
			return err
		}
		if err := system("entitlement.member_scope_changed", &before, after, nil); err != nil {
			return err
		}
	}
	for _, r := range diff.Insert {
		eid := ids.New(ids.Entitlement)
		if _, err := c.q.Exec(c.ctx, `insert into entitlements (id, user_id, organization_id, application_id, tenant_id,
				status, source, subscription_id, starts_at, member_scope, member_overrides, test_mode)
			values (:id, :u, :o, :a, :t, 'active', :src, :sub, :st::timestamptz, :ms, :mo::text[], :tm)`,
			db.Args{"id": eid, "u": db.NullString(r.UserID), "o": db.NullString(r.OrganizationID), "a": r.ApplicationID,
				"t": r.TenantID, "src": r.Source, "sub": db.NullString(r.SubscriptionID), "st": db.Time(c.now),
				"ms": db.NullString(r.MemberScope), "mo": db.NullTextArray(r.MemberOverrides), "tm": r.TestMode}); err != nil {
			return err
		}
		e, err := c.loadEnt(eid)
		if err != nil {
			return err
		}
		if err := system("entitlement.granted", nil, e, nil); err != nil {
			return err
		}
	}
	if err := c.flip(watchPersonal, "entitlement_granted", "entitlement_expired"); err != nil {
		return err
	}
	return c.flip(watchOrg, "org_grant_changed", "org_grant_changed")
}

// platformEntitlementEffects is entitlementEffects with the platform
// itself as the actor: these rows are never changed by a person.
func (c *Call) platformEntitlementEffects(action string, before *entitlement, after entitlement, extra map[string]any) error {
	app := after.ApplicationID
	a := Audit{Action: action, TargetType: "entitlement", TargetID: after.ID, TargetUserID: after.UserID,
		ApplicationID: &app, OrganizationID: after.OrganizationID, TenantID: strp(after.tenantID), After: after.snapshot(),
		ActorType: "system", ActorID: "system"}
	if before != nil {
		a.Before = before.snapshot()
	}
	if err := c.audit(a); err != nil {
		return err
	}
	data := map[string]any{"entitlement": after.snapshot()}
	for k, v := range extra {
		data[k] = v
	}
	return c.emit(action, &app, data)
}

func platformManaged(source string) bool { return source == "free" || source == "subscription" }

func errPlatformManaged() error {
	return httpx.Conflict("entitlement_platform_managed",
		"This Entitlement is managed by the platform from the Application's access tier and the holder's subscription; it can only be disabled or re-enabled.")
}

// ReconcileAllAccess is the hourly safety net under the inline reconciles:
// every platform-managed Entitlement, in both modes.
func (s *Server) ReconcileAllAccess(ctx context.Context) error {
	return eachMode(func(tm bool) error {
		return s.system(ctx, tm, db.Settings{}, func(c *Call) error { return c.reconcileAccess(accessScope{}) })
	})
}

// ── GET /users/{id}/subscription ───────────────────────────────────────

func (s *Server) BillingGetUserSubscription(w http.ResponseWriter, r *http.Request, id gen.UserIdOrMe) {
	s.serve(w, r, opts{op: "billing.getUserSubscription"}, func(c *Call) error {
		uid, err := c.selfID(id)
		if err != nil {
			return err
		}
		if !c.isSelf(uid) && !c.can("billing.manage") {
			return httpx.Forbidden("billing.manage")
		}
		u, err := c.visibleUser(uid)
		if err != nil {
			return err
		}
		sub, err := c.loadSubscriptionFor("user", uid)
		if err != nil {
			return err
		}
		obj, err := c.optionalSubscriptionObject(sub)
		if err != nil {
			return err
		}
		return c.OK(map[string]any{"user_id": uid, "plan": u.Plan, "plan_sources": u.PlanSources, "subscription": obj})
	})
}

// ── GET/PATCH /organizations/{id}/subscription ─────────────────────────

// orgBillingAccess: org admins manage their Organization's subscription;
// billing.manage can read it and open its portal.
func (c *Call) orgBillingAccess(orgID string) (o orgRow, admin bool, err error) {
	o, err = c.loadOrg(orgID)
	if err != nil {
		return o, false, err
	}
	admin, err = c.isOrgAdmin(o.ID)
	if err != nil {
		return o, false, err
	}
	if !admin && !c.can("billing.manage") {
		// Members can see the Organization exists; anyone else gets 404.
		if c.p.IsUser() {
			if member, err := c.isActiveMember(o.ID, c.p.UserID); err != nil {
				return o, false, err
			} else if member {
				return o, false, httpx.Forbidden("billing.manage")
			}
		}
		return o, false, httpx.NotFoundResource("organization")
	}
	return o, admin, nil
}

func (c *Call) orgSubscriptionBody(o orgRow, sub *subscriptionRow) (map[string]any, error) {
	obj, err := c.optionalSubscriptionObject(sub)
	if err != nil {
		return nil, err
	}
	plan := "free"
	if sub != nil && covering(sub.Status) {
		plan = "standard"
	}
	if sub != nil {
		c.Versioned(sub.Version)
	}
	return map[string]any{"organization_id": o.ID, "plan": plan, "subscription": obj}, nil
}

func (s *Server) BillingGetOrganizationSubscription(w http.ResponseWriter, r *http.Request, id string) {
	s.serve(w, r, opts{op: "billing.getOrganizationSubscription"}, func(c *Call) error {
		o, _, err := c.orgBillingAccess(id)
		if err != nil {
			return err
		}
		sub, err := c.loadSubscriptionFor("organization", o.ID)
		if err != nil {
			return err
		}
		body, err := c.orgSubscriptionBody(o, sub)
		if err != nil {
			return err
		}
		return c.OK(body)
	})
}

func (s *Server) BillingUpdateOrganizationSubscription(w http.ResponseWriter, r *http.Request, id string, params gen.BillingUpdateOrganizationSubscriptionParams) {
	s.serve(w, r, opts{op: "billing.updateOrganizationSubscription"}, func(c *Call) error {
		o, admin, err := c.orgBillingAccess(id)
		if err != nil {
			return err
		}
		if !admin {
			return httpx.Forbidden("org_admin")
		}
		sub, err := c.loadSubscriptionFor("organization", o.ID)
		if err != nil {
			return err
		}
		if sub == nil {
			return httpx.NotFoundResource("subscription")
		}
		if err := httpx.CheckIfMatch(c.r, sub.Version); err != nil {
			return err
		}
		p, err := c.Patch()
		if err != nil {
			return err
		}
		for k := range p {
			if k != "member_scope" && k != "member_overrides" {
				return httpx.E(422, "read_only_field", "Only member_scope and member_overrides can be changed here.").With("field", k)
			}
		}
		var f httpx.Fields
		p.RequireNonNull(&f, "member_scope")
		scope, overrides := deref(sub.MemberScope), sub.MemberOverrides
		if p.Has("member_scope") {
			var ms string
			if p.Get("member_scope", &ms, &f) {
				if ms != "all_members" && ms != "allowlist" && ms != "denylist" {
					f.Enum("member_scope", "all_members", "allowlist", "denylist")
				}
				scope = ms
			}
		}
		if p.Has("member_overrides") {
			if p.IsNull("member_overrides") {
				overrides = nil
			} else {
				var mo []string
				if p.Get("member_overrides", &mo, &f) {
					if len(mo) > 1000 {
						f.Max("member_overrides", "too_long", 1000)
					}
					overrides = dedupe(mo)
				}
			}
		}
		if err := f.Err(); err != nil {
			return err
		}
		if err := c.checkOverridesAreMembers(o.ID, overrides); err != nil {
			return err
		}
		if scope != deref(sub.MemberScope) || !slices.Equal(overrides, sub.MemberOverrides) {
			if _, err := c.q.Exec(c.ctx, `update subscriptions set member_scope = :ms, member_overrides = :mo::text[] where id = :id`,
				db.Args{"ms": scope, "mo": db.NullTextArray(overrides), "id": sub.ID}); err != nil {
				return err
			}
			after, err := c.loadSubscription(sub.ID)
			if err != nil {
				return err
			}
			if err := c.audit(Audit{Action: "subscription.members_changed", TargetType: "subscription", TargetID: sub.ID,
				OrganizationID: &o.ID, Before: map[string]any{"member_scope": sub.MemberScope, "member_overrides": sub.MemberOverrides},
				After: map[string]any{"member_scope": after.MemberScope, "member_overrides": after.MemberOverrides}}); err != nil {
				return err
			}
			if err := c.reconcileAccess(accessScope{OrgID: o.ID}); err != nil {
				return err
			}
			sub = &after
		}
		body, err := c.orgSubscriptionBody(o, sub)
		if err != nil {
			return err
		}
		return c.OK(body)
	})
}

// ── Checkout and Portal ────────────────────────────────────────────────

type checkoutRequest struct {
	Plan            string   `json:"plan"`
	SuccessURL      string   `json:"success_url"`
	CancelURL       string   `json:"cancel_url"`
	MemberScope     *string  `json:"member_scope"`
	MemberOverrides []string `json:"member_overrides"`
}

func (in checkoutRequest) validate() error {
	if in.Plan == "business" || in.Plan == "enterprise" {
		return httpx.E(422, "plan_requires_sales", "Business and Enterprise plans are arranged with Substratal directly.")
	}
	var f httpx.Fields
	if in.Plan != "standard" {
		f.Enum("plan", "standard")
	}
	if !httpsURL(in.SuccessURL) {
		f.Add("success_url", "invalid_format").Allowed = []string{"https"}
	}
	if !httpsURL(in.CancelURL) {
		f.Add("cancel_url", "invalid_format").Allowed = []string{"https"}
	}
	return f.Err()
}

func billingUnavailable() error {
	return httpx.E(503, "billing_unavailable", "Billing is temporarily unavailable.").RetryAfter(30)
}

// billingReady checks that checkout can run in this request's mode: a
// test-mode subscription needs Stripe test mode, so an API configured with
// a live Stripe key refuses test-mode checkout.
func (c *Call) billingReady() error {
	if c.s.Billing == nil {
		return billingUnavailable()
	}
	if c.testMode() && c.s.Billing.LiveMode() {
		return httpx.Conflict("test_mode_billing_unavailable",
			"This deployment bills in Stripe live mode, so test-mode subscriptions can't be started here.")
	}
	return nil
}

// ensureSubscriptionRow returns the subscriber's subscription row, creating
// an incomplete one for a first Checkout. A row that already covers its
// subscriber means there's nothing to buy.
func (c *Call) ensureSubscriptionRow(subscriberType, id string, scope *string, overrides []string) (subscriptionRow, error) {
	sub, err := c.loadSubscriptionFor(subscriberType, id)
	if err != nil {
		return subscriptionRow{}, err
	}
	if sub != nil {
		if !checkoutAllowed(sub.Status) {
			return subscriptionRow{}, httpx.Conflict("subscription_exists",
				"There's already a subscription; manage it in the billing portal.").With("subscription_id", sub.ID).With("status", sub.Status)
		}
		if subscriberType == "organization" && scope != nil {
			if _, err := c.q.Exec(c.ctx, `update subscriptions set member_scope = :ms, member_overrides = :mo::text[] where id = :id`,
				db.Args{"ms": *scope, "mo": db.NullTextArray(overrides), "id": sub.ID}); err != nil {
				return subscriptionRow{}, err
			}
			return c.loadSubscription(sub.ID)
		}
		return *sub, nil
	}
	sid := ids.New(SubscriptionPrefix)
	var u, o, ms any
	if subscriberType == "organization" {
		o = id
		s := "all_members"
		if scope != nil {
			s = *scope
		}
		ms = s
	} else {
		u = id
	}
	if _, err := c.q.Exec(c.ctx, `insert into subscriptions (id, user_id, organization_id, member_scope, member_overrides, test_mode)
		values (:id, :u, :o, :ms, :mo::text[], :tm)`,
		db.Args{"id": sid, "u": u, "o": o, "ms": ms, "mo": db.NullTextArray(overrides), "tm": c.testMode()}); err != nil {
		return subscriptionRow{}, err
	}
	row, err := c.loadSubscription(sid)
	if err != nil {
		return row, err
	}
	a := Audit{Action: "subscription.created", TargetType: "subscription", TargetID: sid, After: row}
	if subscriberType == "organization" {
		a.OrganizationID = &id
	} else {
		a.TargetUserID = &id
	}
	return row, c.audit(a)
}

// ensureCustomer returns the subscriber's Stripe Customer, creating and
// recording it on first use.
func (c *Call) ensureCustomer(subscriberType, id, email string) (string, error) {
	table := "users"
	if subscriberType == "organization" {
		table = "organizations"
	}
	var existing *string
	if err := db.One(c.ctx, c.q, &existing, `select to_jsonb(stripe_customer_id) from `+table+` where id = :id`,
		db.Args{"id": id}); err != nil {
		return "", err
	}
	if existing != nil {
		return *existing, nil
	}
	cust, err := c.s.Billing.EnsureCustomer(c.ctx, BillingSubscriber{Type: subscriberType, ID: id}, email)
	if err != nil {
		return "", err
	}
	_, err = c.q.Exec(c.ctx, `update `+table+` set stripe_customer_id = :c where id = :id and stripe_customer_id is null`,
		db.Args{"c": cust, "id": id})
	return cust, err
}

func (c *Call) startCheckout(subscriberType, id, email string, in checkoutRequest) error {
	if err := c.billingReady(); err != nil {
		return err
	}
	sub, err := c.ensureSubscriptionRow(subscriberType, id, in.MemberScope, in.MemberOverrides)
	if err != nil {
		return err
	}
	customer, err := c.ensureCustomer(subscriberType, id, email)
	if err != nil {
		c.s.Log.Error("stripe customer failed", "subscriber", id, "err", err)
		return billingUnavailable()
	}
	qty := 1
	if subscriberType == "organization" {
		if qty, err = c.coveredMembers(sub.ID); err != nil {
			return err
		}
		qty = max(qty, 1)
	}
	url, expires, err := c.s.Billing.CheckoutSession(c.ctx, CheckoutInput{SubscriptionID: sub.ID,
		Subscriber: BillingSubscriber{Type: subscriberType, ID: id}, CustomerID: customer, Quantity: qty,
		SuccessURL: in.SuccessURL, CancelURL: in.CancelURL})
	if err != nil {
		c.s.Log.Error("stripe checkout failed", "subscription", sub.ID, "err", err)
		return billingUnavailable()
	}
	return c.Created(map[string]any{"url": url, "expires_at": expires.UTC().Format(time.RFC3339)})
}

func (c *Call) startPortal(subscriberType, id string) error {
	var in struct {
		ReturnURL string `json:"return_url"`
	}
	if err := c.Decode(&in); err != nil {
		return err
	}
	if !httpsURL(in.ReturnURL) {
		var f httpx.Fields
		f.Add("return_url", "invalid_format").Allowed = []string{"https"}
		return f.Err()
	}
	table := "users"
	if subscriberType == "organization" {
		table = "organizations"
	}
	var customer *string
	if err := db.One(c.ctx, c.q, &customer, `select to_jsonb(stripe_customer_id) from `+table+` where id = :id`,
		db.Args{"id": id}); err != nil {
		return err
	}
	sub, err := c.loadSubscriptionFor(subscriberType, id)
	if err != nil {
		return err
	}
	if customer == nil || sub == nil || sub.StripeSubscriptionID == nil {
		return httpx.Conflict("no_billing_account", "There's no billing history to manage yet.")
	}
	if c.s.Billing == nil {
		return billingUnavailable()
	}
	url, expires, err := c.s.Billing.PortalSession(c.ctx, *customer, in.ReturnURL)
	if err != nil {
		c.s.Log.Error("stripe portal failed", "subscriber", id, "err", err)
		return billingUnavailable()
	}
	return c.Created(map[string]any{"url": url, "expires_at": expires.UTC().Format(time.RFC3339)})
}

func (c *Call) callerEmail() (string, error) {
	emails, err := db.All[string](c.ctx, c.q, `select to_jsonb(email::text) from users where id = :u`, db.Args{"u": c.p.UserID})
	if err != nil || len(emails) == 0 {
		return "", err
	}
	return emails[0], nil
}

func (s *Server) BillingCreateUserCheckoutSession(w http.ResponseWriter, r *http.Request, params gen.BillingCreateUserCheckoutSessionParams) {
	s.serve(w, r, opts{op: "billing.createUserCheckoutSession", userOnly: true, idem: idemRequired}, func(c *Call) error {
		var in checkoutRequest
		if err := c.Decode(&in); err != nil {
			return err
		}
		if in.MemberScope != nil || in.MemberOverrides != nil {
			return httpx.E(422, "read_only_field", "member_scope and member_overrides apply only to Organization subscriptions.")
		}
		if err := in.validate(); err != nil {
			return err
		}
		email, err := c.callerEmail()
		if err != nil {
			return err
		}
		return c.startCheckout("user", c.p.UserID, email, in)
	})
}

func (s *Server) BillingCreateUserPortalSession(w http.ResponseWriter, r *http.Request) {
	s.serve(w, r, opts{op: "billing.createUserPortalSession", userOnly: true}, func(c *Call) error {
		return c.startPortal("user", c.p.UserID)
	})
}

func (s *Server) BillingCreateOrganizationCheckoutSession(w http.ResponseWriter, r *http.Request, id string, params gen.BillingCreateOrganizationCheckoutSessionParams) {
	s.serve(w, r, opts{op: "billing.createOrganizationCheckoutSession", userOnly: true, idem: idemRequired}, func(c *Call) error {
		o, admin, err := c.orgBillingAccess(id)
		if err != nil {
			return err
		}
		if !admin {
			return httpx.Forbidden("org_admin")
		}
		var in checkoutRequest
		if err := c.Decode(&in); err != nil {
			return err
		}
		if err := in.validate(); err != nil {
			return err
		}
		if in.MemberScope != nil && *in.MemberScope != "all_members" && *in.MemberScope != "allowlist" && *in.MemberScope != "denylist" {
			var f httpx.Fields
			f.Enum("member_scope", "all_members", "allowlist", "denylist")
			return f.Err()
		}
		if len(in.MemberOverrides) > 1000 {
			var f httpx.Fields
			f.Max("member_overrides", "too_long", 1000)
			return f.Err()
		}
		in.MemberOverrides = dedupe(in.MemberOverrides)
		if len(in.MemberOverrides) == 0 {
			in.MemberOverrides = nil
		}
		if err := c.checkOverridesAreMembers(o.ID, in.MemberOverrides); err != nil {
			return err
		}
		email, err := c.callerEmail()
		if err != nil {
			return err
		}
		return c.startCheckout("organization", o.ID, email, in)
	})
}

func (s *Server) BillingCreateOrganizationPortalSession(w http.ResponseWriter, r *http.Request, id string) {
	s.serve(w, r, opts{op: "billing.createOrganizationPortalSession", userOnly: true}, func(c *Call) error {
		o, _, err := c.orgBillingAccess(id)
		if err != nil {
			return err
		}
		return c.startPortal("organization", o.ID)
	})
}

// ── Stripe sync (infra/README.md → Webhook handling) ─────────────────────

// StripeSubscription is the subscription state the sync applies, read
// fresh from Stripe by the billing package.
type StripeSubscription struct {
	ID                string // Stripe's sub_… id
	SubscriptionID    string // our sbs_… id, from metadata
	Subscriber        BillingSubscriber
	Status            string
	Plan              string // from the product's metadata.substratal_plan
	PeriodStart       *time.Time
	PeriodEnd         *time.Time
	CancelAtPeriodEnd bool
	ItemID            string // the standard_monthly_usd subscription item
	Quantity          int
}

// RecordStripeEvent inserts the event id (insert-or-skip) and reports
// whether it's new.
func (s *Server) RecordStripeEvent(ctx context.Context, id, typ string) (bool, error) {
	var fresh bool
	err := s.system(ctx, false, db.Settings{}, func(c *Call) error {
		n, err := c.q.Exec(ctx, `insert into stripe_events (id, type) values (:id, :t) on conflict (id) do nothing`,
			db.Args{"id": id, "t": typ})
		fresh = n == 1
		return err
	})
	return fresh, err
}

// MarkStripeEventProcessed records a successfully handled event.
func (s *Server) MarkStripeEventProcessed(ctx context.Context, id, subscriptionID string) error {
	return s.system(ctx, false, db.Settings{AllModes: true}, func(c *Call) error {
		_, err := c.q.Exec(ctx, `update stripe_events set processed_at = now(),
			subscription_id = coalesce(subscription_id, (select id from subscriptions where id = :s)) where id = :id`,
			db.Args{"id": id, "s": nullable(subscriptionID)})
		return err
	})
}

// UnprocessedStripeEvents lists recorded Stripe events still unprocessed
// five minutes after arrival, for the retry job.
func (s *Server) UnprocessedStripeEvents(ctx context.Context) ([]string, error) {
	var out []string
	err := s.system(ctx, false, db.Settings{}, func(c *Call) error {
		var err error
		out, err = db.All[string](ctx, c.q, `select to_jsonb(id) from stripe_events where processed_at is null
			and received_at < now() - interval '5 minutes' order by received_at limit 100`, nil)
		return err
	})
	return out, err
}

// ApplyStripeSubscription is the subscription sync: Stripe is the only
// writer of a subscription's status after Checkout. It returns our
// subscription id. Access follows in the same transaction.
func (s *Server) ApplyStripeSubscription(ctx context.Context, sub StripeSubscription) (string, error) {
	// Find the row in either mode first, then work in its own mode so its
	// audit events and Entitlements carry the right test_mode.
	var row subscriptionRow
	err := s.system(ctx, false, db.Settings{AllModes: true}, func(c *Call) error {
		return db.One(ctx, c.q, &row, `select to_jsonb(s) from subscriptions s where id = :id or stripe_subscription_id = :sub
			order by (id = :id) desc limit 1`, db.Args{"id": sub.SubscriptionID, "sub": sub.ID})
	})
	if err != nil {
		return "", errf("stripe sync: no subscription for %s (%s): %w", sub.ID, sub.SubscriptionID, err)
	}
	// After a resubscribe the row tracks the new Stripe subscription. A late
	// or re-fetched event about the old one must not end coverage the new
	// one provides, so a non-covering state from a subscription the row no
	// longer tracks is ignored.
	if row.StripeSubscriptionID != nil && *row.StripeSubscriptionID != sub.ID && !covering(sub.Status) {
		return row.ID, nil
	}
	err = s.system(ctx, row.TestMode, db.Settings{}, func(c *Call) error {
		if _, err := c.q.Exec(ctx, `update subscriptions set status = :st, stripe_subscription_id = :sub,
				current_period_start = :ps::timestamptz, current_period_end = :pe::timestamptz, cancel_at_period_end = :cap
			where id = :id`,
			db.Args{"st": sub.Status, "sub": sub.ID, "ps": db.NullTime(sub.PeriodStart), "pe": db.NullTime(sub.PeriodEnd),
				"cap": sub.CancelAtPeriodEnd, "id": row.ID}); err != nil {
			return err
		}
		if row.Status != sub.Status {
			a := Audit{Action: "subscription.status_changed", TargetType: "subscription", TargetID: row.ID,
				Before: map[string]any{"status": row.Status}, After: map[string]any{"status": sub.Status}}
			if row.OrganizationID != nil {
				a.OrganizationID = row.OrganizationID
			} else {
				a.TargetUserID = row.UserID
			}
			if err := c.audit(a); err != nil {
				return err
			}
		}
		if row.OrganizationID != nil {
			return c.reconcileAccess(accessScope{OrgID: *row.OrganizationID})
		}
		return c.reconcileAccess(accessScope{UserID: deref(row.UserID)})
	})
	return row.ID, err
}

// QuantityTarget is one Organization subscription's covered-member count
// for the daily quantity sync.
type QuantityTarget struct {
	SubscriptionID       string
	StripeSubscriptionID string
	Quantity             int
	LastSynced           *int
}

// OrganizationQuantities lists every covering Organization subscription's
// current covered-member count, in both modes.
func (s *Server) OrganizationQuantities(ctx context.Context) ([]QuantityTarget, error) {
	var out []QuantityTarget
	err := s.system(ctx, false, db.Settings{AllModes: true}, func(c *Call) error {
		subs, err := db.All[subscriptionRow](ctx, c.q, `select to_jsonb(s) from subscriptions s
			where organization_id is not null and stripe_subscription_id is not null and status in `+coveringStatusesSQL+`
			order by id`, nil)
		if err != nil {
			return err
		}
		for _, sub := range subs {
			n, err := c.coveredMembers(sub.ID)
			if err != nil {
				return err
			}
			out = append(out, QuantityTarget{SubscriptionID: sub.ID, StripeSubscriptionID: deref(sub.StripeSubscriptionID),
				Quantity: max(n, 1), LastSynced: sub.QuantitySynced})
		}
		return nil
	})
	return out, err
}

// RecordQuantitySync stores the quantity last written to Stripe.
func (s *Server) RecordQuantitySync(ctx context.Context, subscriptionID string, quantity int) error {
	return s.system(ctx, false, db.Settings{AllModes: true}, func(c *Call) error {
		_, err := c.q.Exec(ctx, `update subscriptions set quantity_synced = :n, quantity_synced_at = now() where id = :s`,
			db.Args{"n": int64(quantity), "s": subscriptionID})
		return err
	})
}
