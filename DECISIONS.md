# Decisions

Where questions that need a product or business decision get asked. This file is a queue, not the record of what was decided: once a question is answered, the answer is written into the spec page(s) it affects, and its entry here moves to [Resolved](#resolved) as a one-line pointer to where the answer now lives.

## How to use this file

- **Asking for a decision:** add an entry under [Awaiting a decision](#awaiting-a-decision) with the next free number. State the question, what the spec currently assumes (if anything), the proposal, and the alternative. On every spec page the question affects, add an "Awaiting decision" callout linking back here:

  ```markdown
  {: .decision }
  **Proposed — confirm** ([DECISIONS.md #32](https://github.com/CompositeCode/substratalapps.com/blob/main/DECISIONS.md#32-short-title)). Why, and the alternative.
  ```

- **Answering one:** update the spec pages to match the answer, remove their callouts, add a [Changelog](https://compositecode.github.io/substratalapps.com/changelog/) entry, and move the entry here to [Resolved](#resolved) with a link to where the answer lives. If the answer overrides a proposal, change every page the proposal touched in the same commit.
- **Don't let an answer live only in a chat thread, or only in this file.** Implementers read the spec pages, not this log.

Status meanings:

- **🟡 Proposed — confirm:** the spec already contains a complete, buildable answer, written so implementation isn't blocked, but nobody has explicitly confirmed it.
- **🟡 Open — not applied:** the spec has *not* been changed; the inconsistency stays until this is decided.

---

## Awaiting a decision

| # | Question | Status | Where it lives in the spec |
|---|---|---|---|
| 31 | [Billing options: annual plans and a Standard trial](#31-published-prices-and-billing-options) | 🟡 Open | [Pricing → Stripe catalog](https://compositecode.github.io/substratalapps.com/pricing/#stripe-catalog) |

### 31. Published prices and billing options

The prices are settled ([#38](#resolved)): Free, and Standard at $6 per user per month.

**Question:** which of these billing options ship at launch?
- annual Standard plans
- a free trial of Standard

**Current assumption:** monthly USD billing only, and no trial. Neither blocks the build: each is additive Stripe configuration. A card on file is never required for Free, because Free has no Stripe Subscription.

---

## Resolved

Each answer lives in the spec page an implementer would read; this is only an index for older references by number.

| # | Decision | Where the answer lives |
|---|---|---|
| 1 | Identity provider: native auth now, per-Organization SSO architected | [Auth → Native auth and per-Organization SSO](https://compositecode.github.io/substratalapps.com/api-reference/auth/#native-auth-and-per-organization-sso) |
| 2 | Organizations pulled forward to Phase 2 | [Domain Model → Organization](https://compositecode.github.io/substratalapps.com/domain-model/users-and-organizations/#organization) |
| 3 | Applications are separately hosted, native or web | [Trust Model → Applications are separately hosted](https://compositecode.github.io/substratalapps.com/trust-model/#applications-are-separately-hosted) |
| 4 | Billing system of record: each developer's own (for Substratal's own catalog, that's Substratal; see #38) | [Orders & Audit → Billing system of record](https://compositecode.github.io/substratalapps.com/domain-model/orders-and-audit/#billing-system-of-record) |
| 5 | Settings schema on file with the hub | [Settings → Schema validation](https://compositecode.github.io/substratalapps.com/domain-model/settings/#schema-validation) |
| 6 | Revocation must be immediate | [Trust Model → How fast does revocation need to land?](https://compositecode.github.io/substratalapps.com/trust-model/#how-fast-does-revocation-need-to-land) |
| 7 | Dedicated AWS account, `us-east-1` | [DEPLOYMENT.md → AWS account & region](DEPLOYMENT.md#aws-account--region) |
| 8 | Storage: Postgres `jsonb`, typed where declared | [Database Schema → Typed fields](https://compositecode.github.io/substratalapps.com/domain-model/database-schema/#typed-fields-per-application-views-and-expression-indexes) (mechanism per #21) |
| 9 | App developer/publisher model and review lifecycle | [Applications → The review lifecycle](https://compositecode.github.io/substratalapps.com/domain-model/applications/#the-review-lifecycle) |
| 10 | Compliance scope: GDPR/CCPA now, SOC 2 posture now, HIPAA out | [Compliance & Data Protection](https://compositecode.github.io/substratalapps.com/compliance/) |
| 11 | Tenant and Organization are separate concepts | [Tenancy → Tenant vs. Organization](https://compositecode.github.io/substratalapps.com/domain-model/tenancy/#tenant-vs-organization) |
| 12 | Three tenancy tiers, support-gated tier changes | [Tenancy → Tiers](https://compositecode.github.io/substratalapps.com/domain-model/tenancy/#tiers), [How a tier change happens today](https://compositecode.github.io/substratalapps.com/domain-model/tenancy/#how-a-tier-change-happens-today) |
| 13 | Organization vs. User entitlement precedence | [Access Control → Organization vs. User precedence](https://compositecode.github.io/substratalapps.com/access-control/#organization-vs-user-precedence) |
| 14 | Agent API keys: `intended_use` + `restrict_destructive` | [API Keys → Agent keys & `restrict_destructive`](https://compositecode.github.io/substratalapps.com/api-reference/api-keys/#agent-keys--restrict_destructive) |
| 15 | Platform subscription billed through Stripe Billing | [Pricing → How the subscription is charged](https://compositecode.github.io/substratalapps.com/pricing/#how-the-subscription-is-charged) |
| 16 | App-token issuance: embedded login for first-party apps now, hosted authorization code + PKCE before any third-party app | [Auth → Getting an app token](https://compositecode.github.io/substratalapps.com/api-reference/auth/#getting-an-app-token) |
| 17 | App-scoped API keys may carry app-confined platform permissions | [API Keys → App-confined permissions](https://compositecode.github.io/substratalapps.com/api-reference/api-keys/#app-confined-permissions) |
| 18 | Concierge developer onboarding before Phase 3 | [Applications → Who can manage an Application's catalog entry](https://compositecode.github.io/substratalapps.com/domain-model/applications/#who-can-manage-an-applications-catalog-entry) |
| 19 | Derived `access.granted`/`access.revoked` webhooks; `allow()` checks user status | [Webhooks → Event types](https://compositecode.github.io/substratalapps.com/api-reference/webhooks/#event-types) |
| 20 | `order_id` is an opaque reference; billing permissions govern platform subscriptions | [Orders & Audit → Order](https://compositecode.github.io/substratalapps.com/domain-model/orders-and-audit/#order) |
| 21 | Typed settings via per-Application views and expression indexes | [Database Schema → Typed fields](https://compositecode.github.io/substratalapps.com/domain-model/database-schema/#typed-fields-per-application-views-and-expression-indexes) |
| 22 | Any active User may create an Organization | [Organizations → `POST /v1/organizations`](https://compositecode.github.io/substratalapps.com/api-reference/organizations/#post-v1organizations) |
| 23 | Seat counting, Starter cap, daily Stripe quantity sync. *Superseded by #38: seats now count an Organization's covered members, the daily sync is kept, and the Starter cap is gone.* | [Pricing → What a "user" means for an Organization](https://compositecode.github.io/substratalapps.com/pricing/#what-a-user-means-for-an-organization) |
| 24 | A lapsed subscription restricts the Tenant; it never cuts off end users. *Superseded by #38: a lapsed Standard subscription drops its users to Free.* | [Pricing → When a subscription lapses](https://compositecode.github.io/substratalapps.com/pricing/#when-a-subscription-lapses) |
| 25 | Stripe object model: Starter has no Subscription; Checkout, Portal, sales-led Enterprise. *Updated by #38: Free has no Subscription, and there's one Standard price.* | [Pricing → Stripe catalog](https://compositecode.github.io/substratalapps.com/pricing/#stripe-catalog) |
| 26 | 7-day erasure grace period | [Users → `POST /v1/users/{id}/erasure-requests`](https://compositecode.github.io/substratalapps.com/api-reference/users/#post-v1usersiderasure-requests) |
| 27 | Supported regions for `dedicated_region` | [Tenancy → tier-change requests](https://compositecode.github.io/substratalapps.com/api-reference/tenancy/#post-v1tenantsidtier-change-requests) |
| 28 | Amazon SES, Substratal-branded transactional email | [Auth → Email verification](https://compositecode.github.io/substratalapps.com/api-reference/auth/#email-verification) |
| 29 | Per-user, per-Application paths use `/apps/{appId}` | [Roles & Permissions → effective-permissions](https://compositecode.github.io/substratalapps.com/api-reference/roles-and-permissions/#get-v1usersidappsappideffective-permissions) |
| 30 | Test-mode data isolation | [API Keys → Test vs. live](https://compositecode.github.io/substratalapps.com/api-reference/api-keys/#test-vs-live) |
| 32 | Organization and Tenant scoping in the data-access layer; RLS enforces test/live, allowed-set policies arrive with the first isolated Tenant | [NFR → Multi-tenancy](https://compositecode.github.io/substratalapps.com/non-functional-requirements/#multi-tenancy) |
| 33 | Stripe webhook events processed inline, with a retry job instead of SQS | [DEPLOYMENT.md → Webhook handling](DEPLOYMENT.md#webhook-handling) |
| 34 | JWKS served by the API with a 1-hour cache; CloudFront waits for Scale-out | [NFR → Service-level objectives](https://compositecode.github.io/substratalapps.com/non-functional-requirements/#service-level-objectives), [DEPLOYMENT.md → Build checklist](DEPLOYMENT.md#build-checklist) |
| 35 | Seat cap on member-scope changes. *Withdrawn: the Starter seat cap no longer exists (#38).* | — |
| 36 | Test-mode members never count toward a live subscription; test-mode subscriptions run in Stripe test mode | [Pricing → What a "user" means for an Organization](https://compositecode.github.io/substratalapps.com/pricing/#what-a-user-means-for-an-organization), [Billing → Checkout](https://compositecode.github.io/substratalapps.com/api-reference/billing/#checkout) |
| 37 | Audit immutability enforced by a trigger, not a second database role | [NFR → Audit](https://compositecode.github.io/substratalapps.com/non-functional-requirements/#audit), [Database Schema → audit_events](https://compositecode.github.io/substratalapps.com/domain-model/database-schema/#audit_events) |
| 38 | Pricing: Free and Standard ($6 per user per month), paid by individual Users or by Organizations for their members. Business and Enterprise pricing comes later. Decided 2026-10-17. | [Pricing](https://compositecode.github.io/substratalapps.com/pricing/) |
| 39 | Subscriber billing API: a `subscriptions` table per User or Organization, `access_tier` on Applications, platform-managed `free`/`subscription` Entitlements, User and Organization billing endpoints, no developer plan caps. Double coverage is allowed and visible in `plan_sources`; Free is granted as Entitlements at signup; existing Applications default to `standard`. Applied 2026-10-18. | [API Reference → Billing](https://compositecode.github.io/substratalapps.com/api-reference/billing/), [Entitlements → Platform-managed](https://compositecode.github.io/substratalapps.com/domain-model/entitlements/#platform-managed-entitlements), [Database Schema → subscriptions](https://compositecode.github.io/substratalapps.com/domain-model/database-schema/#subscriptions) |
| 40 | Push notification defaults: opt-in platform alert pushes, no broadcast send, 60 per recipient per hour, 90-day inbox, one push preference, push providers as sub-processors | [API Reference → Push Notifications](https://compositecode.github.io/substratalapps.com/api-reference/notifications/) |
