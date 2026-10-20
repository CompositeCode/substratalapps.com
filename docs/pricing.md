---
layout: default
title: Pricing
nav_order: 12.5
---

# Pricing
{: .no_toc }

Two subscription tiers, **Free** and **Standard**. The customer is the person using Substratal's apps and services: an individual who subscribes for themselves, or an Organization that subscribes for its members.
{: .fs-6 .fw-300 }

1. TOC
{: toc }

---

## The tiers

For most people, Free or Standard is all they'll ever need.

| | **Free** | **Standard** |
|---|---|---|
| Price | **$0** | **$6 per user per month** |
| What it includes | Access to the Applications, features, and services Substratal designates as free. | Access to download and use every Application and service Substratal Apps offers, including everything in Free. |
| Who can subscribe | Every User, automatically, with no card required. | A User for themselves, or an [Organization](../domain-model/users-and-organizations/#organization) for its members. |
| Billing | None. | Monthly, USD, tax-exclusive, through [Stripe Billing](#how-the-subscription-is-charged). |

Free and Standard are the only tiers available today.

### Business and Enterprise

Beyond Standard there will be business and enterprise pricing, tailored to an Organization's specific needs, [tenancy](../domain-model/tenancy/), and the geographic location of its data. It isn't offered yet and has no published price. The infrastructure it will be built on is already specified: the [`isolated` and `dedicated_region` Tenant tiers](../domain-model/tenancy/#tiers) and the support-run [tier-change process](../domain-model/tenancy/#how-a-tier-change-happens-today). Until business pricing ships, everything runs on the `shared` tier.

## Who pays

Substratal builds every Application in the catalog today (see [Home](../#what-substratal-apps-actually-is)), so Substratal bills the people who use those Applications directly, through this subscription. Nobody pays per Application. A Standard subscription covers the whole catalog.

There are two ways to be covered by Standard:

- **Individual subscription.** A User subscribes for themselves: one user, $6 a month.
- **Organization subscription.** An Organization subscribes for its members and pays $6 a month for each covered member (see [What a "user" means for an Organization](#what-a-user-means-for-an-organization)). An org admin chooses which members are covered, with the same `all_members` / `allowlist` / `denylist` choice that [org-wide Entitlements](../domain-model/entitlements/#org-wide-entitlements-scoping-members-in-or-out) already use.

A User covered either way has Standard access. Someone covered by an Organization can still subscribe individually. Both subscriptions are billed, and the User's `plan_sources` lists both so a client can suggest cancelling one; nothing is refunded automatically. When a third-party marketplace ships, its Applications will carry their own developers' pricing and billing. This subscription covers Substratal's own catalog only (see [Orders & Audit → Billing system of record](../domain-model/orders-and-audit/#billing-system-of-record)).

## What a "user" means for an Organization

An Organization is billed for each **covered member**: a distinct, non-test-mode User who is an active member of the Organization and is included by its subscription's member selection. Suspended and deleted Users don't count, and neither do pending invitations. A User who belongs to several Organizations that each subscribe is counted by each of them.

The count is **provisioned, not activity-based**. It counts who is covered, not who signed in this month, so an Organization can predict and budget against it.

The count syncs to Stripe once a day with no mid-period proration. Each invoice charges for the count on the day before it's issued. Real-time sync with prorations was considered and not chosen: it's more accurate, but noisier for customers and means more Stripe API calls.

A live subscription never counts test-mode Users, so integration tests can't consume paid seats. A test-mode subscription runs in Stripe test mode and counts only test-mode members whose [`expires_at`](../api-reference/auth/#lifetime) hasn't passed. Test-mode Users last 24 hours by default and never more than 30 days, so test mode can't stand in for a free production tier.

## How access follows the tier

Every [Application](../domain-model/applications/) is designated either **free** or **standard**.

- A **free** Application is available to every active User.
- A **standard** Application is available to every User covered by Standard, through their own subscription or an Organization's.

Access is still enforced through [Entitlements](../domain-model/entitlements/) and the [access-control algorithm](../access-control/#the-algorithm), exactly as for any other grant. The platform keeps them in step with each Application's tier and each subscription (see [Platform-managed Entitlements](../domain-model/entitlements/#platform-managed-entitlements)):

- Every User gets a `free` Entitlement to each free Application at signup.
- A subscription that covers someone gives them a `subscription` Entitlement to each standard Application: a personal one for an individual subscription, or one org-wide grant per Application for an Organization's.
- When an Application changes tier, or a new one is published, the platform grants or ends these Entitlements to match. An hourly job repairs anything that drifted.

An admin can still disable, and later re-enable, one person's access to one Application, whatever their tier. Applications hear about every change through the usual [`access.granted` / `access.revoked`](../api-reference/webhooks/#event-types) webhooks.

## When a subscription lapses

| Situation | What happens |
|---|---|
| Payment fails (`past_due`) | Nothing changes. Stripe retries (Smart Retries, about 3 weeks) and emails the customer. |
| Subscription ends (`canceled`, `unpaid`, `incomplete_expired`, `paused`) | Standard access ends at the end of the paid period. The User, or every member the Organization covered, drops to Free. |
| A member leaves an Organization, or the org admin stops covering them | They lose Standard access through that Organization at once. They're no longer counted from the next daily sync. |
| Voluntary cancellation | Cancel at period end in the Stripe Customer Portal. Standard access continues until then. |

Dropping to Free never deletes anything. Profiles, settings, and per-user data in standard Applications are kept, and come back when the User is covered by Standard again. Only access to standard Applications stops.

## Stripe catalog

Free is a User with no Stripe Subscription. Upgrades go through Stripe Checkout, and payment methods, invoices, and cancellation through the Customer Portal. Billing is monthly, USD only, and tax-exclusive with Stripe Tax.

The Products and Prices configured in Stripe. Lookup keys are what the code references, never Stripe price ids, so test and live mode can share code.

| Product | Price lookup key | Amount | Shape |
|---|---|---|---|
| Substratal Standard | `standard_monthly_usd` | $6.00 per user | Licensed (not metered), `billing_scheme: per_unit`, monthly. `quantity: 1` for an individual subscription. For an Organization, `quantity` is the covered-member count from the daily sync, at least 1. |

- **Free has no Stripe Subscription**, and a User or Organization has no Stripe Customer until its first Checkout. That avoids $0 subscriptions that would need cancelling and replacing on upgrade.
- **Monthly, USD, tax-exclusive** (Stripe Tax enabled). There are no annual plans and no Standard trial: Free is the way to try Substratal's apps, and it never needs a card. Either can be added later as Stripe configuration (a second price, or `subscription_data.trial_period_days` on Checkout) without an API change.
- **Organization quantity** is written to the subscription item by a daily job (00:15 UTC) with `proration_behavior: none`.
- Business and Enterprise prices will be created per customer when those tiers ship.

## Why two tiers

- **Free is free, not just cheap.** The first horizon in [Growth trajectory](https://github.com/CompositeCode/substratalapps.com/blob/main/DEPLOYMENT.md#growth-trajectory) is dozens of users, and the shared-tier cost of serving free Applications is near zero against the infrastructure floor that's paid regardless. Charging for it would tax exactly the adoption the product needs most right now.
- **One paid price, for the whole catalog.** A single per-user price is easy to understand and budget for, and works the same for one person or a whole Organization. Per-Application pricing would make every new Application a new purchasing decision.
- **Infrastructure choice waits for business pricing.** `isolated` and `dedicated_region` tenancy carry a real, named infrastructure cost, driven by a customer's own compliance or data-residency requirement. That's what business and enterprise pricing is for, so neither is offered on Free or Standard.

## How the subscription is charged

The subscription is processed through **Stripe Billing**. Card data never touches this API, and Stripe is PCI-DSS Level 1, which keeps PCI scope minimal (see [Compliance](../compliance/#what-this-doesnt-cover)). Stripe is one of the platform's two sub-processors, alongside AWS (see [Compliance → GDPR (and CCPA)](../compliance/#gdpr-and-ccpa--build-for-it-now)).

The implementation (object mapping, webhook event handling, the daily quantity sync) is specified in [root `DEPLOYMENT.md` → Stripe Billing](https://github.com/CompositeCode/substratalapps.com/blob/main/DEPLOYMENT.md#stripe-billing). It's a build and operations concern, not part of the API Applications call.

## Keeping these numbers honest

**Stripe's official MCP server** is the direct way to check that the Products and Prices configured in Stripe still match this page. The two are kept in sync by hand, the same relationship [Conventions → Machine-readable](../api-reference/conventions/#machine-readable) describes between this site's prose and `openapi.yaml`. When business pricing is designed, the **AWS Pricing** and **AWS Billing and Cost Management** MCP servers are the way to ground its tenancy surcharges in live infrastructure cost rather than estimates.
