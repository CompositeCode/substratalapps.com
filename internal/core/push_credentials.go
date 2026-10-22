package core

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/CompositeCode/substratalapps.com/internal/api/gen"
	"github.com/CompositeCode/substratalapps.com/internal/db"
	"github.com/CompositeCode/substratalapps.com/internal/email"
	"github.com/CompositeCode/substratalapps.com/internal/httpx"
	"github.com/CompositeCode/substratalapps.com/internal/ids"
	"github.com/CompositeCode/substratalapps.com/internal/push"
)

// pushCredential is the API view of an Application's credential for one
// provider (API Reference → Push Notifications → Push credentials). Key
// material is never part of it.
type pushCredential struct {
	ID             string     `json:"id"`
	ApplicationID  string     `json:"application_id"`
	Provider       string     `json:"provider"`
	Status         string     `json:"status"`
	TeamID         string     `json:"team_id,omitempty"`
	KeyID          string     `json:"key_id,omitempty"`
	Topic          string     `json:"topic,omitempty"`
	ProjectID      string     `json:"project_id,omitempty"`
	ClientEmail    string     `json:"client_email,omitempty"`
	Subject        string     `json:"subject,omitempty"`
	VAPIDPublicKey string     `json:"vapid_public_key,omitempty"`
	FailureReason  *string    `json:"failure_reason"`
	FailedAt       *time.Time `json:"failed_at"`
	LastUsedAt     *time.Time `json:"last_used_at"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	version        int
	secret         string
}

// credConfig is the non-secret part, stored as config jsonb.
type credConfig struct {
	TeamID         string `json:"team_id,omitempty"`
	KeyID          string `json:"key_id,omitempty"`
	Topic          string `json:"topic,omitempty"`
	ProjectID      string `json:"project_id,omitempty"`
	ClientEmail    string `json:"client_email,omitempty"`
	Subject        string `json:"subject,omitempty"`
	VAPIDPublicKey string `json:"vapid_public_key,omitempty"`
}

func (c *Call) queryCredentials(where string, args db.Args) ([]pushCredential, error) {
	type row struct {
		ID            string     `json:"id"`
		ApplicationID string     `json:"application_id"`
		Provider      string     `json:"provider"`
		Status        string     `json:"status"`
		Config        credConfig `json:"config"`
		Secret        string     `json:"secret_ciphertext"`
		FailureReason *string    `json:"failure_reason"`
		FailedAt      *time.Time `json:"failed_at"`
		LastUsedAt    *time.Time `json:"last_used_at"`
		CreatedAt     time.Time  `json:"created_at"`
		UpdatedAt     time.Time  `json:"updated_at"`
		Version       int        `json:"version"`
	}
	rows, err := db.All[row](c.ctx, c.q, `select to_jsonb(p) from push_credentials p where `+where, args)
	if err != nil {
		return nil, err
	}
	out := make([]pushCredential, len(rows))
	for i, r := range rows {
		out[i] = pushCredential{ID: r.ID, ApplicationID: r.ApplicationID, Provider: r.Provider, Status: r.Status,
			TeamID: r.Config.TeamID, KeyID: r.Config.KeyID, Topic: r.Config.Topic, ProjectID: r.Config.ProjectID,
			ClientEmail: r.Config.ClientEmail, Subject: r.Config.Subject, VAPIDPublicKey: r.Config.VAPIDPublicKey,
			FailureReason: r.FailureReason, FailedAt: r.FailedAt, LastUsedAt: r.LastUsedAt,
			CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, version: r.Version, secret: r.Secret}
	}
	return out, nil
}

func (p pushCredential) config() credConfig {
	return credConfig{TeamID: p.TeamID, KeyID: p.KeyID, Topic: p.Topic, ProjectID: p.ProjectID,
		ClientEmail: p.ClientEmail, Subject: p.Subject, VAPIDPublicKey: p.VAPIDPublicKey}
}

// decrypt rebuilds the provider credential the push package signs with.
func (s *Server) decryptCredential(ctx context.Context, p pushCredential) (push.Credential, error) {
	secret, err := s.Encrypter.Decrypt(ctx, p.secret)
	if err != nil {
		return push.Credential{}, err
	}
	cred := push.Credential{Provider: p.Provider, TeamID: p.TeamID, KeyID: p.KeyID, Topic: p.Topic,
		Subject: p.Subject, VAPIDPublicKey: p.VAPIDPublicKey}
	switch p.Provider {
	case push.APNs:
		cred.PrivateKey = secret
	case push.FCM:
		sa, err := push.ParseServiceAccount([]byte(secret))
		if err != nil {
			return cred, err
		}
		cred.ServiceAccount = sa
	case push.WebPush:
		cred.VAPIDPrivate = secret
	}
	return cred, nil
}

// credentialApp loads an Application for push-credential access: its
// owner, its own app keys (read only), and applications.manage. Anyone
// else gets 404, so the configuration's existence isn't leaked.
func (c *Call) credentialApp(appID string, write bool) (appRow, error) {
	a, err := c.loadApp(appID)
	if err == db.ErrNotFound {
		return a, httpx.NotFoundResource("application")
	}
	if err != nil {
		return a, err
	}
	if c.can("applications.manage") {
		return a, nil
	}
	if key, ok := c.p.AppKey(); ok {
		if key != appID {
			return a, httpx.NotFoundResource("application")
		}
		if write {
			return a, httpx.ErrUserTokenRequired()
		}
		return a, nil
	}
	owner, err := c.isAppOwner(a)
	if err != nil {
		return a, err
	}
	if !owner {
		if write {
			return a, httpx.Forbidden("applications.manage")
		}
		return a, httpx.NotFoundResource("application")
	}
	return a, nil
}

func validProvider(p string) bool { return slices.Contains(push.Providers, p) }

func invalidCredential(why string) error {
	return httpx.E(422, "invalid_push_credential", "The push credential isn't valid: "+why+".")
}

// credentialInput is the provider-specific body of a POST or PATCH.
type credentialInput struct {
	TeamID         *string         `json:"team_id"`
	KeyID          *string         `json:"key_id"`
	Topic          *string         `json:"topic"`
	PrivateKey     *string         `json:"private_key"`
	ServiceAccount json.RawMessage `json:"service_account"`
	Subject        *string         `json:"subject"`
}

var providerFields = map[string][]string{
	push.APNs:    {"team_id", "key_id", "topic", "private_key"},
	push.FCM:     {"service_account"},
	push.WebPush: {"subject"},
}

// applyCredential validates in against provider and folds it into cfg,
// returning the new secret when key material changed ("" otherwise).
// create requires every field the provider needs.
func applyCredential(provider string, raw httpx.Patch, in credentialInput, cfg *credConfig, create bool) (string, error) {
	var f httpx.Fields
	for k := range raw {
		if k == "provider" || k == "status" {
			continue
		}
		if !slices.Contains(providerFields[provider], k) {
			f.Add(k, "undeclared_key").Message = "not a field of a " + provider + " credential"
		}
	}
	if err := f.Err(); err != nil {
		return "", err
	}
	need := func(field string, v *string) bool {
		if v == nil || strings.TrimSpace(*v) == "" {
			if create || v != nil {
				f.Add(field, "required")
			}
			return false
		}
		return true
	}
	secret := ""
	switch provider {
	case push.APNs:
		if need("team_id", in.TeamID) {
			if len(*in.TeamID) != 10 {
				f.Add("team_id", "invalid_format").Message = "10 characters"
			}
			cfg.TeamID = *in.TeamID
		}
		if need("key_id", in.KeyID) {
			if len(*in.KeyID) != 10 {
				f.Add("key_id", "invalid_format").Message = "10 characters"
			}
			cfg.KeyID = *in.KeyID
		}
		if need("topic", in.Topic) {
			if len(*in.Topic) > 255 {
				f.Max("topic", "too_long", 255)
			}
			cfg.Topic = *in.Topic
		}
		if need("private_key", in.PrivateKey) {
			if err := f.Err(); err != nil {
				return "", err
			}
			if _, err := push.ParseAPNsKey(*in.PrivateKey); err != nil {
				return "", invalidCredential("private_key: " + err.Error())
			}
			secret = *in.PrivateKey
		}
	case push.FCM:
		if len(in.ServiceAccount) == 0 || string(in.ServiceAccount) == "null" {
			if create || raw.Has("service_account") {
				f.Add("service_account", "required")
			}
			break
		}
		sa, err := push.ParseServiceAccount(in.ServiceAccount)
		if err != nil {
			return "", invalidCredential("service_account: " + err.Error())
		}
		cfg.ProjectID, cfg.ClientEmail = sa.ProjectID, sa.ClientEmail
		secret = string(in.ServiceAccount)
	case push.WebPush:
		if need("subject", in.Subject) {
			s := *in.Subject
			if len(s) > 255 || !(strings.HasPrefix(s, "mailto:") || strings.HasPrefix(s, "https://")) {
				f.Add("subject", "invalid_format").Allowed = []string{"mailto:", "https://"}
			}
			cfg.Subject = s
		}
		if create {
			pub, priv, err := push.GenerateVAPID()
			if err != nil {
				return "", err
			}
			cfg.VAPIDPublicKey, secret = pub, priv
		}
	}
	return secret, f.Err()
}

func (s *Server) PushCredentialsList(w http.ResponseWriter, r *http.Request, id string) {
	s.serve(w, r, opts{op: "pushCredentials.list"}, func(c *Call) error {
		a, err := c.credentialApp(id, false)
		if err != nil {
			return err
		}
		creds, err := c.queryCredentials("p.application_id = :a order by p.provider", db.Args{"a": a.ID})
		if err != nil {
			return err
		}
		return c.OK(map[string]any{"data": creds})
	})
}

func (s *Server) PushCredentialsGet(w http.ResponseWriter, r *http.Request, id string, provider gen.PushProvider) {
	s.serve(w, r, opts{op: "pushCredentials.get"}, func(c *Call) error {
		a, err := c.credentialApp(id, false)
		if err != nil {
			return err
		}
		p, err := c.loadCredential(a.ID, string(provider))
		if err != nil {
			return err
		}
		c.Versioned(p.version)
		return c.OK(p)
	})
}

func (c *Call) loadCredential(appID, provider string) (pushCredential, error) {
	if !validProvider(provider) {
		return pushCredential{}, httpx.NotFoundResource("push_credential")
	}
	creds, err := c.queryCredentials("p.application_id = :a and p.provider = :p", db.Args{"a": appID, "p": provider})
	if err != nil {
		return pushCredential{}, err
	}
	if len(creds) == 0 {
		return pushCredential{}, httpx.NotFoundResource("push_credential")
	}
	return creds[0], nil
}

func (s *Server) PushCredentialsCreate(w http.ResponseWriter, r *http.Request, id string, params gen.PushCredentialsCreateParams) {
	s.serve(w, r, opts{op: "pushCredentials.create", userOnly: true, idem: idemOptional}, func(c *Call) error {
		a, err := c.credentialApp(id, true)
		if err != nil {
			return err
		}
		raw, err := c.Patch()
		if err != nil {
			return err
		}
		var in struct {
			Provider string `json:"provider"`
			credentialInput
		}
		if err := c.Decode(&in); err != nil {
			return err
		}
		if !validProvider(in.Provider) {
			var f httpx.Fields
			if in.Provider == "" {
				f.Add("provider", "required")
			} else {
				f.Enum("provider", push.Providers...)
			}
			return f.Err()
		}
		t, err := c.loadTenant(a.TenantID)
		if err != nil {
			return err
		}
		if err := tenantWritable(t); err != nil {
			return err
		}
		var cfg credConfig
		secret, err := applyCredential(in.Provider, raw, in.credentialInput, &cfg, true)
		if err != nil {
			return err
		}
		if existing, err := c.queryCredentials("p.application_id = :a and p.provider = :p",
			db.Args{"a": a.ID, "p": in.Provider}); err != nil {
			return err
		} else if len(existing) > 0 {
			return httpx.Conflict("push_credential_exists", "This Application already has a "+in.Provider+" credential; PATCH it instead.")
		}
		enc, err := c.s.Encrypter.Encrypt(c.ctx, secret)
		if err != nil {
			return err
		}
		credID := ids.New(ids.PushCredential)
		if _, err := c.q.Exec(c.ctx, `insert into push_credentials (id, application_id, tenant_id, provider, config, secret_ciphertext)
			values (:id, :a, :t, :p, :cfg::jsonb, :sec)`,
			db.Args{"id": credID, "a": a.ID, "t": a.TenantID, "p": in.Provider, "cfg": db.JSON(cfg), "sec": enc}); err != nil {
			if _, ok := db.UniqueViolation(err); ok {
				return httpx.Conflict("push_credential_exists", "This Application already has a "+in.Provider+" credential; PATCH it instead.")
			}
			return err
		}
		out, err := c.loadCredential(a.ID, in.Provider)
		if err != nil {
			return err
		}
		if err := c.audit(Audit{Action: "push_credential.created", TargetType: "push_credential", TargetID: out.ID,
			ApplicationID: &a.ID, After: out}); err != nil {
			return err
		}
		c.Versioned(out.version)
		return c.Created(out)
	})
}

func (s *Server) PushCredentialsUpdate(w http.ResponseWriter, r *http.Request, id string, provider gen.PushProvider, params gen.PushCredentialsUpdateParams) {
	s.serve(w, r, opts{op: "pushCredentials.update", userOnly: true}, func(c *Call) error {
		a, err := c.credentialApp(id, true)
		if err != nil {
			return err
		}
		before, err := c.loadCredential(a.ID, string(provider))
		if err != nil {
			return err
		}
		if err := httpx.CheckIfMatch(c.r, before.version); err != nil {
			return err
		}
		raw, err := c.Patch()
		if err != nil {
			return err
		}
		if err := raw.RejectReadOnly("id", "application_id", "provider", "vapid_public_key", "project_id", "client_email",
			"failure_reason", "failed_at", "last_used_at", "created_at", "updated_at"); err != nil {
			return err
		}
		var in struct {
			Status *string `json:"status"`
			credentialInput
		}
		if err := c.Decode(&in); err != nil {
			return err
		}
		if in.Status != nil && *in.Status != "active" {
			return httpx.Conflict("invalid_status_transition", "A credential's status can only go from failed to active.")
		}
		t, err := c.loadTenant(a.TenantID)
		if err != nil {
			return err
		}
		if err := tenantWritable(t); err != nil {
			return err
		}
		cfg := before.config()
		secret, err := applyCredential(before.Provider, raw, in.credentialInput, &cfg, false)
		if err != nil {
			return err
		}
		// Any change to key material, or an explicit status: active, makes
		// a failed credential active again.
		reactivate := secret != "" || in.Status != nil
		args := db.Args{"id": before.ID, "cfg": db.JSON(cfg), "react": reactivate, "sec": nil}
		if secret != "" {
			if args["sec"], err = c.s.Encrypter.Encrypt(c.ctx, secret); err != nil {
				return err
			}
		}
		if _, err := c.q.Exec(c.ctx, `update push_credentials set config = :cfg::jsonb,
				secret_ciphertext = coalesce(:sec, secret_ciphertext),
				status = case when :react then 'active' else status end,
				failure_reason = case when :react then null else failure_reason end,
				failed_at = case when :react then null else failed_at end
			where id = :id`, args); err != nil {
			return err
		}
		after, err := c.loadCredential(a.ID, before.Provider)
		if err != nil {
			return err
		}
		if err := c.audit(Audit{Action: "push_credential.updated", TargetType: "push_credential", TargetID: after.ID,
			ApplicationID: &a.ID, Before: before, After: after}); err != nil {
			return err
		}
		c.Versioned(after.version)
		return c.OK(after)
	})
}

func (s *Server) PushCredentialsDelete(w http.ResponseWriter, r *http.Request, id string, provider gen.PushProvider) {
	s.serve(w, r, opts{op: "pushCredentials.delete", userOnly: true}, func(c *Call) error {
		a, err := c.credentialApp(id, true)
		if err != nil {
			return err
		}
		before, err := c.loadCredential(a.ID, string(provider))
		if err != nil {
			return err
		}
		if _, err := c.q.Exec(c.ctx, `delete from push_credentials where id = :id`, db.Args{"id": before.ID}); err != nil {
			return err
		}
		if err := c.audit(Audit{Action: "push_credential.deleted", TargetType: "push_credential", TargetID: before.ID,
			ApplicationID: &a.ID, Before: before}); err != nil {
			return err
		}
		// One credential serves both modes, but a request sees only its
		// own; the effects on devices and pending deliveries run for both
		// right after commit.
		appID, prov := a.ID, before.Provider
		c.After(func(ctx context.Context) {
			if err := c.s.credentialRemoved(ctx, appID, prov); err != nil {
				c.s.Log.Error("push credential removal effects", "application", appID, "provider", prov, "err", err)
			}
		})
		return c.NoContent()
	})
}

// credentialRemoved skips pending deliveries that depended on a deleted
// credential and, for web_push, deactivates the Application's browser
// subscriptions, which were bound to the deleted VAPID key.
func (s *Server) credentialRemoved(ctx context.Context, appID, provider string) error {
	return eachMode(func(tm bool) error {
		return s.system(ctx, tm, db.Settings{}, func(c *Call) error {
			notes, err := db.All[string](ctx, c.q, `update notification_deliveries d set status = 'skipped', error = 'credential_removed',
					next_attempt_at = null
				from notifications n where n.id = d.notification_id and d.status = 'pending' and d.provider = :p
				  and (n.application_id = :a or d.device_id in (select id from devices where application_id = :a))
				returning to_jsonb(d.notification_id)`, db.Args{"a": appID, "p": provider})
			if err != nil {
				return err
			}
			for _, n := range dedupe(notes) {
				if err := c.settleNotification(n); err != nil {
					return err
				}
			}
			if provider == push.WebPush {
				return c.deactivateDevices("d.application_id = :a and d.provider = 'web_push'", db.Args{"a": appID}, "credential_removed")
			}
			return nil
		})
	})
}

// credentialRejected marks a credential failed after the provider refused
// it, records why, and tells the Application's owner, once per failure.
func (c *Call) credentialRejected(credID, reason string) error {
	var cred struct {
		ApplicationID string `json:"application_id"`
		Provider      string `json:"provider"`
	}
	err := db.One(c.ctx, c.q, &cred, `update push_credentials set status = 'failed', failure_reason = :r, failed_at = now()
		where id = :id and status = 'active' returning jsonb_build_object('application_id', application_id, 'provider', provider)`,
		db.Args{"id": credID, "r": reason})
	if err == db.ErrNotFound {
		return nil // already failed, or deleted meanwhile
	}
	if err != nil {
		return err
	}
	app := cred.ApplicationID
	if err := c.audit(Audit{Action: "push_credential.updated", TargetType: "push_credential", TargetID: credID,
		ApplicationID: &app, Before: map[string]any{"status": "active"},
		After: map[string]any{"status": "failed", "failure_reason": reason}, ActorType: "system", ActorID: "system"}); err != nil {
		return err
	}
	if err := c.emit("push_credential.failed", &app, map[string]any{"provider": cred.Provider, "failure_reason": reason}); err != nil {
		return err
	}
	// Owners are live Users, invisible to a test-mode transaction, so the
	// email goes out from a live-mode read after commit.
	provider := cred.Provider
	c.After(func(ctx context.Context) { c.s.emailCredentialOwners(ctx, app, provider, reason) })
	return nil
}

func (s *Server) emailCredentialOwners(ctx context.Context, app, provider, reason string) {
	var owners []string
	err := s.system(ctx, false, db.Settings{}, func(c *Call) error {
		var err error
		owners, err = db.All[string](ctx, c.q, `select to_jsonb(u.email) from applications a join users u on u.id = a.owner_user_id
				where a.id = :a
			union select to_jsonb(u.email) from applications a
				join organization_memberships m on m.organization_id = a.owner_organization_id and m.role = 'org_admin' and m.status = 'active'
				join users u on u.id = m.user_id where a.id = :a`, db.Args{"a": app})
		return err
	})
	if err != nil {
		s.Log.Error("push credential owners", "application", app, "err", err)
		return
	}
	for _, to := range owners {
		if err := s.Email.Send(ctx, email.Message{To: to, Kind: "push_credential_failed",
			Subject: "A push credential for " + app + " was rejected",
			Text: "The " + provider + " push credential for " + app + " was rejected by the provider (" + reason + "). " +
				"Pushes to that provider are paused until you replace it: PATCH /applications/" + app + "/push-credentials/" + provider + "."}); err != nil {
			s.Log.Error("email push credential owner", "application", app, "err", err)
		}
	}
}
