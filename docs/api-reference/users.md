---
layout: default
title: Users
parent: API Reference
nav_order: 3
---

# Users
{: .no_toc }

See [Domain Model → Users & Organizations](../../domain-model/users-and-organizations/) for the full field reference and lifecycle diagram. Signup, login, password, and MFA live on [Auth](../auth/).
{: .fs-6 .fw-300 }

1. TOC
{: toc }

---

## Endpoints

| Method | Path | Requires | Purpose |
|---|---|---|---|
| `GET` | `/users` | `users.list` (or an app-confined `users.list`) | List/search users. |
| `POST` | `/users` | `users.manage` | Create a user (invite, or direct creation). |
| `GET` | `/users/{id}` | self, `users.list`, or app-confined `users.list` | Fetch one user. `{id}` may be `me`. Soft-deleted users are visible to `users.manage` only. |
| `PATCH` | `/users/{id}` | self (`email` only) or `users.manage` | Update `email`; `users.manage` may also set `status`, including restoring a soft-deleted User. |
| `DELETE` | `/users/{id}` | self or `users.manage` | Soft-delete. |
| `POST` | `/users/{id}/suspend` | `users.manage` | Shortcut for `PATCH {status: "suspended"}`. |
| `POST` | `/users/{id}/invitation` | `users.manage` | Re-send the invitation email to an `invited` user. |
| `GET` | `/users/{id}/export` | self or `users.manage` | Everything this API holds about this User, as one bundle (GDPR Article 20 / CCPA right-to-know). |
| `POST` | `/users/{id}/erasure-requests` | self or `users.manage` (including for an already soft-deleted User) | Request right-to-erasure: soft-delete now, hard-delete cascade after 7 days. |
| `DELETE` | `/users/{id}/erasure-requests/current` | `users.manage` | Cancel a scheduled erasure inside its 7-day window. |

Password, MFA, and session endpoints under `/users/{id}/…` are specified on [Auth](../auth/). Device and notification endpoints (`/users/{id}/devices`, `/users/{id}/notifications`) are on [Push Notifications](../notifications/).

## The User object

```json
{
  "id": "usr_01JAG3Z9X8QS3F6K2M4N5P6R7S",
  "email": "jordan@example.com",
  "email_verified": true,
  "pending_email": null,
  "status": "active",
  "mfa_enabled": false,
  "signup_application_id": "app_timetrack",
  "test_mode": false,
  "expires_at": null,
  "created_at": "2026-01-14T18:02:11Z",
  "updated_at": "2026-09-20T14:12:00Z",
  "last_login_at": "2026-10-02T09:41:03Z"
}
```

| Field | Notes |
|---|---|
| `pending_email` | Set while a self-service email change awaits verification of the new address; `email` still holds the old one until then. |
| `mfa_enabled` | `true` if any `password` identity has confirmed TOTP. Read-only here — see [Auth → MFA](../auth/#mfa-totp). |
| `signup_application_id` | Which Application the User signed up through, if any. Read-only, informational; grants nothing. |
| `test_mode` | `true` if created by a `satk_test_` key — see [Conventions → Authentication](../conventions/#authentication). |
| `expires_at` | When a test-mode User stops working; `null` for every live User. Defaults to 24 hours after creation, at most 30 days. See [Auth → Test-mode Users → Lifetime](../auth/#lifetime). |

## `GET /users`

```
GET /users?status=active&email=jordan@example.com
```

```json
// Response — 200
{
  "data": [ { "id": "usr_01JAG3Z9X8QS3F6K2M4N5P6R7S", "email": "jordan@example.com", "status": "active", "...": "..." } ],
  "page": { "next_cursor": null, "has_more": false }
}
```

| Filter | Matches |
|---|---|
| `status` | `active`, `invited`, `suspended`, and, for `users.manage` callers only, `deleted`. Without an explicit `status=deleted`, deleted users are never listed; to everyone else they don't exist (404). |
| `email` | Exact, case-insensitive. |
| `q` | Case-insensitive prefix match on `email` or Profile `display_name`, minimum 3 characters. |
| `application_id` | Users holding any access path — personal Entitlement in any status, or membership in an Organization holding a grant — to that Application. |
| `organization_id` | Members of that Organization. |
| `role_id` | Users holding any of these Roles by explicit assignment. Repeatable (`?role_id=role_platform_support&role_id=role_platform_billing_admin`), up to 20. People who get an Application's `default_app_role` implicitly, by having access, aren't listed, since that's everyone with access; use `application_id` for them. |
| `created_after`, `created_before` | ISO 8601 bounds on `created_at`. |

**App-confined callers:** an app-scoped [API Key](../api-keys/#app-confined-permissions) holding `users.list` gets exactly the `application_id=<its own app>` result, whatever it passes — the filter is forced, not optional. Any `role_id` it sends must be one of its own Application's Roles, or the request is `403 role_scope_mismatch`.

## `POST /users`

```json
// Request — invite (the default)
{ "email": "jordan@example.com", "display_name": "Jordan Alvarez" }
```
```json
// Request — create active, no email sent
{ "email": "migrated.user@example.com", "status": "active", "send_invitation": false }
```

```json
// Response — 201
{
  "id": "usr_01JAG3Z9X8QS3F6K2M4N5P6R7S",
  "email": "jordan@example.com",
  "email_verified": false,
  "pending_email": null,
  "status": "invited",
  "mfa_enabled": false,
  "signup_application_id": null,
  "test_mode": false,
  "created_at": "2026-10-03T12:00:00Z",
  "updated_at": "2026-10-03T12:00:00Z",
  "last_login_at": null
}
```

- `status` may be `invited` (default) or `active`. An `invited` user gets an invitation email (unless `send_invitation: false`) and activates via [`POST /auth/invitations/accept`](../auth/#post-authinvitationsaccept). An `active` user created this way has no password; they sign in by using [`password/forgot`](../auth/#post-authpasswordforgot) (or SSO, once live). An admin can never set a user's password directly.
- Creating a user also creates a default [Profile](../../domain-model/profiles/) (with `display_name` if given) and [Settings](../../domain-model/settings/), and assigns the `member` platform role — one transaction (see [Non-Functional Requirements → Transaction boundaries](../../non-functional-requirements/#transaction-boundaries)).
- With a `satk_test_` key, the User is test-mode and may take `expires_at` (default 24 hours from now, at least 15 minutes from now, at most 30 days); see [Auth → Test-mode Users → Lifetime](../auth/#lifetime). A live credential sending `expires_at` gets `422 validation_failed`.
- Writes Audit Event `user.created`.

## `GET /users/{id}`

Self, `users.list`, or an app-confined key (only for users with an access path to its Application — anyone else 404s, so existence isn't leaked). Returns the User object above.

A User's Organization memberships aren't a field here — see `GET /organizations` (self-scoped) in [Organizations](../organizations/).

## `PATCH /users/{id}`

```json
// Request — self-service email change
{ "email": "jordan.alvarez@example.com" }
```
```json
// Response — 200: email unchanged, pending_email set, verification email sent to the new address
{ "id": "usr_01JAG3Z9X8QS3F6K2M4N5P6R7S", "email": "jordan@example.com", "pending_email": "jordan.alvarez@example.com", "...": "..." }
```

- **Self** may change `email` only. The change is **pending** until the new address is verified via [`POST /auth/email/verify`](../auth/#post-authemailverify). A notice also goes to the old address. Writes `user.email_changed` when it completes.
- **`users.manage`** may change `email` immediately (`email_verified` resets to `false`, a verification email is sent), and may change `status`:

| Transition | Effect | Audit action |
|---|---|---|
| `active → suspended` | Same as `POST /suspend`, below. | `user.suspended` |
| `suspended → active` | Login works again; `access.granted` fires for every Application the user still has active access to. | `user.reactivated` |
| `invited → active` | Activates without a password (see `POST` above). | `user.updated` |
| `deleted → active` | **Restore.** Undoes a soft-delete: login works again, and `access.granted` fires for every Application the user still has active access to. Rejected with `409 erasure_scheduled` while an erasure request is scheduled (cancel it first), with `409 email_taken` if a new account has taken the email meanwhile (change the email in the same request to proceed), and with `404 user_not_found` once the erasure cascade has completed. | `user.reactivated` |
| anything `→ deleted` | Rejected — use `DELETE`. | — |
| `→ invited` | Rejected. | — |

Rejected transitions return `409 invalid_status_transition`. A self-service caller sending `status` gets `403 status_change_forbidden`.

**`users.manage`** may also set a test-mode User's `expires_at`, within the same range as at creation: no earlier than 15 minutes from now, and no later than 30 days after `created_at`. Moving it later on a User that has already expired brings them back (until the 30-day purge): login works again, and `access.granted` fires for every Application they still have active access to. Moving it earlier doesn't end the account at once; to cut a test run off immediately, suspend the User. Writes `user.updated`. A self-service caller sending `expires_at` gets `403 status_change_forbidden`.

## `DELETE /users/{id}`

```json
// Response — 204
```

Soft-delete: sets `status: deleted` and `deleted_at`, revokes every session, deactivates every push [device](../notifications/#sessions-and-device-lifecycle), fires `access.revoked` (reason `user_deleted`) for every Application the user had active access to, and from then on `GET /users/{id}` 404s for everyone but `users.manage`, who can still fetch, restore, or request erasure for the account. Self, or `users.manage`. Entitlements, Roles, Profile, and Settings are retained, not removed — hard deletion is the separate erasure process below. The email address becomes reusable by a new signup immediately. Writes `user.deleted`.

## The protected superadmin

The platform owner's account is **protected**. Every deploy makes sure it exists as an active, verified `superadmin` (the `SUPERADMIN_EMAIL` deploy variable), and nothing can lock it out:

- `DELETE /users/{id}`, `POST /users/{id}/suspend`, a `PATCH` that changes `status` away from `active` or changes `email`, `POST /users/{id}/erasure-requests`, and `DELETE /users/{id}/roles/role_platform_superadmin` all return `409 user_protected`.
- The database enforces the same rule for every caller, including a direct SQL session (see [Database Schema → users](../../domain-model/database-schema/#users)). Ops can lift it for one transaction to hand ownership to another account.
- If the account is somehow missing, demoted, or unprotected, the next deploy restores it. Its password is never changed by a deploy.

Everything else works normally: it can change its own password, profile, and settings, enable MFA, and sign out.

## `POST /users/{id}/suspend`

```json
// Request
{ "reason": "Chargeback fraud investigation, ticket SUP-2231" }
```
```json
// Response — 200, the User
{
  "id": "usr_01JAG3Z9X8QS3F6K2M4N5P6R7S",
  "email": "jordan@example.com",
  "email_verified": true,
  "pending_email": null,
  "status": "suspended",
  "mfa_enabled": false,
  "signup_application_id": "app_timetrack",
  "test_mode": false,
  "created_at": "2026-01-14T18:02:11Z",
  "updated_at": "2026-10-05T12:40:00Z",
  "last_login_at": "2026-10-02T09:41:03Z"
}
```

Requires `users.manage`. Suspending an already-suspended User returns `200` with no change and no new events. `reason` is optional but recorded on the Audit Event (`user.suspended`). Revokes every session, deactivates every push [device](../notifications/#sessions-and-device-lifecycle), and fires `access.revoked` (reason `user_suspended`) for every Application the user had active access to — the user's Entitlements themselves are **not** changed, so reactivation restores exactly what was there. To cut off one specific app instead of the whole account, use [Entitlements](../entitlements/). Destructive — see [MCP Server → Tool annotations](../../mcp-server/#tool-annotations--safety).

## `POST /users/{id}/invitation`

```json
// Response — 202
```

Requires `users.manage`. Issues a fresh invitation token (invalidating the previous one) and re-sends the email. `409 invalid_status_transition` if the user isn't `invited`.

## `GET /users/{id}/export`

Self, or `users.manage`. Limited to 5 calls per user per day.

```json
// Response — 200
{
  "exported_at": "2026-10-05T12:00:00Z",
  "user": { "id": "usr_01JAG3Z9X8QS3F6K2M4N5P6R7S", "email": "jordan@example.com", "status": "active", "created_at": "2026-01-14T18:02:11Z" },
  "profile": { "display_name": "Jordan Alvarez", "contact_email": "jordan@example.com", "locale": "en-US" },
  "settings": { "theme": "dark", "timezone": "America/Denver", "notifications": { "email": true, "sms": false } },
  "identities": [
    { "method": "password", "mfa_enabled": true, "last_used_at": "2026-10-02T09:41:03Z" }
  ],
  "sessions": [
    { "id": "ses_01JAG8K4Q9R0S1T2V3V4W5X6Y8", "created_at": "2026-10-02T09:41:03Z", "ip_address": "203.0.113.24", "user_agent": "TimeTrack/2.4 (iOS 19.0)" }
  ],
  "organizations": [
    { "organization_id": "org_01JAFZ8Y7X6W5V4V3T2S1R0Q9P", "role": "member", "joined_at": "2026-09-20T14:00:00Z" }
  ],
  "roles": [
    { "role_id": "role_timetrack_admin", "application_id": "app_timetrack", "assigned_at": "2026-10-03T12:05:00Z" }
  ],
  "applications": [
    {
      "application_id": "app_timetrack",
      "entitlement": { "status": "active", "source": "purchase", "starts_at": "2026-01-14T18:05:00Z" },
      "app_profile": { "display_handle": "j.alvarez", "custom": { "department": "Engineering" } },
      "app_settings": { "overrides": { "week_start": "monday" } }
    }
  ],
  "audit_events": [
    { "action": "entitlement.granted", "application_id": "app_timetrack", "timestamp": "2026-01-14T18:05:00Z" }
  ],
  "devices": [
    { "id": "dvc_01JAH2B3C4D5E6F7G8H9J0K1M2", "application_id": "app_timetrack", "provider": "apns", "token_hint": "…9f3a", "name": "Jordan's iPhone", "status": "active", "created_at": "2026-10-17T15:05:00Z" }
  ],
  "notifications": [
    { "id": "ntf_01JAH3C4D5E6F7G8H9J0K1M2N3", "application_id": "app_timetrack", "category": "timesheet.reminder", "title": "Timesheet due tomorrow", "state": "read", "created_at": "2026-10-17T15:10:00Z" }
  ]
}
```

Every section reads from a table the [hard-delete cascade](../../non-functional-requirements/#hard-delete-cascade) clears, with one deliberate exception: `settings`. Global Settings is exported because it's keyed to the User, but erasure leaves it in place because it identifies no one once the rest is gone (see the cascade's closing note). `identities` never includes `password_hash` or `mfa_secret`, and `devices` never includes a push token; `audit_events` is this User's own trail as `target_user_id` (hot storage only — events already archived per [Audit log lifecycle](../../non-functional-requirements/#audit-log-lifecycle) are available on request through support). The response is always complete and synchronous; there is no pagination. If a real account ever makes this too large to return in one response (practically, over 10 MB), the endpoint gains an asynchronous `202` + download-link mode as an additive change.

## `POST /users/{id}/erasure-requests`

The 7-day delay before the hard-delete cascade sits well inside GDPR's 30-day ceiling, and gives support a window to cancel a request made in error or under account takeover.

```json
// Request
{ "reason": "User request via support ticket SUP-4410" }
```
```json
// Response — 202
{
  "user_id": "usr_01JAG3Z9X8QS3F6K2M4N5P6R7S",
  "status": "scheduled",
  "requested_at": "2026-10-05T12:00:00Z",
  "scheduled_for": "2026-10-12T12:00:00Z"
}
```

Self, or `users.manage`. Performs the soft-delete immediately (same effects as `DELETE` above; skipped if the User is already soft-deleted, which `users.manage` can still request erasure for), then runs the [hard-delete cascade](../../non-functional-requirements/#hard-delete-cascade) 7 days later — well inside GDPR's 30-day ceiling, with a short window to catch a request made in error or by someone who took over the account. `users.manage` can cancel inside that window with `DELETE /users/{id}/erasure-requests/current` (`204`; the user stays soft-deleted, and an admin can then restore them with `PATCH status: active`, per the transition table above). Writes `user.erasure_requested`; the cascade writes `user.erased` when it completes. Destructive.

Calling this for an already-scheduled user returns the existing request (`202`, same body) — it's idempotent.

## Errors specific to this resource

| Code | Status | When |
|---|---|---|
| `email_taken` | 409 | `email` collides with an existing user on create or update. |
| `user_not_found` | 404 | `{id}` doesn't resolve — including a soft-deleted user for any caller without `users.manage`, which 404s rather than returning a `deleted` status, to avoid leaking existence past deletion. |
| `status_change_forbidden` | 403 | A self-service caller's `PATCH` attempts to change `status`. |
| `invalid_status_transition` | 409 | A `status` change not in the table above, or an invitation re-send for a non-`invited` user. |
| `erasure_not_scheduled` | 404 | Cancelling an erasure that isn't pending. |
| `erasure_scheduled` | 409 | Restoring a soft-deleted User while their erasure request is still scheduled. |
| `user_protected` | 409 | Suspending, deleting, erasing, re-addressing, or demoting the [protected superadmin](#the-protected-superadmin). |
| `user_token_required` | 400 | `me` used with an API Key. |
| `validation_failed` | 422 | Malformed email, or an unknown `status` value. |
