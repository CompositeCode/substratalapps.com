package core

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/CompositeCode/substratalapps.com/internal/db"
	"github.com/CompositeCode/substratalapps.com/internal/httpx"
	"github.com/CompositeCode/substratalapps.com/internal/schema"
)

// system runs fn as the platform itself (actor "system") in one
// transaction pinned to testMode. Jobs that touch both modes call it once
// per mode, so every event and audit row carries the right mode.
func (s *Server) system(ctx context.Context, testMode bool, settings db.Settings, fn func(c *Call) error) error {
	settings.TestMode = testMode
	c := &Call{s: s, ctx: ctx, now: s.Now().UTC().Truncate(time.Microsecond), headers: map[string][]string{}, sysTestMode: testMode}
	err := s.DB.Tx(ctx, settings, func(q db.Querier) error {
		c.q = q
		return fn(c)
	})
	if err != nil {
		return err
	}
	for _, f := range c.afterCommit {
		f(context.WithoutCancel(ctx))
	}
	return nil
}

func eachMode(fn func(testMode bool) error) error {
	for _, m := range []bool{false, true} {
		if err := fn(m); err != nil {
			return err
		}
	}
	return nil
}

// ── Entitlement sweep (every 5 minutes) ────────────────────────────────

// SweepEntitlements expires rows past ends_at and fires access.granted
// for future-dated grants whose starts_at has arrived. Resolution already
// treats both as effective immediately; this makes the stored status and
// the events catch up.
func (s *Server) SweepEntitlements(ctx context.Context) (expired, started int, err error) {
	err = eachMode(func(tm bool) error {
		return s.system(ctx, tm, db.Settings{}, func(c *Call) error {
			rows, err := c.queryEnts(`e.status in ('active','disabled') and e.ends_at <= now()
				order by e.ends_at limit 500 for update skip locked`, db.Args{})
			if err != nil {
				return err
			}
			for _, e := range rows {
				pairs, err := c.affectedPairs(e)
				if err != nil {
					return err
				}
				// Resolution already treats this row as expired, so "before"
				// is resolved as of the instant before ends_at.
				now := c.now
				c.now = e.EndsAt.Add(-time.Nanosecond)
				watch, err := c.watchAccess(pairs)
				c.now = now
				if err != nil {
					return err
				}
				if _, err := c.q.Exec(ctx, `update entitlements set status = 'expired' where id = :id`, db.Args{"id": e.ID}); err != nil {
					return err
				}
				after, err := c.loadEnt(e.ID)
				if err != nil {
					return err
				}
				if err := c.entitlementEffects("entitlement.expired", &e, after, nil); err != nil {
					return err
				}
				reason := "entitlement_expired"
				if e.OrganizationID != nil {
					reason = "org_grant_changed"
				}
				if e.Status == "active" {
					if err := c.flip(watch, "entitlement_enabled", reason); err != nil {
						return err
					}
				}
				expired++
			}
			starts, err := c.queryEnts(`e.status = 'active' and e.starts_at <= now() and e.start_notified_at is null
				and e.starts_at > e.created_at order by e.starts_at limit 500 for update skip locked`, db.Args{})
			if err != nil {
				return err
			}
			for _, e := range starts {
				pairs, err := c.affectedPairs(e)
				if err != nil {
					return err
				}
				res, err := c.resolvePairs(pairs)
				if err != nil {
					return err
				}
				for _, p := range pairs {
					r := res[p]
					if r.Allowed && r.Res.ActiveVia != nil && r.Res.ActiveVia.EntitlementID == e.ID {
						app := p.App
						if err := c.emit("access.granted", &app, map[string]any{"user_id": p.User, "application_id": p.App,
							"reason": "entitlement_started", "entitlement_status": "active"}); err != nil {
							return err
						}
					}
				}
				if _, err := c.q.Exec(ctx, `update entitlements set start_notified_at = now() where id = :id`, db.Args{"id": e.ID}); err != nil {
					return err
				}
				started++
			}
			return nil
		})
	})
	return
}

// ── Test-mode User expiry (runs with the entitlement sweep) ───────────

// ExpireTestUsers processes every test-mode User whose expires_at has
// passed, exactly once each (Auth → Test-mode Users → Lifetime). Their
// tokens already stopped working at expires_at; this revokes sessions and
// devices, fires access.revoked and user.expired, and writes the audit
// event. status, Entitlements, Roles, and memberships are untouched.
func (s *Server) ExpireTestUsers(ctx context.Context) (n int, err error) {
	err = s.system(ctx, true, db.Settings{}, func(c *Call) error {
		uids, err := db.All[string](ctx, c.q, `select to_jsonb(id) from users
			where test_mode and expires_at <= :now::timestamptz and expired_processed_at is null
			order by expires_at limit 500 for update skip locked`, db.Args{"now": db.Time(c.now)})
		if err != nil {
			return err
		}
		for _, id := range uids {
			u, err := c.findUser("u.id = :id", db.Args{"id": id})
			if err != nil {
				return err
			}
			pairs, err := c.userAppPairs(u.ID)
			if err != nil {
				return err
			}
			// Resolution already treats the User as expired, so "before"
			// is resolved as of the instant before expires_at.
			now := c.now
			c.now = u.ExpiresAt.Add(-time.Nanosecond)
			watch, err := c.watchAccess(pairs)
			c.now = now
			if err != nil {
				return err
			}
			if _, err := c.q.Exec(ctx, `update users set expired_processed_at = :now::timestamptz where id = :u`,
				db.Args{"u": u.ID, "now": db.Time(c.now)}); err != nil {
				return err
			}
			if err := c.revokeSessions(u.ID, "test_user_expired", ""); err != nil {
				return err
			}
			if err := c.flip(watch, "test_user_extended", "test_user_expired"); err != nil {
				return err
			}
			if err := c.emit("user.expired", nil, map[string]any{"user_id": u.ID, "expires_at": u.ExpiresAt}); err != nil {
				return err
			}
			if err := c.audit(Audit{Action: "user.expired", TargetType: "user", TargetID: u.ID, TargetUserID: &u.ID,
				After: map[string]any{"expires_at": u.ExpiresAt}}); err != nil {
				return err
			}
			n++
		}
		return nil
	})
	return
}

// ── Erasure cascade (NFR → Hard-delete cascade) ────────────────────────

// RunErasures runs every scheduled erasure whose 7 days have passed. Each
// step is idempotent, and each step commits before the next, so a failed
// run resumes safely from the start.
func (s *Server) RunErasures(ctx context.Context) (int, error) {
	done := 0
	err := eachMode(func(tm bool) error {
		var due []string
		if err := s.system(ctx, tm, db.Settings{}, func(c *Call) error {
			var err error
			due, err = db.All[string](ctx, c.q, `select to_jsonb(e.user_id) from erasure_requests e
				join users u on u.id = e.user_id
				where e.status = 'scheduled' and e.scheduled_for <= now() order by e.scheduled_for limit 50`, nil)
			return err
		}); err != nil {
			return err
		}
		for _, uid := range due {
			if err := s.erase(ctx, tm, uid); err != nil {
				return errf("erasure %s: %w", uid, err)
			}
			done++
		}
		return nil
	})
	return done, err
}

func (s *Server) erase(ctx context.Context, tm bool, uid string) error {
	steps := []struct {
		settings db.Settings
		fn       func(c *Call) error
	}{
		{db.Settings{}, func(c *Call) error { // 1. credentials go, rows stay
			if _, err := c.q.Exec(ctx, `delete from mfa_recovery_codes where user_identity_id in
				(select id from user_identities where user_id = :u)`, db.Args{"u": uid}); err != nil {
				return err
			}
			_, err := c.q.Exec(ctx, `update user_identities set password_hash = null, mfa_secret = null, mfa_pending_secret = null,
				external_subject_id = null, mfa_enabled = false where user_id = :u`, db.Args{"u": uid})
			return err
		}},
		{db.Settings{}, func(c *Call) error { // 2. profile, sessions, tokens
			for _, q := range []string{
				`delete from profiles where user_id = :u`,
				`delete from refresh_tokens where session_id in (select id from sessions where user_id = :u)`,
				`delete from sessions where user_id = :u`,
				`delete from auth_tokens where user_id = :u`,
				// Push: notifications (their deliveries cascade) and devices.
				`delete from notifications where user_id = :u`,
				`delete from devices where user_id = :u`,
			} {
				if _, err := c.q.Exec(ctx, q, db.Args{"u": uid}); err != nil {
					return err
				}
			}
			return nil
		}},
		{db.Settings{}, func(c *Call) error { // 3. AppProfile fully, AppSettings x-pii keys
			if _, err := c.q.Exec(ctx, `update app_profiles set display_handle = null, custom = '{}' where user_id = :u`,
				db.Args{"u": uid}); err != nil {
				return err
			}
			type row struct {
				App    string          `json:"application_id"`
				Schema json.RawMessage `json:"settings_schema"`
			}
			rows, err := db.All[row](ctx, c.q, `select jsonb_build_object('application_id', s.application_id,
				'settings_schema', a.settings_schema) from app_settings s join applications a on a.id = s.application_id
				where s.user_id = :u`, db.Args{"u": uid})
			if err != nil {
				return err
			}
			for _, r := range rows {
				sch, _, _ := schema.Parse(r.Schema)
				if pii := sch.PIIKeys(); len(pii) > 0 {
					if _, err := c.q.Exec(ctx, `update app_settings set overrides = overrides - :keys::text[]
						where user_id = :u and application_id = :a`,
						db.Args{"keys": db.TextArray(pii), "u": uid, "a": r.App}); err != nil {
						return err
					}
				}
			}
			return nil
		}},
		{db.Settings{AuditMaintenance: true}, func(c *Call) error { // 4. redact snapshots, keep the shape
			_, err := c.q.Exec(ctx, `update audit_events set before = null, after = null
				where target_user_id = :u and (before is not null or after is not null)`, db.Args{"u": uid})
			return err
		}},
		{db.Settings{}, func(c *Call) error { // 5. memberships, overrides, email
			orgs, err := db.All[string](ctx, c.q, `select to_jsonb(organization_id) from organization_memberships
				where user_id = :u and role = 'org_admin' and status = 'active'`, db.Args{"u": uid})
			if err != nil {
				return err
			}
			if _, err := c.q.Exec(ctx, `delete from organization_memberships where user_id = :u`, db.Args{"u": uid}); err != nil {
				return err
			}
			for _, org := range orgs {
				// Never leave an Organization unmanageable: promote its
				// longest-standing active member if it lost its last admin.
				if _, err := c.q.Exec(ctx, `update organization_memberships set role = 'org_admin'
					where organization_id = :o and user_id = (select user_id from organization_memberships
						where organization_id = :o and status = 'active' order by joined_at, user_id limit 1)
					and not exists (select 1 from organization_memberships where organization_id = :o
						and role = 'org_admin' and status = 'active')`, db.Args{"o": org}); err != nil {
					return err
				}
			}
			if _, err := c.q.Exec(ctx, `update entitlements set member_overrides = array_remove(member_overrides, :u)
				where :u = any(member_overrides)`, db.Args{"u": uid}); err != nil {
				return err
			}
			if _, err := c.q.Exec(ctx, `update subscriptions set member_overrides = array_remove(member_overrides, :u)
				where :u = any(member_overrides)`, db.Args{"u": uid}); err != nil {
				return err
			}
			_, err = c.q.Exec(ctx, `update users set email = 'erased+' || id || '@invalid.substratal', pending_email = null,
				status = 'deleted', deleted_at = coalesce(deleted_at, now()) where id = :u`, db.Args{"u": uid})
			return err
		}},
		// 6. Entitlement rows stay, pseudonymized by the steps above.
		{db.Settings{}, func(c *Call) error { // 7. complete
			if _, err := c.q.Exec(ctx, `update erasure_requests set status = 'completed', completed_at = now()
				where user_id = :u and status = 'scheduled'`, db.Args{"u": uid}); err != nil {
				return err
			}
			return c.audit(Audit{Action: "user.erased", TargetType: "user", TargetID: uid, TargetUserID: &uid,
				ActorType: "system", ActorID: "system"})
		}},
	}
	for _, st := range steps {
		if err := s.system(ctx, tm, st.settings, st.fn); err != nil {
			return err
		}
	}
	return nil
}

// ── Audit archival (NFR → Audit log lifecycle) ─────────────────────────

// ArchivedEvent is the shape-only record kept in cold storage forever:
// every field but the before/after snapshot.
type ArchivedEvent struct {
	ID             string    `json:"id"`
	Action         string    `json:"action"`
	ActorType      string    `json:"actor_type"`
	ActorID        string    `json:"actor_id"`
	TargetType     string    `json:"target_type"`
	TargetID       string    `json:"target_id"`
	TargetUserID   *string   `json:"target_user_id"`
	ApplicationID  *string   `json:"application_id"`
	OrganizationID *string   `json:"organization_id"`
	TenantID       *string   `json:"tenant_id"`
	RequestID      *string   `json:"request_id"`
	TestMode       bool      `json:"test_mode"`
	Timestamp      time.Time `json:"timestamp"`
}

// Archiver writes a batch of shape-only events to cold storage (S3 Glacier
// Deep Archive in AWS; a local directory otherwise).
type Archiver func(ctx context.Context, batch []ArchivedEvent) error

// AuditHotWindowDays is how long an Audit Event stays in hot storage, the
// same for every Tenant (NFR → Audit log lifecycle).
const AuditHotWindowDays = 365

// ArchiveAudit moves events older than the hot window to cold storage,
// shape only, and deletes them from the hot table.
func (s *Server) ArchiveAudit(ctx context.Context, archive Archiver) (int, error) {
	total := 0
	err := eachMode(func(tm bool) error {
		for {
			var batch []ArchivedEvent
			err := s.system(ctx, tm, db.Settings{AuditMaintenance: true}, func(c *Call) error {
				var err error
				batch, err = db.All[ArchivedEvent](ctx, c.q, `select to_jsonb(a) - 'before' - 'after' from audit_events a
					where a.timestamp < now() - make_interval(days => :days)
					order by a.timestamp limit 1000 for update of a skip locked`, db.Args{"days": int64(AuditHotWindowDays)})
				if err != nil || len(batch) == 0 {
					return err
				}
				if err := archive(ctx, batch); err != nil {
					return err
				}
				ids := make([]string, len(batch))
				for i, e := range batch {
					ids[i] = e.ID
				}
				_, err = c.q.Exec(ctx, `delete from audit_events where id = any(:ids::text[])`, db.Args{"ids": db.TextArray(ids)})
				return err
			})
			if err != nil {
				return err
			}
			total += len(batch)
			if len(batch) < 1000 {
				return nil
			}
		}
	})
	return total, err
}

// ── Cleanup (nightly) ──────────────────────────────────────────────────

// Cleanup removes expired bookkeeping rows and 30-day-old test data.
func (s *Server) Cleanup(ctx context.Context) error {
	if err := s.system(ctx, false, db.Settings{AllModes: true}, func(c *Call) error {
		for _, q := range []string{
			`delete from auth_tokens where expires_at < now() - interval '1 day'`,
			`delete from idempotency_keys where expires_at < now()`,
			`delete from rate_limit_counters where window_start < now() - interval '2 days'`,
			`delete from refresh_tokens where session_id in (select id from sessions
				where coalesce(revoked_at, least(idle_expires_at, absolute_expires_at)) < now() - interval '30 days')`,
			`delete from sessions where coalesce(revoked_at, least(idle_expires_at, absolute_expires_at)) < now() - interval '30 days'`,
			`delete from webhook_deliveries where created_at < now() - interval '30 days'`,
			`delete from webhook_events e where created_at < now() - interval '30 days'
				and not exists (select 1 from webhook_deliveries d where d.event_id = e.id)`,
			`delete from stripe_events where received_at < now() - interval '90 days'`,
			`delete from audit_exports where expires_at < now()`,
		} {
			if _, err := c.q.Exec(ctx, q, nil); err != nil {
				return errf("cleanup %q: %w", strings.Fields(q)[2], err)
			}
		}
		return nil
	}); err != nil {
		return err
	}
	return s.PurgeTestData(ctx)
}

// PurgeTestData deletes test-mode rows older than 30 days (API Keys →
// Test vs. live), children first.
func (s *Server) PurgeTestData(ctx context.Context) error {
	return s.system(ctx, true, db.Settings{AuditMaintenance: true}, func(c *Call) error {
		old := `test_mode and created_at < now() - interval '30 days'`
		users := `(select id from users where ` + old + `)`
		for _, q := range []string{
			`delete from notification_deliveries where ` + old,
			`delete from notifications where ` + old,
			`delete from devices where ` + old,
			`delete from webhook_deliveries where ` + old,
			`delete from webhook_events where ` + old,
			`delete from webhook_subscriptions where ` + old,
			`delete from audit_events where test_mode and timestamp < now() - interval '30 days'`,
			`delete from app_settings where ` + old,
			`delete from app_profiles where ` + old,
			`delete from user_role_assignments where test_mode and assigned_at < now() - interval '30 days'`,
			`delete from entitlements where ` + old,
			`update stripe_events set subscription_id = null where subscription_id in (select id from subscriptions where ` + old + `)`,
			`delete from subscriptions where ` + old,
			`delete from organization_memberships where test_mode and invited_at < now() - interval '30 days'`,
			`delete from roles where ` + old + ` and not exists (select 1 from user_role_assignments a where a.role_id = roles.id)`,
			`delete from api_keys where ` + old,
			`delete from mfa_recovery_codes where user_identity_id in (select id from user_identities where user_id in ` + users + `)`,
			`delete from user_identities where user_id in ` + users,
			`delete from refresh_tokens where session_id in (select id from sessions where user_id in ` + users + `)`,
			`delete from sessions where user_id in ` + users,
			`delete from auth_tokens where user_id in ` + users,
			`delete from erasure_requests where user_id in ` + users,
			`delete from profiles where user_id in ` + users,
			`delete from settings where user_id in ` + users,
		} {
			if _, err := c.q.Exec(ctx, q, nil); err != nil {
				return err
			}
		}
		// Users and Organizations last, only once nothing references them.
		for _, q := range []string{
			`delete from organizations o where ` + old + ` and not exists (select 1 from organization_memberships m where m.organization_id = o.id)
				and not exists (select 1 from entitlements e where e.organization_id = o.id)
				and not exists (select 1 from applications a where a.owner_organization_id = o.id)
				and not exists (select 1 from tenants t where t.owner_organization_id = o.id)
				and not exists (select 1 from subscriptions s where s.organization_id = o.id)`,
			`delete from users u where ` + old + ` and not exists (select 1 from organization_memberships m where m.user_id = u.id)
				and not exists (select 1 from entitlements e where e.user_id = u.id)
				and not exists (select 1 from applications a where a.owner_user_id = u.id)
				and not exists (select 1 from tenants t where t.owner_user_id = u.id)
				and not exists (select 1 from organizations o where o.created_by = u.id)
				and not exists (select 1 from subscriptions s where s.user_id = u.id)`,
		} {
			if _, err := c.q.Exec(ctx, q, nil); err != nil {
				return err
			}
		}
		return nil
	})
}

// RebuildAllProjections rebuilds every Application's typed settings view
// (nightly safety net for a rebuild that failed after a schema change).
func (s *Server) RebuildAllProjections(ctx context.Context) error {
	var apps []string
	if err := s.system(ctx, false, db.Settings{}, func(c *Call) error {
		var err error
		apps, err = db.All[string](ctx, c.q, `select to_jsonb(id) from applications order by id`, nil)
		return err
	}); err != nil {
		return err
	}
	for _, a := range apps {
		if err := RebuildSettingsProjection(ctx, s.DB, a); err != nil {
			return err
		}
	}
	return nil
}

var _ = httpx.ErrInternal
