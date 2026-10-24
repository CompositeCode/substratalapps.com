package core

import (
	"strings"

	"github.com/CompositeCode/substratalapps.com/internal/db"
	"github.com/CompositeCode/substratalapps.com/internal/httpx"
)

// Optional list counts (Conventions → Pagination → include=total).

// listTotal counts a list's matching rows for ?include=total, up to one
// past the cap. from is the list's FROM clause with its joins, and where
// its filters without the cursor position, so every page reports the same
// total. Nil when the caller didn't ask.
func (c *Call) listTotal(from string, where []string, args db.Args) (*int64, error) {
	if want, err := httpx.WantTotal(c.r); err != nil || !want {
		return nil, err
	}
	cond := "true"
	if len(where) > 0 {
		cond = strings.Join(where, " and ")
	}
	var n int64
	if err := db.One(c.ctx, c.q, &n, httpx.CountSQL("select 1 from "+from+" where "+cond), args); err != nil {
		return nil, err
	}
	return &n, nil
}

// appVisibleSQL is appVisible as a condition on applications a, for
// counting: a per-row check in Go would make the count an unbounded scan.
// Keep the two in step.
func (c *Call) appVisibleSQL(args db.Args) string {
	if c.can("applications.manage") {
		return "true"
	}
	approved := "a.visibility = 'public'"
	var parts []string
	if app, ok := c.p.AppKey(); ok {
		parts = append(parts, "a.id = :vis_key")
		args["vis_key"] = app
	}
	if c.p.IsUser() {
		parts = append(parts, ownerSQL(args, c.p.UserID))
		approved = `(a.visibility = 'public' or (a.visibility = 'invite_only' and (
			exists (select 1 from entitlements e where e.user_id = :vis_me and e.application_id = a.id)
			or exists (select 1 from entitlements e join organization_memberships m on m.organization_id = e.organization_id
			           where m.user_id = :vis_me and e.application_id = a.id))))`
	}
	parts = append(parts, "(a.review_status = 'approved' and "+approved+")")
	return "(" + strings.Join(parts, " or ") + ")"
}

// roleVisibleSQL is canSeeRole as a condition on roles r, for counting.
// Keep the two in step.
func (c *Call) roleVisibleSQL(args db.Args) string {
	platform := "false"
	if (c.p.IsUser() && len(c.p.Perms) > 0) || c.p.PlatformKey() {
		platform = "true"
	}
	app := "false"
	switch key, isKey := c.p.AppKey(); {
	case c.can("roles.manage"):
		app = "true"
	case isKey:
		app = "r.scope = :vis_key"
		args["vis_key"] = key
	case c.p.IsUser():
		app = "exists (select 1 from applications a where a.id = r.scope and " + ownerSQL(args, c.p.UserID) + ")"
	}
	return "(case when r.scope = 'platform' then " + platform + " else " + app + " end)"
}

// ownerSQL is isAppOwner as a condition on applications a.
func ownerSQL(args db.Args, userID string) string {
	args["vis_me"] = userID
	return `(a.owner_user_id = :vis_me or (a.owner_user_id is null and a.owner_organization_id in (
		select organization_id from organization_memberships
		 where user_id = :vis_me and role = 'org_admin' and status = 'active')))`
}

// emptyList answers a list the caller can see nothing in, with a total of
// 0 when one was asked for.
func emptyList[T any](c *Call) error {
	want, err := httpx.WantTotal(c.r)
	if err != nil {
		return err
	}
	l := httpx.List[T]{Data: []T{}}
	if want {
		var zero int64
		l = l.WithTotal(&zero)
	}
	return c.OK(l)
}
