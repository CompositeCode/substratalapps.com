---
layout: default
title: Non-Functional Requirements
nav_order: 10
---

# Non-Functional Requirements
{: .no_toc }

1. TOC
{: toc }

---

## Authentication

- **User-facing login:** native email/password, owned in-house, plus per-Organization SSO architected and deferred. See [Auth → Native auth and per-Organization SSO](../api-reference/auth/#native-auth-and-per-organization-sso). Tokens are RS256 JWTs signed with keys in AWS KMS and published at `/.well-known/jwks.json`. Platform access tokens live 15 minutes, app tokens 5 minutes, and refresh tokens are rotating and single-use with reuse detection.
- **Passwords:** Argon2id (`m=19456 KiB, t=2, p=1`), 12–128 characters, checked against a bundled list of the 100,000 most common breached passwords. No composition rules, no forced rotation (NIST SP 800-63B). There's no password column on `users`; credentials live on [UserIdentity](../domain-model/users-and-organizations/#useridentity).
- **Brute-force protection:** per-account lockout (5 failures in 15 minutes locks the account for 15 minutes), plus per-IP limits on every unauthenticated auth endpoint (see [Rate limiting](#rate-limiting)). Login errors never reveal whether an account exists.
- **MFA:** optional TOTP (RFC 6238) for `password` identities, with 10 single-use recovery codes. TOTP secrets are encrypted with KMS at rest. An SSO identity's MFA belongs to its IdP.
- **Service-to-service:** billing → hub, app → hub, and any other backend caller authenticates with a scoped [API Key](../api-reference/api-keys/), never user credentials. mTLS isn't offered.
- **Secrets at rest:** every bearer credential (refresh tokens, API Key secrets, emailed tokens, recovery codes) is stored only as a hash. Webhook signing secrets, TOTP secrets, and [push credentials](../api-reference/notifications/#push-credentials) must be usable by the server, so they're KMS-encrypted instead. So are push device tokens: they aren't credentials, but they're personal data the server has to send to.
- **Bootstrapping:** the very first `superadmin` cannot be created through the public API — nothing can call `POST /v1/users/{id}/roles/{roleId}` with `roles.manage` before a `superadmin` exists to grant it. It's seeded directly against the database, not specified as an endpoint. Every deploy makes sure the platform owner's account (the `SUPERADMIN_EMAIL` deploy variable) exists as a **protected superadmin**: active, verified, holding `superadmin`, and impossible to suspend, delete, erase, re-address, or demote, through the API ([`409 user_protected`](../api-reference/users/#the-protected-superadmin)) or directly in the database. Its password is set with the one-time admin CLI (`cmd/admin bootstrap`) or the password-reset flow.
- **MCP:** the [MCP Server](../mcp-server/) is not a separate auth surface — it carries and forwards the same Bearer credential (user token or API Key) as any other caller. No new rule in this section applies to it that doesn't already apply here.

## CORS

`GET` endpoints intended for direct browser use by a future first-party UI (the catalog, a user's own entitlements/profile/settings via `me`) allow that UI's origin. Admin and service-to-service endpoints do not allow browser-origin requests at all — an admin console calls through its own backend, not directly from the browser with a platform API key. The exact allow-listed origin(s) are an environment-level config value, not part of this spec.

## Authorization enforcement point

The hub is the **only** writer of Entitlement and Role state. Apps read (via JWT claim, introspection, or webhook) — they never maintain a parallel copy they write to independently. If an app needs a role concept finer than what [App Roles](../domain-model/roles-and-permissions/) supports, that's a sign the app needs a new permission defined in its catalog entry, not a local workaround.

## Multi-tenancy

Three scoping axes. They answer different questions, and are easy to conflate if read too quickly:

- **Test vs. live** — every table a test-mode credential can write carries `test_mode`, and a **Postgres Row-Level Security policy** keyed on a session variable the API sets in every request's transaction (`app.current_test_mode`) keeps test and live rows invisible to each other, on both database backends. It fails closed: a transaction that doesn't set it sees live rows only. This is enforced by the database, not by application `WHERE` clauses a future query could forget.
- **`organization_id`** — team grouping. Every record that can belong to an [Organization](../domain-model/users-and-organizations/#organization) carries it from day one, so it's never a migration later.
- **`tenant_id`** — infrastructure placement. Every record scoped to one [Application](../domain-model/applications/) carries it, denormalized from that Application's own `tenant_id` (see [Tenancy](../domain-model/tenancy/)).

On the `shared` tier, Organization and Tenant scoping are enforced in the data-access layer: app-confined API keys, owner and `org_admin` checks, and explicit `tenant_id` / `organization_id` filters, each covered by integration tests. They aren't RLS policies, because a single request legitimately spans many of both: a User in several Organizations, a platform admin listing every Entitlement, a job sweeping every Tenant. An `isolated` or `dedicated_region` Tenant's rows never share a physical cluster with another Tenant's at all. When the first such Tenant needs a second line of defense inside its own cluster, per-request allowed-set policies (`app.allowed_tenant_ids`, with an explicit platform bypass) are added then. See [Tenant vs. Organization](../domain-model/tenancy/#tenant-vs-organization) for why these are separate columns.

## Concurrency control

Entitlement, Role assignment, and Settings writes can race — two support agents acting on the same user, or a webhook retry landing alongside a manual edit. Mutating endpoints on these resources accept an `If-Match` header carrying the resource's current version (returned as an `ETag` on every `GET`); a write with a stale or missing `ETag` where one was expected returns `409` with `code: "version_conflict"` rather than silently applying a last-write-wins update. `If-Match` is optional on every `PATCH`/`DELETE`; without it, the write is last-write-wins. Endpoints where sending it matters most: `PATCH /v1/entitlements/{id}`, `PATCH /v1/users/{id}/apps/{appId}/settings`, and `PATCH /v1/roles/{id}`. Role *assignment* needs no `If-Match`, because assign and remove are idempotent set operations that can't lose an update.

## Transaction boundaries

Any write described as "also creates" or "also assigns" elsewhere in this spec — most notably `POST /v1/users` creating a default Profile, Settings, and `member` Role assignment in one call — is one database transaction. A partial failure (e.g. the Role assignment insert fails) rolls back the whole call; the client sees a single error, not a half-created User. The same rule applies to `PATCH /v1/entitlements/{id}` and its [Audit Event](../domain-model/orders-and-audit/#audit-event): the state change and the audit record are written together or not at all — an Audit Event that doesn't actually correspond to a committed change is worse than no Audit Event.

## Audit

Every state change listed in the [Action catalog](../domain-model/orders-and-audit/#action-catalog) produces an immutable [Audit Event](../domain-model/orders-and-audit/), written in the same transaction as the change: actor (user, API key, or system), action, target, before/after snapshot, request id, and timestamp. Audit events are never deleted or edited through the API, including when the record they describe is later deleted. Two scheduled jobs are the only writers after insert, and both only reduce an event to its shape: the archival job ([Audit log lifecycle](#audit-log-lifecycle)) and step 4 of the [Hard-delete cascade](#hard-delete-cascade), which redacts `before`/`after`. The database enforces this for every caller, including a direct SQL session: a trigger rejects every `UPDATE` and `DELETE` on `audit_events` unless the transaction has set `app.audit_maintenance = 'on'`, which only those two jobs do.

## Idempotency & retries

Billing webhooks retry. Admin tooling double-clicks. Any mutating endpoint that creates or transitions an Entitlement **must** accept an `Idempotency-Key` header and return the original result on a repeated key rather than creating a duplicate. See [Conventions](../api-reference/conventions/#idempotency).

## Rate limiting

Per-API-key token bucket, returning `429` with a `Retry-After` header on exhaustion:

| Scope | Limit |
|---|---|
| Default, all endpoints, per API Key or per User session | 100 requests/minute |
| `POST /v1/entitlements`, `POST /v1/users` | 20 requests/minute — tighter, since these are the endpoints scripted abuse would hit first |
| `GET /v1/users/{id}/apps/{appId}/effective-permissions`, `POST /v1/auth/app-tokens`, `POST /v1/auth/oauth/token` | 300 requests/minute — expected to be called on a hot path by downstream apps (see [Trust Model](../trust-model/)), so it's deliberately not bottlenecked at the default rate |
| `POST /v1/auth/login`, `/signup`, `/mfa/verify` | 10 requests/minute per IP address, in addition to per-account lockout |
| `POST /v1/auth/password/forgot`, `/email/verify/resend` | 5 requests/minute per IP address, and 3 per hour per email address |
| `POST /v1/auth/token/refresh` | 30 requests/minute per session |
| `GET /v1/users/{id}/export` | 5 per user per day |
| `POST /v1/organizations` | 10 per user per day |
| `POST /v1/applications`, self-service | 10 per user per day (staff with `applications.manage` aren't limited beyond the default) |
| `POST /v1/users/{id}/notifications` | 600 requests/minute per API Key, and 60 notifications per recipient User per Application per rolling hour (`429 rate_limited`, `details.scope: "recipient"`). Platform alerts don't count toward the per-recipient limit. |
| `POST /v1/users/{id}/devices/{deviceId}/test` | 10 per device per hour |

Every response carries `RateLimit-Limit`, `RateLimit-Remaining`, and `RateLimit-Reset` (seconds until the bucket refills). A `429` also carries `Retry-After`.

These are starting defaults, not a promise — tune them against real traffic once the API is live, and raise per-key limits for high-volume service integrations (billing) rather than exempting them from limiting entirely. A call made through the [MCP Server](../mcp-server/) counts against the same bucket as the REST call it translates to — there's no separate "MCP traffic" dimension, since it's the same credential hitting the same underlying endpoint.

## Security logging

Separate from the [Audit log](#audit), which records *changes to state*. The security log records *access attempts*: every login, failed login, MFA challenge, refresh, refresh-token reuse, lockout, `401`, `403`, and rate-limit hit. Each entry is structured JSON carrying `X-Request-Id`, the actor (user id or key id), the IP address, the user agent, and the outcome, and never any token, secret, or password. They go to CloudWatch Logs and are retained for 1 year. A CloudWatch alarm fires on spikes in failed logins, refresh-token reuse, or `destructive_operation_restricted`. This log is what SOC 2 access-monitoring evidence, and breach scoping under [Compliance](../compliance/), come from.

## Service-level objectives

Targets for the API itself, measured monthly at the API Gateway. They're published to customers only once there's a track record, and they aren't contractual until a business or enterprise agreement says so.

| Objective | Target |
|---|---|
| Availability (non-5xx share of requests) | 99.9% |
| `effective-permissions`, `app-tokens`, `oauth/token` latency | p95 < 150 ms, p99 < 400 ms |
| All other endpoints | p95 < 400 ms |
| Webhook dispatch (commit → first attempt) | p95 < 10 s |
| Push notification dispatch (commit → first provider attempt) | p95 < 10 s |
| JWKS availability | 99.99%. JWKS is served by the API with `Cache-Control: public, max-age=3600`, and Applications cache it for that hour, so a short API outage doesn't stop token verification. CloudFront goes in front of the whole API domain at the [Scale-out](https://github.com/CompositeCode/substratalapps.com/blob/main/plan-remaining.md#scale-out) trigger, not at Tier 0. |

## Versioning

- `/v1` now. Additive, backward-compatible changes (new optional fields, new endpoints) ship without a version bump.
- A breaking change gets a new version prefix. The old version keeps working for a minimum 6-month deprecation window, announced in the [Changelog](../changelog/) the day the replacement ships, with a `Deprecation` and `Sunset` response header (RFC 8594) added to every response the old version serves from that point on.
- Webhook payloads are versioned independently of the URL version, by a date string (`api_version`, currently `2026-10-05`) pinned on each subscription at creation. A breaking payload change ships as a new date. Existing subscriptions keep their pinned shape until their owner moves them forward with `PATCH`, and old payload versions are supported for at least 12 months. Additive payload fields don't get a new date.
- The [MCP Server](../mcp-server/)'s protocol version (negotiated per MCP's own date-based scheme during `initialize`) is likewise independent of `/v1` — a protocol-version bump there doesn't imply a `/v2` here, and vice versa.
- **Enums can grow new values without a version bump.** `EntitlementStatus`, `UserStatus`, and similar closed-looking lists in [openapi.yaml](../openapi.yaml) are allowed to gain new members as additive, non-breaking changes. Clients — and any implementation — must treat an unrecognized enum value as "handle generically / no special case," never as an error to reject the response over. This is a contract, not just a suggestion: don't write a `switch` with no default case against any enum in this spec.

## Request tracing

Every response carries an `X-Request-Id` (server-generated if the caller didn't send one in the request). Log it at every layer a request touches. This is what turns "a user says something failed around 2pm" into a specific, searchable trace — cheap to add at the start, expensive to retrofit once there's production traffic to correlate.

## Testing strategy

- **Contract tests** validate the implementation's actual responses against [openapi.yaml](../openapi.yaml) — this is what keeps the machine-readable spec from silently drifting from reality, which is the normal failure mode for hand-maintained API docs.
- **Unit tests** on the [Access Control](../access-control/) resolution logic (`effective_permissions`) specifically — it's the one piece of logic every single request depends on, and it's pure/deterministic enough to be cheap to test exhaustively (every combination of entitlement status × platform role × app role).
- **Integration tests** run against a real local Postgres (see [Deployment Architecture → Local development](https://github.com/CompositeCode/substratalapps.com/blob/main/infra/README.md)), not a mocked data layer — the Row-Level Security policies in [Multi-tenancy](#multi-tenancy) are exactly the kind of thing a mock would let silently pass while actually being broken.
- **Seed/fixture data** for local and test environments should be generated from the same request/response examples already in the [API Reference](../api-reference/) — one source of realistic data, not a second hand-maintained copy that drifts from the docs.

## Data retention

- Audit Events: retained indefinitely (compliance system of record) — see [Audit log lifecycle](#audit-log-lifecycle) immediately below for how that stays true without keeping every Audit Event in hot, directly-queryable storage forever.
- Disabled/revoked Entitlements: retained, not deleted — re-enabling or investigating a dispute depends on the history.
- [Notifications](../domain-model/notifications/#notification): 90 days, then deleted, whatever their read state. Notification delivery results: 30 days. Push devices: deleted 30 days after they become `inactive`.
- Deleted Users: soft-deleted first (status transition), with a separate, deliberate hard-delete process for right-to-erasure requests — see [Hard-delete cascade](#hard-delete-cascade) immediately below for what that process actually does, not just that it exists.

### Audit log lifecycle

"Retained indefinitely" and the audit log's **hot-storage window** are two different axes, not a contradiction. The hot window is one year for every Tenant; business and enterprise pricing may negotiate a longer one when it ships.

1. **Hot storage.** A freshly-written Audit Event lives in the primary `audit_events` table (Aurora Postgres) with its full `before`/`after` snapshot — queryable via [API Reference → Audit](../api-reference/audit/) exactly as specified elsewhere on this page.
2. **Archival, at the end of the hot window.** A scheduled job (see [Deployment Architecture → Audit log archival](https://github.com/CompositeCode/substratalapps.com/blob/main/infra/README.md)) moves every Audit Event older than the hot window out of the hot table: `action`, `timestamp`, `actor_type`/`actor_id`, `target_type`/`target_id`, and every other shape field move to cheap, indefinite cold storage; the `before`/`after` snapshot values — the one place the row could hold personal data — are dropped at this point rather than carried forward, the same redaction [Hard-delete cascade](#hard-delete-cascade) step 4 already performs for a deleted User, just applied uniformly by age instead of by erasure request.
3. **Cold storage, forever.** The shape-only archive is never deleted, satisfying "retained indefinitely" literally — what's indefinite is the record that something happened, not a guarantee that its full payload stays warm (or stays at all) past the hot window. Retrieving an archived event for a dispute or investigation is a deliberately manual, support-mediated process, not a live API path — see [Deployment Architecture](https://github.com/CompositeCode/substratalapps.com/blob/main/infra/README.md) for the actual retrieval SLA.

This is also why the hot/cold split doesn't weaken [Audit trail vs. erasure tension](../compliance/#gdpr-and-ccpa--build-for-it-now): a snapshot value is gone (not just hidden) the moment it leaves hot storage, for every Tenant, regardless of whether a User ever requests erasure.

### Hard-delete cascade

Triggered by `POST /v1/users/{id}/erasure-requests` (see [Users](../api-reference/users/#post-v1usersiderasure-requests)) and run by a scheduled job **7 days** after the request, well within GDPR Article 12(3)'s 30-day ceiling. The 7-day gap is a window to cancel a request made in error or under account takeover. Soft-delete (the `status: deleted` transition) happens immediately when the request arrives; this cascade is the follow-up. Runs in this order, each step committed before the next:

1. Delete `password_hash`, `mfa_secret`, and `mfa_pending_secret` from every [UserIdentity](../domain-model/users-and-organizations/#useridentity) row the User holds, and delete their `mfa_recovery_codes`. The credential goes, not the row: the row's `method`/`created_at` stays as a record that an identity of that kind existed. Null `external_subject_id`.
2. Delete the User's [Profile](../domain-model/profiles/) row outright, and every `sessions`, `refresh_tokens`, and `auth_tokens` row (sessions hold IP addresses and user agents). Delete every [Device](../domain-model/notifications/#device) (push tokens), [Notification](../domain-model/notifications/#notification), and notification delivery the User holds.
3. For every [AppProfile](../domain-model/profiles/#appprofile) the User holds, set `display_handle` to `null` and `custom` to `{}`. AppProfile is identity data, so all of it is treated as personal. For every [AppSettings](../domain-model/settings/#appsettings) row, remove only the keys the Application's `settings_schema` marks `x-pii: true` (see [`settings_schema` rules](../domain-model/settings/#settings_schema-rules)), and keep structural preferences such as `week_start`. The rows themselves are retained, since an Application may have a legitimate reason to know an entitlement-shaped record existed.
4. Redact `before`/`after` snapshot values (not the whole row) on every [Audit Event](../domain-model/orders-and-audit/#audit-event) where this User is `target_user_id`. That preserves `action`/`timestamp`/`actor` (the *shape* of what happened) while removing the one place a deleted field's value could otherwise still be read back.
5. Remove the User from every Organization: delete their `organization_memberships` rows (`pending` or `active`), and remove their id from every org-wide Entitlement's `member_overrides`. A plain soft-delete leaves both in place, so a restored User comes back exactly as they were. If this removes an Organization's last active `org_admin`, its longest-standing active member is promoted, so the Organization is never left unmanageable (the `last_org_admin` guard can't apply to a cascade that has to complete). On `users`: set `email` to `erased+<id>@invalid.substratal`, null `pending_email`, and keep `status: deleted`. The row must survive as the foreign-key target of the history in step 6.
6. Leave [Entitlement](../domain-model/entitlements/) rows (and their opaque `order_id`) in place, pseudonymized by the steps above having removed the identifying data they'd otherwise join against. They're retained under a legitimate-interest basis (financial/business records) distinct from, and not overridden by, the erasure right.
7. Mark the `erasure_requests` row `completed` and write `user.erased` (actor `system`).

Each step is idempotent, so a failed run resumes safely from the start.

`GET /v1/users/{id}/export` (see [API Reference → Users](../api-reference/users/)) reads every table this cascade writes to, plus one it deliberately doesn't: global [Settings](../domain-model/settings/). Export answers "what do you have on me," so it includes everything keyed to the User. Erasure answers "stop having anything that identifies you," and global Settings (`locale`, `timezone`, `theme`, `notifications`) identifies no one once the User's email, Profile, and credentials are gone, so the row stays as a pseudonymous preference record. Every other table is symmetric: if export shows it, the cascade clears it.

## Availability expectations on the trust model

Because downstream apps verify access against this API (see [Trust Model](../trust-model/)), this API's availability is a dependency of every app in the catalog, not just the hub dashboard. JWT verification keeps apps functioning through a brief hub outage; live introspection calls do not. This is a reason to lean on JWTs for routine checks and reserve introspection for genuinely sensitive actions, independent of the latency tradeoff discussed on that page.
