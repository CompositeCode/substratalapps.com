---
layout: default
title: OpenAPI Specification
nav_order: 6.5
has_children: true
permalink: /openapi/
---

# OpenAPI Specification
{: .no_toc }

The complete Platform API contract as an [OpenAPI 3.1](https://spec.openapis.org/oas/v3.1.0) document, for developers building Applications against Substratal Apps. Browse it in the [Interactive Reference](reference/), download it, or generate a client from it.
{: .fs-6 .fw-300 }

1. TOC
{: toc }

---

## At a glance

| | |
|---|---|
| Format | OpenAPI **3.1.0**, YAML |
| Document | [`openapi.yaml`]({{ site.baseurl }}/openapi.yaml) |
| Interactive view | [Interactive Reference](reference/) (Swagger UI) |
| Base URL | `https://api.substratalapps.com` |
| API version | `Substratal-Version: 1` request header, required (see [Conventions → Versioning](../api-reference/conventions/#versioning)) |
| Authentication | `Authorization: Bearer <token>`: a user access token or an API Key (`bearerAuth`) |
| Content type | `application/json; charset=utf-8` |
| Operations | 118, every one with a stable `operationId` |
| Schemas | 109 reusable components under `components.schemas` |

The prose pages under [API Reference](../api-reference/) are authoritative. `openapi.yaml` is kept in sync with them by hand, and the implementation's contract tests validate every response against it, so a drift between the two fails CI rather than reaching a client. See [Conventions → Machine-readable](../api-reference/conventions/#machine-readable).

## Get the document

```bash
curl -O {{ site.url }}{{ site.baseurl }}/openapi.yaml
```

Every published version of this site keeps its own copy of `openapi.yaml`, so you can pin a client to the spec it was generated from. See [Versions]({{ site.baseurl }}/versions/).

## What the document covers

Operations are grouped by tag, and each tag matches an API Reference page that explains the rules behind it.

| Tag | Operations | Prose reference |
|---|---|---|
| Meta | 1 | [Conventions → Health check](../api-reference/conventions/#health-check) |
| Auth | 21 | [Auth](../api-reference/auth/) |
| Users | 10 | [Users](../api-reference/users/) |
| Profiles | 4 | [Profiles](../api-reference/profiles/) |
| Settings | 4 | [Settings](../api-reference/settings/) |
| Applications | 4 | [Applications](../api-reference/applications/) |
| Entitlements | 6 | [Entitlements](../api-reference/entitlements/) |
| Roles & Permissions | 10 | [Roles & Permissions](../api-reference/roles-and-permissions/) |
| Organizations | 11 | [Organizations](../api-reference/organizations/) |
| Tenancy | 6 | [Tenancy](../api-reference/tenancy/) |
| Billing | 7 | [Billing](../api-reference/billing/) |
| Audit | 2 | [Audit](../api-reference/audit/) |
| Webhooks | 9 | [Webhooks](../api-reference/webhooks/) |
| Notifications | 17 | [Push Notifications](../api-reference/notifications/) |
| API Keys | 6 | [API Keys](../api-reference/api-keys/) |

Some endpoints aren't in the document's `paths`, because they follow external standards instead:

- `GET /.well-known/jwks.json` and `GET /.well-known/openid-configuration`: signing keys and discovery, for verifying app tokens with any standard JWT library. See [Auth → Signing keys](../api-reference/auth/#signing-keys-jwks).
- `/mcp`: the [MCP Server](../mcp-server/), generated from this document's `operationId`s.

## Authentication in the spec

The document declares one security scheme, `bearerAuth` (HTTP bearer), and applies it globally. The few operations that need no credential, such as `GET /health`, signup, and login, override it with `security: []`.

The same header carries both credential types:

| Credential | Looks like | Used by | Get one from |
|---|---|---|---|
| User access token | A JWT | A person, or a client acting for them | `POST /auth/login`, `/auth/signup`, `/auth/mfa/verify`, or `/auth/token/refresh` |
| API Key | `satk_live_…` or `satk_test_…` | Your backend, service to service | [API Keys](../api-reference/api-keys/) |

Some operations accept only one of the two. For example, every Billing operation returns `400 user_token_required` when called with an API Key. The operation's description says so, and its prose page lists the permission it requires.

A `satk_test_…` key works against the same base URL but only ever sees test-mode records. There's no separate sandbox host. See [Conventions → Authentication](../api-reference/conventions/#authentication).

## Reading an operation

Each operation in the document follows the same conventions, so you only need to learn them once:

- **`operationId`** is stable and namespaced (`entitlements.grant`, `users.get`). Generated clients use it for method names, and the MCP server uses it for tool names. Renaming one counts as a breaking change.
- **Errors** all share one shape (`{"error": {"code", "message", "details"}}`, with the request id in the `X-Request-Id` header) through the shared responses `BadRequest`, `Unauthorized`, `Forbidden`, `NotFound`, `Conflict`, `Unprocessable`, `TooManyRequests`, and `ServiceUnavailable`. Branch on `error.code`, never on `message`. See [Conventions → Errors](../api-reference/conventions/#errors).
- **Pagination** uses the shared `Limit` and `Cursor` parameters (`limit` 1–100, default 25). See [Conventions → Pagination](../api-reference/conventions/#pagination).
- **Idempotency.** Operations that create something take the `Idempotency-Key` header. Some require it, and their descriptions say which. See [Conventions → Idempotency](../api-reference/conventions/#idempotency).
- **Concurrency.** Single-resource responses carry an `ETag` header. Send it back as `If-Match` on `PATCH` to avoid overwriting someone else's change. See [Conventions → Concurrency](../api-reference/conventions/#concurrency-etag--if-match).
- **IDs** are typed by prefix (`usr_`, `ent_`, `org_`, …). Most are a prefix plus a ULID. Application and Role ids are derived (`app_<slug>`, `role_<slug>_<name>`). See [Conventions → IDs](../api-reference/conventions/#ids).
- **`me`** can stand in for your own User id in any `/users/{id}` path. See [Conventions → Addressing yourself](../api-reference/conventions/#addressing-yourself-me).

### Vendor extensions

| Extension | Values | Meaning |
|---|---|---|
| `x-substratal-destructive` | `always` \| `conditional` | The operation destroys or irreversibly changes data. It's rejected for API Keys created with `restrict_destructive: true`, and the MCP server marks it `destructiveHint`. See [API Keys → Agent keys](../api-reference/api-keys/#agent-keys--restrict_destructive). |
| `x-substratal-destructive-when` | a description of the request | With `conditional`, only requests matching this are destructive. The rest of the operation stays callable with a restricted key. |
| `x-substratal-phase` | `mvp`, `phase2`, `phase3` | The [Roadmap](../roadmap/) phase the operation ships in. |

Code generators ignore these extensions, so they're safe to leave in.

## Generate a client

Any OpenAPI 3.1 tool works. A few common ones:

```bash
# TypeScript types, with no runtime dependency
npx openapi-typescript {{ site.url }}{{ site.baseurl }}/openapi.yaml -o substratal.d.ts

# Go: types and a client (this is what the reference implementation uses for its server side)
go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest \
  -generate types,client -package substratal openapi.yaml > substratal.gen.go

# Any of 50+ languages
npx @openapitools/openapi-generator-cli generate -i openapi.yaml -g python -o ./substratal-python
```

To explore by hand, import `openapi.yaml` into Postman, Insomnia, Bruno, or any other tool that reads OpenAPI. The [Interactive Reference](reference/) also has "Try it out" built in, against a local devserver.

## Validate your integration

- **Lint the document, or your copy of it.** Run `npx @redocly/cli lint openapi.yaml` or `npx @stoplight/spectral-cli lint openapi.yaml`.
- **Check your mocks against it.** `npx @stoplight/prism-cli mock openapi.yaml` serves a mock API from the document, so you can build a client before you have credentials.
- **Run the reference implementation locally.** The repository's devserver serves every route at `http://localhost:8080` with no AWS account. See the repository's [README → Local development](https://github.com/CompositeCode/substratalapps.com#local-development). The Interactive Reference can send requests to it once its origin is allowed. See the steps at the top of the [Interactive Reference](reference/).

## Versioning and change policy

The API version is the `Substratal-Version` request header (currently `1`), not part of the base URL. The document doesn't declare it on each operation, so configure a generated client to send it as a default header; the Interactive Reference sends it for you. `info.version` in the document (currently a `1.0.0-draft` pre-release) is the version of the document itself.

- **Additive changes** (new optional fields, new operations, new enum values) ship within the current version. Write clients to ignore fields they don't know, and to handle enum values they don't recognize.
- **Breaking changes** ship as the next version number, and the previous version keeps working for at least 6 months. See [Non-Functional Requirements → Versioning](../non-functional-requirements/#versioning) for what counts as breaking.
- Every change is recorded in the [Changelog](../changelog/).
