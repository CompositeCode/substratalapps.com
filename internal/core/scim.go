package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/CompositeCode/substratalapps.com/internal/db"
	"github.com/CompositeCode/substratalapps.com/internal/httpx"
	"github.com/CompositeCode/substratalapps.com/internal/ids"
)

// SCIM 2.0 provisioning (API Reference → SCIM): an Organization's identity
// provider invites and removes its members with a per-Organization bearer
// token. It drives the same membership rules an org admin does: a
// provisioned person gets a pending membership they accept themselves, and
// deprovisioning removes the membership (and with it the Organization's
// grants), never the person's own account.

type scimTokenRow struct {
	ID             string     `json:"id"`
	OrganizationID string     `json:"organization_id"`
	CreatedBy      string     `json:"created_by"`
	TestMode       bool       `json:"test_mode"`
	CreatedAt      time.Time  `json:"created_at"`
	LastUsedAt     *time.Time `json:"last_used_at"`
}

type scimToken struct {
	scimTokenRow
	Token   string `json:"token,omitempty"`
	BaseURL string `json:"base_url"`
}

func (s *Server) scimBaseURL() string { return strings.TrimRight(s.PublicBaseURL, "/") + "/scim/v2" }

// scimTokenAdmin loads the Organization for an org_admin or
// organizations.manage caller.
func (c *Call) scimTokenAdmin(id string) (orgRow, error) {
	o, m, err := c.orgReadable(id)
	if err != nil {
		return o, err
	}
	return o, c.orgAdminWrite(o, m)
}

func (c *Call) loadScimToken(orgID string) (scimTokenRow, error) {
	var t scimTokenRow
	err := db.One(c.ctx, c.q, &t, `select to_jsonb(t) - 'secret_hash' from scim_tokens t where organization_id = :o`,
		db.Args{"o": orgID})
	if err == db.ErrNotFound {
		return t, httpx.NotFoundResource("scim_token")
	}
	return t, err
}

func (s *Server) OrganizationsGetScimToken(w http.ResponseWriter, r *http.Request, id string) {
	s.serve(w, r, opts{op: "organizations.getScimToken"}, func(c *Call) error {
		if _, err := c.scimTokenAdmin(id); err != nil {
			return err
		}
		t, err := c.loadScimToken(id)
		if err != nil {
			return err
		}
		return c.OK(scimToken{scimTokenRow: t, BaseURL: c.s.scimBaseURL()})
	})
}

func (s *Server) OrganizationsCreateScimToken(w http.ResponseWriter, r *http.Request, id string) {
	s.serve(w, r, opts{op: "organizations.createScimToken", userOnly: true}, func(c *Call) error {
		o, err := c.scimTokenAdmin(id)
		if err != nil {
			return err
		}
		if _, err := c.q.Exec(c.ctx, `delete from scim_tokens where organization_id = :o`, db.Args{"o": o.ID}); err != nil {
			return err
		}
		secret := ids.Secret(ids.SCIMSecret)
		var t scimTokenRow
		if err := db.One(c.ctx, c.q, &t, `insert into scim_tokens (id, organization_id, secret_hash, created_by, test_mode)
			values (:id, :o, :h, :by, :tm) returning to_jsonb(scim_tokens) - 'secret_hash'`,
			db.Args{"id": ids.New(ids.SCIMToken), "o": o.ID, "h": ids.Hash(secret), "by": c.p.UserID, "tm": c.testMode()}); err != nil {
			return err
		}
		if err := c.audit(Audit{Action: "organization.scim_token_created", TargetType: "organization", TargetID: o.ID,
			OrganizationID: &o.ID, After: map[string]any{"scim_token_id": t.ID}}); err != nil {
			return err
		}
		return c.Created(scimToken{scimTokenRow: t, Token: secret, BaseURL: c.s.scimBaseURL()})
	})
}

func (s *Server) OrganizationsDeleteScimToken(w http.ResponseWriter, r *http.Request, id string) {
	s.serve(w, r, opts{op: "organizations.deleteScimToken"}, func(c *Call) error {
		o, err := c.scimTokenAdmin(id)
		if err != nil {
			return err
		}
		t, err := c.loadScimToken(o.ID)
		if err != nil {
			return err
		}
		if _, err := c.q.Exec(c.ctx, `delete from scim_tokens where id = :id`, db.Args{"id": t.ID}); err != nil {
			return err
		}
		if err := c.audit(Audit{Action: "organization.scim_token_revoked", TargetType: "organization", TargetID: o.ID,
			OrganizationID: &o.ID, Before: map[string]any{"scim_token_id": t.ID}}); err != nil {
			return err
		}
		return c.NoContent()
	})
}

// ── The SCIM endpoints (/scim/v2) ──────────────────────────────────────

const (
	scimUserSchema   = "urn:ietf:params:scim:schemas:core:2.0:User"
	scimListSchema   = "urn:ietf:params:scim:api:messages:2.0:ListResponse"
	scimErrorSchema  = "urn:ietf:params:scim:api:messages:2.0:Error"
	scimPatchSchema  = "urn:ietf:params:scim:api:messages:2.0:PatchOp"
	scimMemberSchema = "urn:substratal:params:scim:schemas:extension:2.0:Membership"
	scimContentType  = "application/scim+json"
	scimMaxCount     = 200
)

// scimError is an RFC 7644 §3.12 error.
type scimError struct {
	Status   int
	ScimType string
	Detail   string
}

func (e *scimError) Error() string { return e.Detail }

func scimErr(status int, scimType, detail string) error {
	return &scimError{Status: status, ScimType: scimType, Detail: detail}
}

// scimHandler serves /scim/v2.
func (s *Server) scimHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /scim/v2/ServiceProviderConfig", s.scim(s.scimServiceProviderConfig))
	mux.HandleFunc("GET /scim/v2/ResourceTypes", s.scim(s.scimResourceTypes))
	mux.HandleFunc("GET /scim/v2/Users", s.scim(s.scimListUsers))
	mux.HandleFunc("POST /scim/v2/Users", s.scim(s.scimCreateUser))
	mux.HandleFunc("GET /scim/v2/Users/{id}", s.scim(s.scimGetUser))
	mux.HandleFunc("PUT /scim/v2/Users/{id}", s.scim(s.scimReplaceUser))
	mux.HandleFunc("PATCH /scim/v2/Users/{id}", s.scim(s.scimPatchUser))
	mux.HandleFunc("DELETE /scim/v2/Users/{id}", s.scim(s.scimDeleteUser))
	mux.HandleFunc("/scim/v2/", func(w http.ResponseWriter, r *http.Request) {
		writeScimError(w, &scimError{Status: 404, Detail: "This SCIM resource isn't supported; only Users is."})
	})
	return mux
}

type scimFn func(c *Call, o orgRow) (int, any, error)

// scim runs fn for a request carrying an Organization's SCIM token, in one
// transaction pinned to the token's mode. Writes are attributed to the
// token: actor type system, actor id the token's sct_ id.
func (s *Server) scim(fn scimFn) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
		defer cancel()
		c := &Call{s: s, ctx: ctx, r: r, op: opts{op: "scim"}, now: s.Now().UTC().Truncate(time.Microsecond),
			headers: http.Header{}, ip: clientIP(r), requestID: RequestID(r.Context())}
		raw, err := httpx.ReadBody(r)
		if err != nil {
			writeScimError(w, err)
			return
		}
		c.raw = raw
		r.Body = io.NopCloser(bytes.NewReader(raw))
		var status int
		var body any
		var fnErr error
		txErr := s.DB.Tx(ctx, db.Settings{AllModes: true}, func(q db.Querier) error {
			c.q = q
			tok, err := bearer(c)
			if err != nil || !strings.HasPrefix(tok, ids.SCIMSecret) {
				fnErr = scimErr(401, "", "A valid SCIM bearer token is required.")
				return fnErr
			}
			var t scimTokenRow
			if err := db.One(ctx, q, &t, `select to_jsonb(t) - 'secret_hash' from scim_tokens t where secret_hash = :h`,
				db.Args{"h": ids.Hash(tok)}); err == db.ErrNotFound {
				fnErr = scimErr(401, "", "A valid SCIM bearer token is required.")
				return fnErr
			} else if err != nil {
				return err
			}
			c.p = &Principal{Type: "system", ID: t.ID, TestMode: t.TestMode, Perms: map[string]bool{}}
			if err := c.setMode(t.TestMode); err != nil {
				return err
			}
			if err := c.hit("scim:"+t.ID, perMinute(defaultPrincipalLimit), true); err != nil {
				fnErr = err
				return err
			}
			if t.LastUsedAt == nil || c.now.Sub(*t.LastUsedAt) > time.Minute {
				if _, err := q.Exec(ctx, `update scim_tokens set last_used_at = now() where id = :id`, db.Args{"id": t.ID}); err != nil {
					return err
				}
			}
			o, err := c.loadOrg(t.OrganizationID)
			if err != nil {
				return err
			}
			status, body, err = fn(c, o)
			if err != nil {
				fnErr = err
			}
			return err
		})
		if fnErr != nil {
			if _, ok := fnErr.(*scimError); !ok && !isAPIError(fnErr) {
				s.Log.Error("scim request failed", "request_id", c.requestID, "err", fnErr)
			}
			for k, v := range c.headers {
				w.Header()[k] = v
			}
			writeScimError(w, fnErr)
			return
		}
		if txErr != nil {
			s.Log.Error("scim transaction failed", "request_id", c.requestID, "err", txErr)
			writeScimError(w, scimErr(500, "", "Unexpected server error."))
			return
		}
		for _, f := range c.afterCommit {
			f(context.WithoutCancel(ctx))
		}
		for k, v := range c.headers {
			w.Header()[k] = v
		}
		if status == http.StatusNoContent {
			w.WriteHeader(status)
			return
		}
		w.Header().Set("Content-Type", scimContentType)
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(body)
	}
}

// writeScimError writes an RFC 7644 error, mapping the API's own errors.
func writeScimError(w http.ResponseWriter, err error) {
	var se *scimError
	if !errors.As(err, &se) {
		var e *httpx.Error
		if errors.As(err, &e) {
			se = &scimError{Status: e.Status, Detail: e.Message}
			switch {
			case e.Code == "already_member":
				se.ScimType = "uniqueness"
			case e.Code == "last_org_admin":
				se.ScimType = "mutability"
			case e.Status == 400 || e.Status == 422:
				se.ScimType = "invalidValue"
			}
		} else {
			se = &scimError{Status: 500, Detail: "Unexpected server error."}
		}
	}
	out := map[string]any{"schemas": []string{scimErrorSchema}, "status": strconv.Itoa(se.Status), "detail": se.Detail}
	if se.ScimType != "" {
		out["scimType"] = se.ScimType
	}
	w.Header().Set("Content-Type", scimContentType)
	w.WriteHeader(se.Status)
	_ = json.NewEncoder(w).Encode(out)
}

// scimUser is a membership as a SCIM User. active is true for a pending or
// active membership; the extension says which.
func (c *Call) scimUser(m member) map[string]any {
	u := map[string]any{
		"schemas":  []string{scimUserSchema, scimMemberSchema},
		"id":       m.UserID,
		"userName": m.Email,
		"emails":   []map[string]any{{"value": m.Email, "primary": true}},
		"active":   true,
		"meta": map[string]any{"resourceType": "User", "created": m.InvitedAt.UTC().Format(time.RFC3339),
			"location": c.s.scimBaseURL() + "/Users/" + m.UserID},
		scimMemberSchema: map[string]any{"status": m.MembershipStatus, "role": m.Role},
	}
	if m.DisplayName != nil {
		u["displayName"] = *m.DisplayName
	}
	return u
}

func (c *Call) scimMember(o orgRow, id string) (member, error) {
	m, err := c.loadMember(o.ID, id)
	if err != nil {
		return m, scimErr(404, "", "No User with this id is provisioned in this Organization.")
	}
	return m, nil
}

func (s *Server) scimServiceProviderConfig(c *Call, _ orgRow) (int, any, error) {
	return 200, map[string]any{
		"schemas":          []string{"urn:ietf:params:scim:schemas:core:2.0:ServiceProviderConfig"},
		"documentationUri": "https://compositecode.github.io/substratalapps.com/api-reference/scim/",
		"patch":            map[string]any{"supported": true},
		"bulk":             map[string]any{"supported": false, "maxOperations": 0, "maxPayloadSize": 0},
		"filter":           map[string]any{"supported": true, "maxResults": scimMaxCount},
		"changePassword":   map[string]any{"supported": false},
		"sort":             map[string]any{"supported": false},
		"etag":             map[string]any{"supported": false},
		"authenticationSchemes": []map[string]any{{"type": "oauthbearertoken", "name": "Bearer token",
			"description": "The Organization's SCIM token, from POST /v1/organizations/{id}/scim-token."}},
	}, nil
}

func (s *Server) scimResourceTypes(c *Call, _ orgRow) (int, any, error) {
	return 200, map[string]any{"schemas": []string{scimListSchema}, "totalResults": 1, "startIndex": 1, "itemsPerPage": 1,
		"Resources": []map[string]any{{"schemas": []string{"urn:ietf:params:scim:schemas:core:2.0:ResourceType"},
			"id": "User", "name": "User", "endpoint": "/Users", "schema": scimUserSchema,
			"schemaExtensions": []map[string]any{{"schema": scimMemberSchema, "required": false}}}}}, nil
}

var scimFilterRe = regexp.MustCompile(`(?i)^\s*(userName|emails(?:\.value)?)\s+eq\s+"([^"]*)"\s*$`)

func (s *Server) scimListUsers(c *Call, o orgRow) (int, any, error) {
	q := c.r.URL.Query()
	where := []string{"m.organization_id = :o"}
	args := db.Args{"o": o.ID}
	if f := q.Get("filter"); f != "" {
		m := scimFilterRe.FindStringSubmatch(f)
		if m == nil {
			return 0, nil, scimErr(400, "invalidFilter", `Only userName eq "…" (or emails eq "…") is supported.`)
		}
		where = append(where, "lower(u.email::text) = lower(:email)")
		args["email"] = m[2]
	}
	start, count := 1, 100
	if v := q.Get("startIndex"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 1 {
			start = n
		}
	}
	if v := q.Get("count"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			count = min(n, scimMaxCount)
		}
	}
	cond := strings.Join(where, " and ")
	var total int
	if err := db.One(c.ctx, c.q, &total, `select to_jsonb(count(*)) from organization_memberships m
		join users u on u.id = m.user_id where `+cond, args); err != nil {
		return 0, nil, err
	}
	args["lim"], args["off"] = int64(count), int64(start-1)
	rows, err := db.All[member](c.ctx, c.q, memberSQL+" where "+cond+
		" order by m.invited_at, m.user_id limit :lim offset :off", args)
	if err != nil {
		return 0, nil, err
	}
	res := make([]map[string]any, len(rows))
	for i, m := range rows {
		res[i] = c.scimUser(m)
	}
	return 200, map[string]any{"schemas": []string{scimListSchema}, "totalResults": total, "startIndex": start,
		"itemsPerPage": len(res), "Resources": res}, nil
}

type scimUserIn struct {
	UserName string `json:"userName"`
	Emails   []struct {
		Value   string `json:"value"`
		Primary bool   `json:"primary"`
	} `json:"emails"`
	Active *bool `json:"active"`
}

// address is the email to provision: userName, or the primary email.
func (in scimUserIn) address() string {
	if strings.Contains(in.UserName, "@") {
		return in.UserName
	}
	for _, e := range in.Emails {
		if e.Primary {
			return e.Value
		}
	}
	if len(in.Emails) > 0 {
		return in.Emails[0].Value
	}
	return in.UserName
}

func (s *Server) scimCreateUser(c *Call, o orgRow) (int, any, error) {
	var in scimUserIn
	if err := json.Unmarshal(c.raw, &in); err != nil {
		return 0, nil, scimErr(400, "invalidSyntax", "The body isn't a SCIM User.")
	}
	addr, ok := validEmail(in.address())
	if !ok {
		return 0, nil, scimErr(400, "invalidValue", "userName (or a primary email) must be an email address.")
	}
	if in.Active != nil && !*in.Active {
		return 0, nil, scimErr(400, "invalidValue", "A User can't be provisioned inactive.")
	}
	uid, err := c.inviteMember(o, addr, "member")
	if err != nil {
		return 0, nil, err
	}
	m, err := c.loadMember(o.ID, uid)
	if err != nil {
		return 0, nil, err
	}
	return 201, c.scimUser(m), nil
}

func (s *Server) scimGetUser(c *Call, o orgRow) (int, any, error) {
	m, err := c.scimMember(o, c.r.PathValue("id"))
	if err != nil {
		return 0, nil, err
	}
	return 200, c.scimUser(m), nil
}

// scimDeactivate removes the membership; the response shows the final
// state, inactive, and the User is gone from this Organization afterwards.
func (c *Call) scimDeactivate(orgID string, m member) (int, any, error) {
	if err := c.removeMember(orgID, m); err != nil {
		return 0, nil, err
	}
	u := c.scimUser(m)
	u["active"] = false
	return 200, u, nil
}

func (s *Server) scimReplaceUser(c *Call, o orgRow) (int, any, error) {
	m, err := c.scimMember(o, c.r.PathValue("id"))
	if err != nil {
		return 0, nil, err
	}
	var in scimUserIn
	if err := json.Unmarshal(c.raw, &in); err != nil {
		return 0, nil, scimErr(400, "invalidSyntax", "The body isn't a SCIM User.")
	}
	if in.Active != nil && !*in.Active {
		return c.scimDeactivate(o.ID, m)
	}
	return 200, c.scimUser(m), nil
}

func (s *Server) scimPatchUser(c *Call, o orgRow) (int, any, error) {
	m, err := c.scimMember(o, c.r.PathValue("id"))
	if err != nil {
		return 0, nil, err
	}
	var in struct {
		Operations []struct {
			Op    string          `json:"op"`
			Path  string          `json:"path"`
			Value json.RawMessage `json:"value"`
		} `json:"Operations"`
	}
	if err := json.Unmarshal(c.raw, &in); err != nil || len(in.Operations) == 0 {
		return 0, nil, scimErr(400, "invalidSyntax", "The body isn't a SCIM PatchOp.")
	}
	deactivate := false
	for _, op := range in.Operations {
		switch strings.ToLower(op.Op) {
		case "replace", "add":
		default:
			continue // remove: nothing this API stores can be removed by SCIM
		}
		var active *bool
		if strings.EqualFold(op.Path, "active") {
			active = scimBool(op.Value)
		} else if op.Path == "" {
			var v map[string]json.RawMessage
			if json.Unmarshal(op.Value, &v) == nil {
				for k, raw := range v {
					if strings.EqualFold(k, "active") {
						active = scimBool(raw)
					}
				}
			}
		}
		if active != nil && !*active {
			deactivate = true
		}
	}
	if deactivate {
		return c.scimDeactivate(o.ID, m)
	}
	return 200, c.scimUser(m), nil
}

// scimBool reads a SCIM boolean, which some identity providers send as a
// string ("False").
func scimBool(raw json.RawMessage) *bool {
	var b bool
	if json.Unmarshal(raw, &b) == nil {
		return &b
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		if v, err := strconv.ParseBool(strings.ToLower(s)); err == nil {
			return &v
		}
	}
	return nil
}

func (s *Server) scimDeleteUser(c *Call, o orgRow) (int, any, error) {
	m, err := c.scimMember(o, c.r.PathValue("id"))
	if err != nil {
		return 0, nil, err
	}
	if err := c.removeMember(o.ID, m); err != nil {
		return 0, nil, err
	}
	return http.StatusNoContent, nil, nil
}
