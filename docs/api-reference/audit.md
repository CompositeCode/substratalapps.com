---
layout: default
title: Audit
parent: API Reference
nav_order: 11
---

# Audit
{: .no_toc }

See [Domain Model → Audit Event](../../domain-model/orders-and-audit/#audit-event) for the full field reference and the authoritative action catalog.
{: .fs-6 .fw-300 }

1. TOC
{: toc }

---

## Endpoints

| Method | Path | Requires | Purpose |
|---|---|---|---|
| `GET` | `/audit-events` | `audit.view` (platform or app-confined); or self via `target_user_id=me`; or a Tenant owner via `tenant_id` | Query the audit log. |
| `GET` | `/audit-events/{id}` | same as above, for that event | Fetch one event. |
| `POST` | `/audit-exports` | same as `GET /audit-events`, for the filters sent | Export matching events to a file. |
| `GET` | `/audit-exports/{id}` | the export's requester, or `audit.view` | Check an export, and get its download link. |

There's deliberately no write endpoint for Audit Events. They're produced only as a side effect of other writes, never created directly, and never edited or deleted through the API. An export is a read: it copies events into a file and changes none of them.

## `GET /audit-events`

```
GET /audit-events?target_user_id=usr_01JAG3Z9X8QS3F6K2M4N5P6R7S&since=2026-09-01T00:00:00Z
```

```json
// Response — 200
{
  "data": [
    {
      "id": "evt_01JAG7X3P8QY110M9N807P6Q5R",
      "action": "entitlement.disabled",
      "actor": { "type": "user", "id": "usr_01JAG9STAFF000000000000000" },
      "target": { "type": "entitlement", "id": "ent_01JAG9F4Q1W2E3R4T5Y6V7J809" },
      "target_user_id": "usr_01JAG3Z9X8QS3F6K2M4N5P6R7S",
      "application_id": "app_invoicer",
      "organization_id": null,
      "tenant_id": "tnt_01JAG1SYSTEM00000000000000",
      "before": { "status": "active" },
      "after": { "status": "disabled", "disabled_reason": "billing_dispute" },
      "request_id": "req_7c1e9a2f4b",
      "timestamp": "2026-09-30T16:22:41Z"
    }
  ],
  "page": { "next_cursor": null, "has_more": false }
}
```

| Filter | Notes |
|---|---|
| `target_user_id` | The User whose access or data changed. `me` is allowed. |
| `target_type`, `target_id` | For example `target_type=api_key&target_id=key_…`. |
| `actor_id` | A `usr_…`, a `key_…`, or `system`. |
| `actor_type` | `user`, `api_key`, or `system`. |
| `action` | One action. Repeat the parameter for several (`?action=role.assigned&action=role.removed`). |
| `application_id`, `organization_id`, `tenant_id` | Scope filters. |
| `since`, `until` | ISO 8601, inclusive/exclusive. |

**Who sees what:**

| Caller | Sees |
|---|---|
| Platform `audit.view` | Everything. |
| App-scoped key with `audit.view` | Only events whose `application_id` is its own Application. The filter is forced. |
| Any User, with `target_user_id=me` | Their own trail. Seeing the history of changes made *to* your own account is a self-service right, not an admin privilege. Calling without that filter and without `audit.view` returns `403`. |
| A Tenant owner, with `tenant_id=<their tenant>` | Events for their own Applications. That's the developer's view of what happened in their apps, including what support did. |

**Hot window only.** The API returns only events still in hot storage, which is one year for every Tenant. Older events are archived in a shape-only form and are retrievable through support, not through this endpoint. A `since` earlier than the window is allowed and simply returns nothing older. The response header `Audit-Hot-Window-Start` gives the earliest timestamp available for the caller's scope. See [Non-Functional Requirements → Audit log lifecycle](../../non-functional-requirements/#audit-log-lifecycle).

## `GET /audit-events/{id}`

Returns one event, same shape. `404 audit_event_not_found` if it doesn't exist, isn't visible to the caller, or has been archived.

## Exporting

For compliance evidence (a SOC 2 audit, a customer's request for their records) or offline analysis, export the log to a file instead of paging through it.

```json
// POST /audit-exports   (Idempotency-Key required)
{ "format": "csv", "tenant_id": "tnt_01JAG1SYSTEM00000000000000", "since": "2026-01-01T00:00:00Z" }
```
```json
// Response — 202
{
  "id": "aex_01JAH2K4Q9R0S1T2U3V4W5X6Y7",
  "status": "pending",
  "format": "csv",
  "filters": { "tenant_id": "tnt_01JAG1SYSTEM00000000000000", "since": "2026-01-01T00:00:00Z" },
  "row_count": null,
  "download_url": null,
  "download_url_expires_at": null,
  "error": null,
  "created_at": "2026-10-21T18:00:00Z",
  "completed_at": null,
  "expires_at": "2026-10-28T18:00:00Z",
  "test_mode": false
}
```

- **Filters and scope** are exactly `GET /audit-events`'s, as body fields: `action` is an array (up to 100), `target_user_id` may be `me`, and the caller is confined to the same scope as on the list. An app-confined key exports only its own Application's events, a User without `audit.view` only their own trail (`target_user_id: "me"`) or a Tenant they own. Hot window only, like the list.
- **`format`** is `ndjson` (default), one Audit Event object per line, or `csv`, with the columns `id, timestamp, action, actor_type, actor_id, target_type, target_id, target_user_id, application_id, organization_id, tenant_id, request_id, before, after` (`before`/`after` as JSON text). Rows are oldest first.
- **Asynchronous.** The request returns `202` at once, and a job writes the file, usually within a minute. Poll `GET /audit-exports/{id}` until `status` is `ready` (or `failed`, with `error` saying why: `too_many_rows` over 1,000,000 events, so narrow the filters).
- **Downloading.** Once `ready`, `download_url` is a signed link valid for 15 minutes (`download_url_expires_at`). Fetch the export again for a fresh one. The file and the record are deleted 7 days after the request (`expires_at`).
- **Who can see it:** whoever requested it, and `audit.view`. Anyone else gets `404 audit_export_not_found`.
- **It's audited.** When the file is ready, `audit.exported` is written with the requester as actor, the filters, the format, and the row count, in the export's own scope, so it appears in the same trail it exported from.
- Limited to 20 per caller per day.

## Retention

Audit Events are never deleted or edited through the API, including after the record they describe is hard-deleted. For example, a user's right-to-erasure request removes their Profile/Settings, but the audit trail of *that deletion itself* persists, with snapshot values redacted. See [Non-Functional Requirements → Data retention](../../non-functional-requirements/#data-retention).

## Errors specific to this resource

| Code | Status | When |
|---|---|---|
| `audit_event_not_found` | 404 | See above. |
| `audit_export_not_found` | 404 | The export doesn't exist, has expired, or wasn't requested by the caller (and they lack `audit.view`). |
| `forbidden` | 403 | No `audit.view`, and the query isn't scoped to `me` or an owned Tenant. |
