---
layout: default
title: SCIM
parent: API Reference
nav_order: 9.5
---

# SCIM
{: .no_toc }

How an Organization's own identity provider (Okta, Microsoft Entra ID, Google Workspace, …) invites and removes its members automatically, with [SCIM 2.0](https://datatracker.ietf.org/doc/html/rfc7644).
{: .fs-6 .fw-300 }

1. TOC
{: toc }

---

## What it does, and doesn't

SCIM drives the same membership rules an [org admin](../organizations/) follows by hand. It isn't a way around them:

- **Provisioning a User invites them.** `POST /Users` is [`POST /organizations/{id}/members`](../organizations/#post-organizationsidmembers) by email: the person gets a `pending` membership and an invitation, and joins when they accept it. Until then the membership grants nothing. Activating someone on their behalf stays a platform action, so an identity provider can't attach an existing account to an Organization without its owner agreeing.
- **Deprovisioning removes the membership.** `active: false` (by `PATCH` or `PUT`) or `DELETE` is [`DELETE …/members/{userId}`](../organizations/#delete-organizationsidmembersuserid): the Organization's grants stop reaching the person, `organization.member_removed` and `access.revoked` fire, and the membership is gone. The person's own Substratal account, and anything they have outside the Organization, is untouched. Provisioning them again sends a new invitation.
- **Users only.** Groups, bulk operations, sorting, and ETags aren't supported. Roles aren't set through SCIM: everyone provisioned is a `member`, and org admins promote people with [`PATCH …/members/{userId}`](../organizations/#patch-organizationsidmembersuserid).

SCIM isn't SSO. Members still sign in with their Substratal password until [per-Organization SSO](../auth/#native-auth-and-per-organization-sso) ships.

## Setting it up

An org admin (or `organizations.manage`) creates the Organization's SCIM token, with a User token:

```json
// POST /organizations/{id}/scim-token → 201
{
  "id": "sct_01JAH4M2N3P4Q5R6S7T8V9W0X1",
  "organization_id": "org_01JAGK8N9P0Q1R2S3T4V5W6X7Y",
  "token": "scim_01JAH4M2N3P4Q5R6S7T8V9W0X2",
  "base_url": "https://api.substratalapps.com/scim/v2",
  "created_by": "usr_01JAG3Z9X8QS3F6K2M4N5P6R7S",
  "created_at": "2026-10-21T18:00:00Z",
  "last_used_at": null,
  "test_mode": false
}
```

Give the identity provider `base_url` and `token` (as a bearer token). The secret is shown only in this response. Only its SHA-256 is stored, like an [API Key](../api-keys/)'s.

| Method | Path | Purpose |
|---|---|---|
| `POST` | `/organizations/{id}/scim-token` | Create the token, or rotate it: the previous one stops working at once. Writes `organization.scim_token_created`. |
| `GET` | `/organizations/{id}/scim-token` | Show it (never the secret), including `last_used_at`. `404 scim_token_not_found` when SCIM isn't set up. |
| `DELETE` | `/organizations/{id}/scim-token` | Turn SCIM off. Writes `organization.scim_token_revoked`. |

An Organization has at most one token. A test-mode Organization's token runs in test mode.

## Endpoints

All under `base_url`, with `Authorization: Bearer scim_…`. They don't take `Substratal-Version`: identity providers can't add custom headers, and SCIM's own version is in the path (`/scim/v2`). The token endpoints above are ordinary API calls and need it, like every other request (see [Conventions → Versioning](../conventions/#versioning)). Responses are `application/scim+json`, and errors use SCIM's own shape (`urn:ietf:params:scim:api:messages:2.0:Error`, with `status`, `detail`, and `scimType` where it applies), not the API's usual [error envelope](../conventions/#errors).

| Method | Path | What it does |
|---|---|---|
| `GET` | `/ServiceProviderConfig`, `/ResourceTypes` | Capabilities, for the identity provider's setup check. |
| `GET` | `/Users` | List the Organization's members and pending invitations. `filter` supports `userName eq "…"` (and `emails eq "…"`), case-insensitive. Paged with `startIndex` (1-based) and `count` (default 100, at most 200). |
| `POST` | `/Users` | Invite. `userName` is the email address; if it isn't one, the primary `emails` value is used. `409` `uniqueness` if they're already a member or invited. |
| `GET` | `/Users/{id}` | One member. `id` is their `usr_…` id. |
| `PUT` | `/Users/{id}` | `active: false` deprovisions. Other attributes are accepted and ignored, since the person owns their own profile. |
| `PATCH` | `/Users/{id}` | `replace` (or `add`) of `active` to `false`, as `path: "active"` or inside `value`, deprovisions. `"False"` as a string is accepted too, as some identity providers send it. Other operations are accepted and ignored. |
| `DELETE` | `/Users/{id}` | Deprovision. `204`. |

Removing the Organization's last active `org_admin` is refused (`409`, `scimType: "mutability"`), the same `last_org_admin` rule as everywhere else.

### The User resource

```json
{
  "schemas": ["urn:ietf:params:scim:schemas:core:2.0:User", "urn:substratal:params:scim:schemas:extension:2.0:Membership"],
  "id": "usr_01JAG3Z9X8QS3F6K2M4N5P6R7S",
  "userName": "sam@acme.example",
  "emails": [{ "value": "sam@acme.example", "primary": true }],
  "displayName": "Sam",
  "active": true,
  "meta": { "resourceType": "User", "created": "2026-10-21T18:00:00Z", "location": "https://api.substratalapps.com/scim/v2/Users/usr_01JAG3Z9X8QS3F6K2M4N5P6R7S" },
  "urn:substratal:params:scim:schemas:extension:2.0:Membership": { "status": "active", "role": "member" }
}
```

`active` is `true` for both pending and active memberships, so the identity provider sees what it provisioned. The extension's `status` says whether the person has accepted. `displayName` appears only once they have, the same rule the [member list](../organizations/#get-organizationsidmembers) follows, so provisioning an address never reveals whether it already had an account. The response to a deprovisioning `PATCH` or `PUT` shows `active: false`. After that the User is gone from the Organization, and `GET` returns `404`.

## Audit and limits

SCIM writes produce the same Audit Events as the equivalent org-admin actions, with actor type `system` and actor id the token's `sct_…` id. The token is limited to the default 100 requests a minute.

## Errors

| Status | `scimType` | When |
|---|---|---|
| 400 | `invalidFilter` | A `filter` other than `userName eq "…"`. |
| 400 | `invalidValue` / `invalidSyntax` | No email address to provision, `active: false` on create, or a body that isn't a SCIM User or PatchOp. |
| 401 | | Missing, wrong, or revoked token. |
| 403 | | The Organization is suspended. |
| 404 | | No such User in this Organization, or a resource other than Users. |
| 409 | `uniqueness` | Already a member or invited. |
| 409 | `mutability` | Would remove the last active `org_admin`. |
| 429 | | Rate limited. |
