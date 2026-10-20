---
layout: default
title: Billing
parent: API Reference
nav_order: 10.5
---

# Billing
{: .no_toc }

The Free/Standard subscription: a User subscribing to Standard for themselves, or an Organization subscribing for the members it covers, processed by Stripe. See [Pricing](../../pricing/) for the tiers and rules. A third-party developer's billing of their own users never touches this API (see [Orders & Audit → Billing system of record](../../domain-model/orders-and-audit/#billing-system-of-record)).
{: .fs-6 .fw-300 }

1. TOC
{: toc }

---

## How it fits together

- **Free needs nothing.** Every User is on Free from signup and has a [platform-managed Entitlement](../../domain-model/entitlements/#platform-managed-entitlements) to each free Application.
- **Subscribing** sends the person through Stripe Checkout, using a URL from this API. A User subscribes for themselves; an org admin subscribes their Organization.
- **Payment methods, invoices, and cancellation** are handled in the Stripe Customer Portal, again through a URL from this API.
- **A Stripe Customer** is created for a User or Organization the first time they check out, not at signup.
- **Access follows the subscription.** While a subscription covers someone, the platform keeps a `source: subscription` Entitlement for them to each standard Application. When it stops covering them, those Entitlements end. Applications hear about every change through the usual [`access.granted` / `access.revoked`](../webhooks/#event-types) webhooks.
- **Business and Enterprise** plans aren't offered yet. Checkout for either returns `422 plan_requires_sales`.

Stripe tells this API about every change through its webhook. This API never takes card data, and a subscription's `status` only changes in response to Stripe events. The handler is documented in [`infra/README.md` → Stripe Billing](https://github.com/CompositeCode/substratalapps.com/blob/main/infra/README.md#stripe-billing).

## Endpoints

| Method | Path | Requires | Purpose |
|---|---|---|---|
| `GET` | `/v1/users/{id}/subscription` | self, or `billing.manage` | A User's plan, what covers it, and their own subscription. |
| `POST` | `/v1/users/me/billing/checkout-sessions` | self, User token | Start a Stripe Checkout to subscribe yourself to Standard. |
| `POST` | `/v1/users/me/billing/portal-sessions` | self, User token | Open the Stripe Customer Portal for your own subscription. |
| `GET` | `/v1/organizations/{id}/subscription` | `org_admin`, or `billing.manage` | The Organization's plan and subscription. |
| `PATCH` | `/v1/organizations/{id}/subscription` | `org_admin` | Choose which members the subscription covers. |
| `POST` | `/v1/organizations/{id}/billing/checkout-sessions` | `org_admin`, User token | Start a Stripe Checkout to subscribe the Organization to Standard. |
| `POST` | `/v1/organizations/{id}/billing/portal-sessions` | `org_admin` or `billing.manage`, User token | Open the Stripe Customer Portal for the Organization's subscription. |

Checkout and Portal act for a person, so they need a User token, not an API Key (`400 user_token_required`). An active member of the Organization who isn't an `org_admin` gets `403`; anyone else gets `404 organization_not_found`.

## The Subscription object

One row per subscriber, kept across lapses and resubscribes.

```json
{
  "id": "sbs_01JAGC3D4E5F6G7H8J9K011M2N",
  "subscriber_type": "organization",
  "user_id": null,
  "organization_id": "org_01JAGB2C3D4E5F6G7H8J9K0L1M",
  "plan": "standard",
  "status": "active",
  "current_period_start": "2026-10-01T00:00:00Z",
  "current_period_end": "2026-11-01T00:00:00Z",
  "cancel_at_period_end": false,
  "member_scope": "all_members",
  "member_overrides": null,
  "quantity": { "current": 31, "last_synced": 30, "last_synced_at": "2026-10-17T00:15:03Z" },
  "unit_amount": 600,
  "currency": "usd",
  "test_mode": false,
  "created_at": "2026-09-30T18:02:11Z",
  "updated_at": "2026-10-17T00:15:03Z"
}
```

| Field | Notes |
|---|---|
| `id` | `sbs_` prefix. |
| `subscriber_type` | `user` (an individual subscription) or `organization`. Exactly one of `user_id` and `organization_id` is set. |
| `plan` | Always `standard` today. |
| `status` | `active`, `trialing`, `past_due`, `canceled`, `unpaid`, `incomplete`, `incomplete_expired`, or `paused`. Mirrors Stripe. `active`, `trialing`, and `past_due` cover their users with Standard; the rest don't. A Checkout that hasn't completed yet is `incomplete`. New values may be added; see [Non-Functional Requirements → Versioning](../../non-functional-requirements/#versioning). |
| `member_scope`, `member_overrides` | Organization subscriptions only: `all_members`, `allowlist`, or `denylist`, with the same meaning as on [org-wide Entitlements](../../domain-model/entitlements/#org-wide-entitlements-scoping-members-in-or-out). `null` on an individual subscription. |
| `quantity.current` | Users billed, computed live. Always `1` for an individual subscription. For an Organization, its covered members: active Users with an active membership, included by `member_scope`, in the subscription's own mode. See [Pricing → What a "user" means for an Organization](../../pricing/#what-a-user-means-for-an-organization). |
| `quantity.last_synced` | What was last written to Stripe by the daily sync, which is what the next invoice charges for. |
| `unit_amount` | Cents per user per month. |

Invoices and receipts aren't mirrored here. Stripe is the system of record for those, and the Portal shows them.

## `GET /v1/users/{id}/subscription`

```json
// Response — 200, a User covered by their Organization
{
  "user_id": "usr_01JAGC3D4E5F6G7H8J9K0L1M2N",
  "plan": "standard",
  "plan_sources": [{ "type": "organization", "id": "org_01JAGB2C3D4E5F6G7H8J9K0L1M" }],
  "subscription": null
}
```

`plan` and `plan_sources` are the same read-only fields as on the [User object](../users/#the-user-object). `subscription` is the User's own individual subscription, if they've ever started one, and `null` otherwise. A User covered both individually and by an Organization lists both sources, so a client can suggest cancelling the individual one. Nothing is refunded automatically.

## `GET /v1/organizations/{id}/subscription`

```json
// Response — 200
{
  "organization_id": "org_01JAGB2C3D4E5F6G7H8J9K0L1M",
  "plan": "standard",
  "subscription": { "id": "sbs_01JAGC3D4E5F6G7H8J9K011M2N", "subscriber_type": "organization", "status": "active", "...": "..." }
}
```

`plan` is `standard` while the subscription covers, and `free` otherwise. When a subscription exists, the response carries its `ETag`.

## `PATCH /v1/organizations/{id}/subscription`

```json
// Request
{ "member_scope": "allowlist", "member_overrides": ["usr_01JAGC3D4E5F6G7H8J9K0L1M2N", "usr_01JAGD4E5F6G7H8J9K0L1M2N3P"] }
```

Changes which members the subscription covers. Only `member_scope` and `member_overrides` can be sent; anything else is `422 read_only_field`. Every id in `member_overrides` must be a current active member (`422 member_override_not_a_member`). Send `If-Match` to avoid overwriting another admin's change. Members gained or lost take effect immediately, firing `access.granted` / `access.revoked`, and the change is counted from the next daily sync. Returns `404 subscription_not_found` if the Organization has never started a subscription.

## Checkout

`POST /v1/users/me/billing/checkout-sessions` (yourself) and `POST /v1/organizations/{id}/billing/checkout-sessions` (an Organization).

```json
// Request (the Organization form also accepts member_scope and member_overrides)
{
  "plan": "standard",
  "success_url": "https://dashboard.substratalapps.com/billing?checkout=success",
  "cancel_url": "https://dashboard.substratalapps.com/billing?checkout=cancelled",
  "member_scope": "all_members"
}
```
```json
// Response — 201
{
  "url": "https://checkout.stripe.com/c/pay/cs_live_a1B2c3D4...",
  "expires_at": "2026-10-05T13:00:00Z"
}
```

- `plan` must be `standard`. `business` or `enterprise` returns `422 plan_requires_sales`.
- Allowed when there's no subscription yet, or the existing one is `incomplete`, `incomplete_expired`, `canceled`, or `unpaid`. Otherwise `409 subscription_exists`, and the subscriber should use the portal instead.
- The first Checkout creates the Subscription with `status: incomplete`, and writes `subscription.created`.
- For an Organization, `member_scope` defaults to `all_members`, and Checkout's quantity is the current covered-member count, at least 1. The individual form rejects `member_scope` and `member_overrides`.
- Requires `Idempotency-Key`. `success_url` and `cancel_url` must be `https`. Stripe adds `session_id`.
- Returning to `success_url` does **not** by itself mean the subscription started. Coverage begins when Stripe's `checkout.session.completed` / `customer.subscription.created` webhook arrives, usually within seconds. Clients should poll the `GET …/subscription` endpoint until `plan` is `standard`.
- **Test mode.** A test-mode User or Organization gets a test-mode subscription, which needs Stripe test mode. A deployment configured with a live Stripe key returns `409 test_mode_billing_unavailable`.

## Portal

`POST /v1/users/me/billing/portal-sessions` and `POST /v1/organizations/{id}/billing/portal-sessions`.

```json
// Request
{ "return_url": "https://dashboard.substratalapps.com/billing" }
```
```json
// Response — 201
{ "url": "https://billing.stripe.com/p/session/live_YWNjdF8x...", "expires_at": "2026-10-05T12:05:00Z" }
```

The Portal is configured to allow updating the payment method, viewing invoices, and cancelling at period end. Covered members are changed with `PATCH …/subscription`, not in the Portal. A subscriber who has never completed a Checkout gets `409 no_billing_account`.

## Events

Subscription changes write Audit Events with target type `subscription`: `subscription.created`, `subscription.status_changed`, and `subscription.members_changed`. There are no outbound webhook events for billing itself. What Applications need to know, who gained or lost access, arrives as [`access.granted` / `access.revoked`](../webhooks/#event-types), along with the `entitlement.*` events for the platform-managed rows.

## Errors specific to this resource

| Code | Status | When |
|---|---|---|
| `organization_not_found` | 404 | `{id}` doesn't resolve, or the caller isn't a member and lacks `billing.manage`. |
| `subscription_not_found` | 404 | `PATCH` on an Organization that has never started a subscription. |
| `plan_requires_sales` | 422 | Checkout requested for `business` or `enterprise`. |
| `subscription_exists` | 409 | Checkout requested while a subscription is in force. |
| `no_billing_account` | 409 | Portal requested by a subscriber who has never completed a Checkout. |
| `test_mode_billing_unavailable` | 409 | Test-mode Checkout on a deployment that bills in Stripe live mode. |
| `billing_unavailable` | 503 | Stripe is unreachable, or billing isn't configured. `Retry-After` is set. |
| `user_token_required` | 400 | Checkout or Portal called with an API Key, or `me` used without a User token. |
| `idempotency_key_required` | 400 | Checkout without an `Idempotency-Key`. |
| `forbidden` | 403 | Another User's subscription without `billing.manage`, or an Organization endpoint by a member who isn't an `org_admin`. |
