package core

import (
	"context"
	"fmt"

	"github.com/CompositeCode/substratalapps.com/internal/auth"
	"github.com/CompositeCode/substratalapps.com/internal/db"
	"github.com/CompositeCode/substratalapps.com/internal/ids"
)

// BootstrapSuperadmin creates (or promotes) the first superadmin directly
// against the database. The public API can't, by design: nothing can hold
// roles.manage before a superadmin exists to grant it (NFR →
// Authentication → Bootstrapping). Only cmd/admin and tests call this.
func (s *Server) BootstrapSuperadmin(ctx context.Context, email, password string) (string, error) {
	addr, ok := validEmail(email)
	if !ok {
		return "", fmt.Errorf("bootstrap: invalid email %q", email)
	}
	if err := auth.CheckPolicy(password, addr); err != nil {
		return "", fmt.Errorf("bootstrap: %w", err)
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return "", err
	}
	var userID string
	err = s.system(ctx, false, db.Settings{}, func(c *Call) error {
		u, err := c.findUser("u.email = :e and u.deleted_at is null", db.Args{"e": addr})
		switch err {
		case nil:
			userID = u.ID
			if _, err := c.setPassword(u.ID, hash); err != nil {
				return err
			}
			if _, err := c.q.Exec(ctx, `update users set status = 'active', email_verified = true where id = :u`, db.Args{"u": u.ID}); err != nil {
				return err
			}
		case db.ErrNotFound:
			nu, err := c.createUser(newUser{Email: addr, Status: "active", EmailVerify: true, PasswordHash: hash,
				ActorType: "system", ActorID: "system"})
			if err != nil {
				return err
			}
			userID = nu.ID
		default:
			return err
		}
		if _, err := c.q.Exec(ctx, `insert into user_role_assignments (user_id, role_id, application_id, assigned_by_type, assigned_by_id)
			values (:u, 'role_platform_superadmin', null, 'system', 'system') on conflict do nothing`, db.Args{"u": userID}); err != nil {
			return err
		}
		return c.audit(Audit{Action: "role.assigned", TargetType: "role_assignment", TargetID: userID + ":role_platform_superadmin",
			TargetUserID: &userID, ActorType: "system", ActorID: "system",
			After: map[string]any{"user_id": userID, "role_id": "role_platform_superadmin", "application_id": nil}})
	})
	return userID, err
}

// EnsureProtectedSuperadmin makes the platform's owner account exist, as an
// active superadmin that can never be removed (migration 0004). It runs on
// every deploy, after migrations (cmd/migrate with SUPERADMIN_EMAIL set), so
// the account is always there and always protected:
//
//   - If no live User has the email, it's created active and verified, with
//     no password. Set one with cmd/admin bootstrap, or the password-reset
//     flow.
//   - If the User exists, it's made active and verified again, marked
//     protected, and given back the superadmin Role if it was missing.
//
// It never changes an existing password, and writes audit events only for
// what it actually changed. It reports whether it created the User.
func (s *Server) EnsureProtectedSuperadmin(ctx context.Context, email string) (userID string, created bool, err error) {
	addr, ok := validEmail(email)
	if !ok {
		return "", false, fmt.Errorf("superadmin: invalid email %q", email)
	}
	err = s.system(ctx, false, db.Settings{}, func(c *Call) error {
		u, err := c.findUser("u.email = :e and u.deleted_at is null", db.Args{"e": addr})
		switch err {
		case nil:
			userID = u.ID
		case db.ErrNotFound:
			nu, err := c.createUser(newUser{Email: addr, Status: "active", EmailVerify: true,
				DisplayName: "Superadmin", ActorType: "system", ActorID: "system"})
			if err != nil {
				return err
			}
			userID, created = nu.ID, true
		default:
			return err
		}
		if _, err := c.q.Exec(ctx, `update users set status = 'active', email_verified = true, protected = true
			where id = :u and (status <> 'active' or not email_verified or not protected)`, db.Args{"u": userID}); err != nil {
			return err
		}
		n, err := c.q.Exec(ctx, `insert into user_role_assignments (user_id, role_id, application_id, assigned_by_type, assigned_by_id)
			values (:u, 'role_platform_superadmin', null, 'system', 'system') on conflict do nothing`, db.Args{"u": userID})
		if err != nil || n == 0 {
			return err
		}
		return c.audit(Audit{Action: "role.assigned", TargetType: "role_assignment", TargetID: userID + ":role_platform_superadmin",
			TargetUserID: &userID, ActorType: "system", ActorID: "system",
			After: map[string]any{"user_id": userID, "role_id": "role_platform_superadmin", "application_id": nil}})
	})
	return userID, created, err
}

var _ = ids.New
