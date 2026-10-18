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
| 32 | [Organization and Tenant Row-Level Security](#32-organization-and-tenant-row-level-security) | 🟡 Proposed — confirm | [NFR → Multi-tenancy](https://compositecode.github.io/substratalapps.com/non-functional-requirements/#multi-tenancy) |
| 33 | [Stripe webhook processing: inline, not SQS](#33-stripe-webhook-processing-inline-not-sqs) | 🟡 Proposed — confirm | [DEPLOYMENT.md → Webhook handling](DEPLOYMENT.md#webhook-handling) |
| 34 | [JWKS without CloudFront at Tier 0](#34-jwks-without-cloudfront-at-tier-0) | 🟡 Proposed — confirm | [DEPLOYMENT.md → Build checklist](DEPLOYMENT.md#build-checklist), [NFR → SLOs](https://compositecode.github.io/substratalapps.com/non-functional-requirements/#service-level-objectives) |
| 36 | [Test mode and seats](#36-test-mode-and-seats) | 🟡 Proposed — confirm | [Pricing → What a "user" means for an Organization](https://compositecode.github.io/substratalapps.com/pricing/#what-a-user-means-for-an-organization) |
| 37 | [Audit immutability without a second database role](#37-audit-immutability-without-a-second-database-role) | 🟡 Proposed — confirm | [NFR → Audit](https://compositecode.github.io/substratalapps.com/non-functional-requirements/#audit), [Database Schema → audit_events](https://compositecode.github.io/substratalapps.com/domain-model/database-schema/#audit_events) |
| 40 | [Push notification defaults](#40-push-notification-defaults) | 🟡 Proposed — confirm | [API Reference → Push Notifications](https://compositecode.github.io/substratalapps.com/api-reference/notifications/), [Domain Model → Notifications](https://compositecode.github.io/substratalapps.com/domain-model/notifications/) |
| 39 | [Subscriber billing API](#39-subscriber-billing-api) | 🟡 Open — not applied | [API Reference → Billing](https://compositecode.github.io/substratalapps.com/api-reference/billing/), [Tenancy](https://compositecode.github.io/substratalapps.com/domain-model/tenancy/), [Entitlements](https://compositecode.github.io/substratalapps.com/domain-model/entitlements/), [DEPLOYMENT.md → Stripe Billing](DEPLOYMENT.md#stripe-billing) |

### 31. Published prices and billing options

The prices are settled ([#38](#resolved)): Free, and Standard at $6 per user per month.

**Question:** which of these billing options ship at launch?
- annual Standard plans
- a free trial of Standard

**Current assumption:** monthly USD billing only, and no trial. Neither blocks the build: each is additive Stripe configuration. A card on file is never required for Free, because Free has no Stripe Subscription.

### 32. Organization and Tenant Row-Level Security

Surfaced by the implementation (2026-10-06).

**Question:** [NFR → Multi-tenancy](https://compositecode.github.io/substratalapps.com/non-functional-requirements/#multi-tenancy) asks for RLS policies keyed on `app.current_org_id` and `app.current_tenant_id` that fail closed. Which org and tenant does a request set when the caller legitimately spans many: a User in several Organizations, a platform admin listing every Entitlement, a job sweeping every Tenant?

**What's built:** RLS enforces **test/live isolation** (`app.current_test_mode`) on every table a test credential can write, on both database backends. Tenant and Organization scoping is enforced in queries (app-confined keys, owner checks, explicit `tenant_id`/`organization_id` filters) and covered by integration tests, not by RLS.

**Proposal:** keep that for the `shared` tier, and add per-request allowed-set policies (`app.allowed_tenant_ids`, with an explicit platform bypass) only when an `isolated`/`dedicated_region` Tenant first needs a second line of defense. **Alternative:** design and add both policies now.

### 33. Stripe webhook processing: inline, not SQS

**Question:** [DEPLOYMENT.md → Webhook handling](DEPLOYMENT.md#webhook-handling) says to record the event, return `200`, and process from SQS.

**What's built:** the handler records the event id (insert-or-skip, so duplicates return `200` at once), then processes it in the same invocation, re-fetching the Subscription from Stripe. A failure leaves `processed_at` null, and the `stripe-events` job (every 10 minutes) re-fetches and reprocesses it.

**Proposal:** keep it inline; processing is one Stripe read and one transaction, well within Stripe's timeout, and the retry job gives the same durability as a queue. **Alternative:** add the SQS hop as specified.

### 34. JWKS without CloudFront at Tier 0

**Question:** the build checklist puts CloudFront in front of `/.well-known/jwks.json`, but JWKS lives on `api.substratalapps.com`, so caching just that path means putting CloudFront in front of the whole API, which [Scale-out](DEPLOYMENT.md#scale-out) lists as a later trigger.

**What's built:** JWKS is served by the API Lambda with `Cache-Control: public, max-age=3600`, and Applications cache it as the spec already tells them to.

**Proposal:** defer CloudFront to the Scale-out trigger. **Alternative:** add a CloudFront distribution for the whole domain now, to hold the 99.99% JWKS SLO through an API outage.

### 36. Test mode and seats

**Question:** an Organization's Standard subscription is billed per covered member. Do test-mode members count?

**What's built:** test-mode Users never count as seats, and seat limits never apply to test-mode writes.

**Proposal:** confirm. Test-mode members are never counted or billed, so test traffic can't consume paid seats. Test-mode subscriptions run against Stripe test mode. **Alternative:** count them, billed through a separate test-mode price.

### 37. Audit immutability without a second database role

**Question:** [NFR → Audit](https://compositecode.github.io/substratalapps.com/non-functional-requirements/#audit) gives the archival and erasure jobs a separate database role holding `UPDATE`/`DELETE` on `audit_events`. With the Data API, a second role means a second cluster secret and credential path.

**What's built:** a trigger rejects every `UPDATE`/`DELETE` on `audit_events` unless the transaction sets `app.audit_maintenance = 'on'`, which only those two jobs do. The guarantee is enforced by the database for every caller, including a direct SQL session.

**Proposal:** confirm the trigger. **Alternative:** add the second role and secret, and give only the jobs Lambda access to it.

### 39. Subscriber billing API

Surfaced by the pricing change (2026-10-17, [#38](#resolved)).

**Question:** [Pricing](https://compositecode.github.io/substratalapps.com/pricing/) now bills the people who use Substratal's apps (individual Users, and Organizations for their members), not the Application owner's Tenant. What API changes implement it?

**What's built:** Tenant billing. `tenants.plan` (`starter`/`team`/`enterprise`), `subscription_status`, and `restricted`; the `/v1/tenants/{id}/subscription`, `/usage`, and `/billing/*` endpoints; plan caps on Applications, AppRoles, webhooks, and Starter seats (`409 plan_limit_reached`, `402 subscription_required`); and a daily per-Tenant seat sync.

**Proposal:**

1. **Data model.**
   - Add a `subscriptions` table (`sbs_` ids) keyed to a subscriber: `subscriber_type` (`user` | `organization`) and `subscriber_id`. It holds `plan` (`standard`), Stripe status and period fields, `cancel_at_period_end`, `quantity`, `last_synced_at`, and `test_mode`. Organization rows also hold `member_scope` and `member_overrides`.
   - Store `stripe_customer_id` on `users` and `organizations`, created at the first Checkout rather than at signup.
   - Add `applications.access_tier` (`free` | `standard`, default `standard`).
   - Drop `tenants.plan`, `subscription_status`, `restricted`, and the Tenant Stripe columns, along with their check constraints. The drop happens in a later release, so every migration stays expand-only.
2. **Access stays Entitlement-based.**
   - Add `free` and `subscription` to `Entitlement.source`. These rows are platform-managed: an admin can disable them, but only the platform grants or revokes them.
   - A `free` Entitlement to each free Application is granted when a User signs up.
   - An active individual subscription grants a personal `subscription` Entitlement to each standard Application, with `order_id` set to the Stripe subscription id.
   - An active Organization subscription grants one org-wide `subscription` Entitlement per standard Application, using the subscription's member selection.
   - A reconcile job backfills grants when an Application is published or its `access_tier` changes.
   - When a subscription ends, its Entitlements get `ends_at` set to the end of the paid period.
   - `allow()` doesn't change apart from dropping `seat_limited`. The existing `access.granted`/`access.revoked` webhooks tell Applications when someone's tier changes.
3. **Endpoints.** These replace the four `/v1/tenants/{id}` billing endpoints.

   | Method | Path | Requires |
   |---|---|---|
   | `GET` | `/v1/users/{id}/subscription` | self or `billing.manage` |
   | `POST` | `/v1/users/me/billing/checkout-sessions` (`plan: standard`) | self, User token |
   | `POST` | `/v1/users/me/billing/portal-sessions` | self, User token |
   | `GET` | `/v1/organizations/{id}/subscription` | `org_admin` or `billing.manage` |
   | `PATCH` | `/v1/organizations/{id}/subscription` (`member_scope`, `member_overrides`) | `org_admin` |
   | `POST` | `/v1/organizations/{id}/billing/checkout-sessions` | `org_admin`, User token |
   | `POST` | `/v1/organizations/{id}/billing/portal-sessions` | `org_admin` or `billing.manage`, User token |

   - The User object gains read-only `plan` (`free` | `standard`) and `plan_sources` (`[{"type": "user" | "organization", "id": …}]`).
   - The Application object gains `access_tier`.
   - `GET /v1/applications` gains an `access_tier` filter.
4. **Errors.**
   - Remove the Applications, AppRoles, webhook, and seat caps. `plan_limit_reached` stays reserved for business and enterprise limits.
   - Remove `402 subscription_required` and `restricted`.
   - Keep `subscription_exists`, `no_billing_account`, `billing_unavailable`, and `plan_requires_sales` (Checkout for a business or enterprise plan).
   - Tier-change requests no longer check `plan`. They stay support-run.
5. **Audit and jobs.**
   - `tenant.plan_changed` and `tenant.subscription_status_changed` are replaced by `subscription.created`, `subscription.status_changed`, and `subscription.members_changed`, with target type `subscription`.
   - The daily seat sync becomes a covered-member sync for Organization subscriptions only.
   - Audit archival uses one fixed 1-year hot window.
6. **Stripe.**
   - The catalog is one Product with one price, `standard_monthly_usd`.
   - Customers and subscriptions carry `metadata.subscriber_type` and `metadata.subscriber_id` instead of `tenant_id`.
   - `cmd/stripe-catalog` and the webhook handler are updated to match.

**Sub-questions inside the proposal:**
- **Covered twice.** A User covered by an Organization can still subscribe individually. The proposal allows it, with `plan_sources` showing both so a client can suggest cancelling, and no automatic refund. The alternative is to reject the individual Checkout with `409 already_covered`.
- **Free access.** The proposal uses `free` Entitlements granted at signup. The alternative is a rule in `allow()` that passes free Applications for any active User unless that User's Entitlement is disabled. That rule writes no rows, but it adds a second way to grant access.
- **Default tier for existing Applications.** The proposal is `standard`.

**Alternative:** keep Tenant billing and model Free and Standard as Tenant plans. That doesn't fit, because the subscriber is no longer the Application owner.

### 40. Push notification defaults

Surfaced by adding push notifications to the spec (2026-10-17).

**Question:** [Push Notifications](https://compositecode.github.io/substratalapps.com/api-reference/notifications/) is fully specified, but six choices in it are defaults that could reasonably go the other way. Confirm each. None of them blocks building, and every alternative is additive.

1. **Platform alerts push only through Applications that opt in.** A device belongs to one Application and can only be reached with that app's credentials, so a platform alert (new sign-in, password changed, …) is pushed only to devices of Applications with `receives_platform_notifications: true`. Otherwise it goes to the inbox and the existing email. **Alternative:** push it to every active device, which would buzz a User once per installed app for the same alert.
2. **No broadcast or batch send yet.** Sending is one User per call (`POST /v1/users/{id}/notifications`), within the 600-per-minute sender limit. **Alternative:** add `POST /v1/notifications` with up to 500 `user_ids`, or an Organization fan-out, when an Application needs announcements.
3. **A per-recipient limit of 60 notifications per User per Application per hour** (`429 rate_limited`, `details.scope: recipient`). **Alternative:** a different number, or none.
4. **The inbox keeps 90 days**, and delivery results 30. **Alternative:** a longer window, which only changes the purge job.
5. **Preferences are the existing `notifications.push` channel only**, globally or per Application. There are no quiet hours and no per-category mutes. **Alternative:** add `notifications.quiet_hours` and per-category switches to Settings later. Platform `security.*` alerts would then need a rule about whether quiet hours can delay them.
6. **Apple, Google, and the browser vendors' push services are sub-processors**, receiving only push payloads. That's listed on [Compliance](https://compositecode.github.io/substratalapps.com/compliance/), with guidance to keep sensitive content out of payloads. **Alternative:** none, since push can't work without them. Confirm it's acceptable for the DPA list.

**Proposal:** confirm all six as specified.

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
| 35 | Seat cap on member-scope changes. *Withdrawn: the Starter seat cap no longer exists (#38).* | — |
| 38 | Pricing: Free and Standard ($6 per user per month), paid by individual Users or by Organizations for their members. Business and Enterprise pricing comes later. Decided 2026-10-17. | [Pricing](https://compositecode.github.io/substratalapps.com/pricing/) |
