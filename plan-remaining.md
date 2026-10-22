# Plan: what's left to build

Everything specified for the Platform API that isn't built yet, in one place. What *is* built is described by the [API specification](https://compositecode.github.io/substratalapps.com/) (the system of record), [README.md](README.md) (code, CI/CD, local development), and [infra/README.md](infra/README.md) (deployment architecture as built). One-time setup that has to be done by hand, rather than built, is in the [`[bespoke]` issues](https://github.com/CompositeCode/substratalapps.com/issues?q=is%3Aissue+%22%5Bbespoke%5D%22+sort%3Acreated-asc).

This replaces `PLAN.md`, `DEPLOYMENT.md`, and `ORIGINAL-SPEC-DRAFT.md`, which were retired on 2026-10-20 once everything else in them was built and documented where it belongs. They remain in git history.

1. [Status](#status)
2. [Verify the build in production](#verify-the-build-in-production)
3. [First-party web pages](#first-party-web-pages)
4. [Phase 3: marketplace and scale features](#phase-3-marketplace-and-scale-features)
5. [Dedicated tenancy infrastructure](#dedicated-tenancy-infrastructure)
6. [Growth trajectory](#growth-trajectory)
7. [Scale-out](#scale-out)
8. [Scaling the MCP server](#scaling-the-mcp-server)
9. [Rules for every change](#rules-for-every-change)

---

## Status

As of 2026-10-22, every operation in `openapi.yaml` is implemented, including subscriber billing, push notifications, the hosted PKCE endpoints, and test-mode User expiry. SSO returns `501 not_implemented` until the broker integration, as specified. Production infrastructure exists in AWS (`substratal-prod`, `us-east-1`); the first deploy is finishing (see the [`[bespoke]` issues](https://github.com/CompositeCode/substratalapps.com/issues?q=is%3Aissue+%22%5Bbespoke%5D%22+sort%3Acreated-asc)). No product decisions are open: each is written into the spec page it governs and recorded in the [Changelog](https://compositecode.github.io/substratalapps.com/changelog/).

## Verify the build in production

The phase exit criteria have only been checked against the local devserver. Once production is up, check them there:

- **Phase 0:** `GET /health` returns `200` through the real Lambda and API Gateway path. (Done: it returns `200` in production, verified 2026-10-22.)
- **Phase 1:** the [Quickstart](https://compositecode.github.io/substratalapps.com/quickstart/) runs end to end against production: create a user, grant an app, check access, turn it off. Run `scripts/quickstart.sh` with `SUBSTRATAL_API=https://api.substratalapps.com` and a test-mode key.
- **Phase 2:** a third-party-shaped app signs a user in through the embedded flow, receives `access.revoked` and kills the session, calls introspection, has its `settings_schema` validated and typed on write, and receives a push sent through its app key. A test-mode User and a test-mode Organization subscribe to Standard through Checkout (needs a Stripe test-mode key in a non-production environment, or temporarily in production), gain the standard Applications, and lapse back to Free.

## First-party web pages

The API is complete, but the user-facing pages its emails and redirects point to don't exist yet. Both are clients of the API, not part of it:

- **Dashboard** (`dashboard.substratalapps.com`, `DASHBOARD_URL`): **being built now, outside this repository, and tracked there.** It's part of the management app, which has web, macOS, Windows, iOS, Android, and Linux versions. The email-verification, password-reset, invitation-acceptance, and email-change links land here. It's also where a User subscribes to Standard and manages their account, devices, and inbox, and where org admins manage members and the Organization's subscription. Its web version is the first-party browser origin in the API's CORS allow-list. The verify-email and reset-password pages are the minimum needed before email-based signup works for real users. Nothing for it is built in this repository; this API only needs to keep serving what those pages call.
- **Hosted sign-in** (`auth.substratalapps.com/authorize`, `AUTHORIZE_URL`): the page behind the hosted authorization-code + PKCE flow ([Auth → Getting an app token](https://compositecode.github.io/substratalapps.com/api-reference/auth/#getting-an-app-token)). The endpoints are built; only the page is missing. Needed before any third-party Application, and by any first-party app that prefers it to embedded login.

## Phase 3: marketplace and scale features

Matches [Roadmap → Phase 3](https://compositecode.github.io/substratalapps.com/roadmap/#phase-3) — explicitly **not** built speculatively ahead of the triggers named in [Scale-out](#scale-out) and [Applications → The review lifecycle](https://compositecode.github.io/substratalapps.com/domain-model/applications/#the-review-lifecycle):

- **SSO broker integration** (`sso_connections` becoming real, an actual WorkOS-or-equivalent wiring behind `POST /auth/sso/{provider}/callback`) — the schema and the route exist from Phase 1 per [Auth → Native auth and per-Organization SSO](https://compositecode.github.io/substratalapps.com/api-reference/auth/#native-auth-and-per-organization-sso), deliberately built in advance precisely so this phase is additive wiring, not a migration. Triggered by a real Organization actually asking for it, not a phase-number default.
- Richer audit/compliance views (filtering, export) — driven by actual [Compliance](https://compositecode.github.io/substratalapps.com/compliance/) needs (a SOC 2 audit, a customer's data-export request), not built ahead of either.
- **The hosted sign-in page** above, if no first-party app has needed it sooner. Self-service Application registration is built, so this is now what stands between a self-registered Application and going live.

## Dedicated tenancy infrastructure

Only the `shared` tier is built: the schema, `tenants` table, tier-change requests, and `migrating` status all exist, but no `isolated` or `dedicated_region` cluster can be provisioned yet, and requests aren't routed per Tenant. Build this when the first customer actually needs it, alongside the business and enterprise pricing that charges for it ([Pricing → Business and Enterprise](https://compositecode.github.io/substratalapps.com/pricing/#business-and-enterprise)). The tier design is in [infra/README.md → Tenancy tiers](infra/README.md#tenancy-tiers--where-they-run).

To build:

- Terraform for an additional Aurora Serverless v2 cluster and secret per dedicated Tenant (and, for `dedicated_region`, optionally a regional API Gateway and Lambda deployment).
- Per-Tenant request routing: a lookup from `tenant_id` to cluster ARN and secret, read on each request by the data-access layer.
- Per-request allowed-set RLS policies (`app.allowed_tenant_ids`, with an explicit platform bypass) as a second line of defense inside a dedicated cluster ([NFR → Multi-tenancy](https://compositecode.github.io/substratalapps.com/non-functional-requirements/#multi-tenancy)).
- Tooling for the runbook below.

### Migration runbook

A tier change (`shared → isolated`, or either `→ dedicated_region`) is support-run, during a brief scheduled maintenance window:

1. Provision the destination cluster (new Aurora Serverless v2 cluster; for `dedicated_region`, in the target region) via the same [Build checklist](README.md#one-time-setup-phase-0) steps used for Tier 0 itself.
2. Set the source Tenant's `status` to `migrating` — the API layer treats this as read-only for that Tenant's rows specifically, not a global outage.
3. Snapshot and restore (RDS snapshot copy, or `pg_dump`/`pg_restore` at this data volume) every row carrying that `tenant_id` into the destination cluster.
4. Update the `tenants` lookup row's cluster endpoint, flip `tier`/`region`/`status` back to `active`, decommission the old rows once the cutover is confirmed.
5. Record an Audit Event (`tenant.tier_changed`).

Because Entitlement, AppProfile, and AppSettings rows all denormalize `tenant_id`, step 3 is a `WHERE tenant_id = ...` export per table — no cross-table join needed to find everything that has to move.

## Growth trajectory

The stated plan: dozens of users and a handful of Applications in the first 6–12 months, roughly 10x from there over the following 6–12 months, and another 10x (100x from today) beyond that.

| Horizon | Rough scale | What this means for the architecture |
|---|---|---|
| 0–6/12 months | Dozens of users, a few Applications | **Tier 0 as specified, unchanged.** The [floor cost table](infra/README.md#illustrative-tier-0-floor-cost) is sized for exactly this. |
| 6–12 months out (~10x) | Low hundreds of users | Watch rate-limit defaults and the Aurora ACU ceiling (not just the floor) — this is when the *first* [Scale-out](#scale-out) triggers are plausible, not guaranteed. Check real metrics rather than upgrading preemptively. |
| 12+ months (~100x) | Low thousands of users | The horizon [Scale-out](#scale-out)'s read-replica, Provisioned Concurrency, and RDS Proxy rows are realistically aimed at. Also the point multi-tenancy query performance (the test/live Row-Level Security policy plus the `tenant_id` filters) is worth profiling specifically, not just trusting by design. |

## Scale-out

Nothing below is pre-built into Tier 0 — each is a deliberate upgrade, triggered by a specific, named condition, not a default growth path.

| Trigger | Change |
|---|---|
| Sustained read load on `effective-permissions` / catalog reads | Add an Aurora read replica; consider API Gateway response caching for read-heavy, low-churn paths. |
| Lambda cold-start latency becomes user-visible on hot paths | Provisioned Concurrency on the `effective-permissions` and entitlement-toggle handlers specifically — not a blanket setting. |
| Sustained high-throughput DB access, where Data API's per-call overhead starts to matter | Move those paths from Data API to direct VPC connections through **RDS Proxy** — this is the point where a NAT Gateway (or a cheaper NAT instance) becomes worth its cost, because throughput now justifies it rather than convenience. |
| Geographically distributed callers, latency-sensitive | CloudFront in front of API Gateway. |
| Hub availability becomes a real incident risk for every downstream app, not a theoretical one | Multi-region active-passive — deliberately not a Tier 0/1 concern; premature multi-region is itself a cost and complexity risk. |
| Webhook delivery volume grows enough to need isolation from the main API's blast radius | Split the worker Lambda's concurrency/alerting from the API handlers' (it already runs as a separate function). |
| Push volume grows enough that APNs connection reuse, or isolation from webhook delivery, matters | Give push delivery its own worker function with reserved concurrency, so long-lived HTTP/2 connections to APNs survive between batches and a burst of pushes can't delay webhooks. |
| More than one engineer shipping concurrently | A second AWS account (via AWS Organizations) for staging, separate from production — Tier 0 runs on a single account with a `dev`/`prod` naming convention, which is enough until this trigger. |
| A specific customer requires dedicated or regional infrastructure | Run the [Tenancy tiers](infra/README.md#tenancy-tiers--where-they-run) migration for that one customer's Tenant — the one scale-out trigger in this table that's customer-specific rather than aggregate-volume-driven. |
| Agent/MCP tool-call traffic needs server-push, resumable streams, or dedicated concurrency | See [MCP server → Scaling the MCP server](#scaling-the-mcp-server) — its own trigger table. |
| Stripe billing volume or complexity outgrows a single webhook Lambda (e.g. real invoice-dunning logic, usage-based billing beyond simple per-seat metering) | Revisit as its own build, not folded into this table speculatively — nothing here is blocked on it. |

## Scaling the MCP server

| Trigger | Change |
|---|---|
| Sustained high-frequency tool-call traffic on hot tools | Provisioned Concurrency on the MCP Lambda specifically — same pattern and trigger as the [Scale-out](#scale-out) table's cold-start row. |
| A real need for server-initiated push — mid-call progress notifications, or resumable streams | Move `/mcp` from a plain Lambda-behind-API-Gateway integration to a **Lambda Function URL with streaming responses enabled** (`InvokeMode: RESPONSE_STREAM`) — API Gateway's HTTP API integration buffers the full Lambda response, so it can't carry a chunked `text/event-stream`; a Function URL can. |
| Session/stream-resumption state that no longer fits in a short-lived signed token | A small **DynamoDB** table, keyed by session ID, TTL'd like `idempotency_keys` — ephemeral, high-churn, single-key lookup: the textbook case Aurora is the wrong tool for and DynamoDB is the right one. |
| More than a handful of concurrent agent integrations with genuinely different trust levels | Per-integration API Keys scoped as narrowly as each integration actually needs — an operational practice, not an infrastructure change. |

## Rules for every change

These applied from the first line of code and still apply to everything above:

- **Access Control's permission checks on every endpoint**, from the first commit of it. Retrofitting authorization onto endpoints built "open" is how it gets missed somewhere.
- **Conventions' error shape, pagination, idempotency, and `me` self-addressing** come from the shared helpers. New endpoints get them for free and don't reimplement them.
- **Contract tests against `openapi.yaml`** for every endpoint, the day it's built.
- **The spec pages and the [Changelog](https://compositecode.github.io/substratalapps.com/changelog/) stay current.** An implementation detail that contradicts or settles something in the spec gets written into the page it governs, with a Changelog entry, immediately.
- **Migrations are expand-only per release**, and numbered in the order they'll reach production ([README → Migrations](README.md#migrations-expand-then-contract)).
