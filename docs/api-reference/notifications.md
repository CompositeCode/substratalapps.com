---
layout: default
title: Push Notifications
parent: API Reference
nav_order: 12.5
---

# Push Notifications
{: .no_toc }

How alerts and other updates reach a User: on their devices through Apple Push Notification service (APNs), Firebase Cloud Messaging (FCM), and Web Push, and in an inbox their client reads through this API. See [Domain Model → Notifications](../../domain-model/notifications/) for the entities, and [Webhooks](../webhooks/) for the separate, server-to-server channel that tells an *Application* about changes.
{: .fs-6 .fw-300 }

1. TOC
{: toc }

---

{: .note }
Specified 2026-10-17 and implemented 2026-10-18 ([Changelog](../../changelog/)). Its defaults are settled: platform alerts push only through Applications that opt in, sending is one User per call (no broadcast yet), each recipient gets at most 60 notifications per Application per hour, the inbox keeps 90 days, the `notifications.push` channel is the only preference, and the push providers are sub-processors.

## How it fits together

Four pieces, each owned by a different party:

| Piece | What it is | Who writes it |
|---|---|---|
| [Push credential](#push-credentials) | An Application's own APNs key, FCM service account, or Web Push (VAPID) key pair. Push providers deliver only for the app that holds these, so every Application brings its own. | The Application's owner |
| [Device](#devices) | One install of an Application on one device, signed in as one User, plus the provider token that reaches it. | The User's client, or the Application's backend |
| [Notification](#sending-a-notification) | One message to one User: a title, a body, optional data, and how urgent it is. | The Application's backend (app-scoped key), or the platform itself for [platform alerts](#platform-alerts) |
| [Inbox](#the-inbox) | Every notification a User has received, with read and archived state, readable through the API whether or not the push itself arrived. | The User's client marks items read or archived |

A send writes the Notification and one delivery per eligible device in a single transaction, the same outbox pattern [webhooks](../webhooks/#retries-ordering-and-idempotency) use. A worker then hands each delivery to the provider. A device a provider reports as gone is deactivated automatically, so a send never keeps targeting a dead token.

Push is a notification channel, not an access-control mechanism. It never grants or revokes anything, and an Application still meets the [immediate-revocation requirement](../../trust-model/#how-fast-does-revocation-need-to-land) through `access.revoked` webhooks, not pushes.

## Endpoints

| Method | Path | Requires | Purpose |
|---|---|---|---|
| `GET` | `/applications/{id}/push-credentials` | the owner, the app's own key, or `applications.manage` | List the providers this Application has configured. Secrets are never returned. |
| `POST` | `/applications/{id}/push-credentials` | the owner or `applications.manage`, with a User token | Configure one provider. |
| `GET` | `/applications/{id}/push-credentials/{provider}` | as for the list | Fetch one provider's configuration. |
| `PATCH` | `/applications/{id}/push-credentials/{provider}` | the owner or `applications.manage`, with a User token | Replace key material, change non-secret fields, or re-enable a `failed` credential. |
| `DELETE` | `/applications/{id}/push-credentials/{provider}` | the owner or `applications.manage`, with a User token | Remove a provider. |
| `GET` | `/users/{id}/devices` | self, the app's own key, or `users.list` | List a User's devices. |
| `POST` | `/users/{id}/devices` | self or the app's own key | Register a device, or refresh an existing registration. |
| `GET` | `/users/{id}/devices/{deviceId}` | self, the app's own key, or `users.list` | Fetch one device. |
| `PATCH` | `/users/{id}/devices/{deviceId}` | self or the app's own key | Update a device's name, app version, or rotated token. |
| `DELETE` | `/users/{id}/devices/{deviceId}` | self, the app's own key, or `users.manage` | Unregister a device. |
| `POST` | `/users/{id}/devices/{deviceId}/test` | self or the app's own key | Send a test push to one device and wait for the provider's answer. |
| `POST` | `/users/{id}/notifications` | the app's own key with `notifications.send`, or platform `notifications.send` | Send one notification to one User. |
| `GET` | `/users/{id}/notifications` | self, the app's own key, or `users.list` | List the User's inbox. |
| `GET` | `/users/{id}/notifications/unread-count` | self or the app's own key | Count unread notifications. |
| `POST` | `/users/{id}/notifications/mark-read` | self or the app's own key | Mark many notifications read at once. |
| `GET` | `/users/{id}/notifications/{notificationId}` | self, the app's own key, or `users.list` | Fetch one notification, with its per-device delivery results. |
| `PATCH` | `/users/{id}/notifications/{notificationId}` | self or the app's own key | Mark one notification read, unread, or archived. |

"The app's own key" means any app-scoped [API Key](../api-keys/) scoped to that Application, as on [Settings](../settings/). An app key only ever sees its own Application's push credentials, devices, and notifications. Anything else returns `404`, as though it didn't exist. It never sees [platform alerts](#platform-alerts), which belong to the User. `{id}` may be `me` for a User token (see [Conventions → `me`](../conventions/#addressing-yourself-me)).

**Two ways a client reaches these endpoints.** An Application using [embedded login](../auth/#getting-an-app-token) holds a platform access token and calls the `/users/me/…` endpoints directly. An Application using the hosted flow holds only app tokens, which this API doesn't accept as a Bearer credential, so its client hands the provider token to its own backend, which registers the device with its app key. Both paths produce the same Device. The only difference is that a device registered by an app key isn't tied to a [session](#sessions-and-device-lifecycle).

## Push credentials

APNs, FCM, and browser push services deliver a push only when the sender proves it's the app the token was issued to. So each Application configures its own credentials, and the platform signs every request to the provider with them. There's at most one credential per provider per Application.

Push credentials are Application configuration, shared by test and live mode the same way the [Application](../applications/) row itself is. Test-mode notifications reach only test-mode devices, but they use the same credentials. APNs development builds are handled per device, by `environment: sandbox`, not by a separate credential.

### `POST /applications/{id}/push-credentials`

```json
// Request — APNs, token-based authentication with a .p8 key
{
  "provider": "apns",
  "team_id": "ABCDE12345",
  "key_id": "XYZ987WVU6",
  "topic": "com.substratal.timetrack",
  "private_key": "-----BEGIN PRIVATE KEY-----\nMIGTAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBHkwdwIBAQQg...\n-----END PRIVATE KEY-----\n"
}
```
```json
// Request — FCM, HTTP v1 API with a service account
{
  "provider": "fcm",
  "service_account": {
    "type": "service_account",
    "project_id": "timetrack-prod",
    "private_key_id": "3f1c...",
    "private_key": "-----BEGIN PRIVATE KEY-----\n...\n-----END PRIVATE KEY-----\n",
    "client_email": "push-sender@timetrack-prod.iam.gserviceaccount.com"
  }
}
```
```json
// Request — Web Push. The platform generates the VAPID key pair.
{ "provider": "web_push", "subject": "mailto:push@timetrack.substratalapps.com" }
```
```json
// Response — 201 (APNs)
{
  "id": "pcr_01JAH1A2B3C4D5E6F7G8H9J0K1",
  "application_id": "app_timetrack",
  "provider": "apns",
  "status": "active",
  "team_id": "ABCDE12345",
  "key_id": "XYZ987WVU6",
  "topic": "com.substratal.timetrack",
  "failure_reason": null,
  "failed_at": null,
  "last_used_at": null,
  "created_at": "2026-10-17T15:00:00Z",
  "updated_at": "2026-10-17T15:00:00Z"
}
```

| Provider | Required | Returned (never the secret) |
|---|---|---|
| `apns` | `team_id` (10 characters), `key_id` (10 characters), `topic` (the app's bundle id), `private_key` (the `.p8` file's contents: a PEM PKCS#8 P-256 key) | `team_id`, `key_id`, `topic` |
| `fcm` | `service_account` (the downloaded JSON key, with `type: service_account`, `project_id`, `private_key`, and `client_email`) | `project_id`, `client_email` |
| `web_push` | `subject` (a `mailto:` or `https:` contact URI, sent to browser push services per RFC 8292) | `subject`, `vapid_public_key` (base64url, uncompressed P-256 point) |

- `private_key`, `service_account`, and the generated VAPID private key are **write-only**. They're stored KMS-encrypted, never returned by any endpoint, and never written to an Audit Event, the same treatment as a [webhook signing secret](../conventions/#ids).
- The material is checked for shape on write (the key parses, the JSON has the required fields), and a failure is `422 invalid_push_credential`. It isn't checked against the provider, because that would make the request depend on Apple's or Google's availability. Use [`POST …/devices/{deviceId}/test`](#post-usersiddevicesdeviceidtest) to prove it end to end.
- `vapid_public_key` is what a web client passes as `applicationServerKey` to `PushManager.subscribe()`. The app's own key can read it, so a web client can fetch it from its backend.
- A second credential for the same provider is `409 push_credential_exists`. Use `PATCH` to replace it.
- Writes `push_credential.created`.

Credential management is always done by a person, the same rule as [API Keys](../api-keys/#endpoints). An API Key, including the Application's own, can read credentials but never create, change, or remove them (`400 user_token_required`).

### `GET /applications/{id}/push-credentials`, `GET …/{provider}`

The list returns every configured provider as `{ "data": [ … ] }` with no pagination, since there are at most three. `{provider}` is `apns`, `fcm`, or `web_push`. `status` is one of:

- `active`.
- `failed`, after the provider rejected the credential itself (APNs `403 InvalidProviderToken`, FCM `401`/`403`). `failure_reason` holds the provider's reason code and `failed_at` when it happened. Sends to that provider's devices are skipped with `skip_reason: credential_failed` until the credential is fixed, the owner is emailed, and a [`push_credential.failed`](#webhook-events) webhook fires.

### `PATCH /applications/{id}/push-credentials/{provider}`

```json
// Request — rotate the APNs key
{ "key_id": "NEWKEY1234", "private_key": "-----BEGIN PRIVATE KEY-----\n...\n-----END PRIVATE KEY-----\n" }
```

Writable: the same fields as on `POST` for that provider, plus `status` (only `failed → active`). Any change to key material also returns a `failed` credential to `active`. `provider` and `application_id` are fixed (`422 read_only_field`). The VAPID key pair can't be rotated in place, because every existing browser subscription is bound to it. Delete and recreate the credential instead. Writes `push_credential.updated`. Returns `200` with the credential.

### `DELETE /applications/{id}/push-credentials/{provider}`

`204`. Pending deliveries to that provider are skipped (each delivery's `error` is `credential_removed`), and new sends skip the provider (`skip_reason: no_credentials` when no other provider can reach the User). Devices are kept, because an APNs or FCM token stays valid and works again once a credential is configured. The exception is `web_push`: browser subscriptions are bound to the deleted VAPID key, so every `web_push` device of the Application becomes `inactive` with `inactive_reason: credential_removed`. Writes `push_credential.deleted`. Destructive. See [MCP Server → Tool annotations](../../mcp-server/#tool-annotations--safety).

## Devices

A Device is one install of one Application, signed in as one User, with the provider token that reaches it. The same phone running two Applications is two Devices, because each app gets its own token from the OS.

### `POST /users/{id}/devices`

Called on every app launch and whenever the OS issues a new token, so registration is an idempotent upsert, not a one-time create.

```json
// Request — iOS
{
  "application_id": "app_timetrack",
  "provider": "apns",
  "token": "7f3c9a1e2b4d6f8a0c2e4b6d8f0a2c4e6b8d0f2a4c6e8b0d2f4a6c8e0b2d9f3a",
  "environment": "production",
  "name": "Jordan's iPhone",
  "app_version": "2.4.0"
}
```
```json
// Request — a browser, with the PushSubscription from PushManager.subscribe()
{
  "application_id": "app_timetrack",
  "provider": "web_push",
  "subscription": {
    "endpoint": "https://fcm.googleapis.com/fcm/send/eHk3...:APA91b...",
    "keys": { "p256dh": "BNcRdreALRFXTkOOUHK1EtK2wtaz5Ry4YfYCA_0QTpQtUbVlUls0VJXg7A8u-Ts1XbjhazAkj7I99e8QcYP7DkM", "auth": "tBHItJI5svbpez7KI4CCXg" }
  },
  "name": "Chrome on macOS"
}
```
```json
// Response — 201 for a new registration, 200 for a refresh of an existing one
{
  "id": "dvc_01JAH2B3C4D5E6F7G8H9J0K1M2",
  "user_id": "usr_01JAG3Z9X8QS3F6K2M4N5P6R7S",
  "application_id": "app_timetrack",
  "provider": "apns",
  "environment": "production",
  "token_hint": "…9f3a",
  "name": "Jordan's iPhone",
  "app_version": "2.4.0",
  "session_id": "ses_01JAG8K4Q9R0S1T2V3V4W5X6Y8",
  "status": "active",
  "inactive_reason": null,
  "last_registered_at": "2026-10-17T15:05:00Z",
  "last_delivery_at": null,
  "test_mode": false,
  "created_at": "2026-10-17T15:05:00Z",
  "updated_at": "2026-10-17T15:05:00Z"
}
```

| Field | Rules |
|---|---|
| `application_id` | Required. The User must have active access to it right now ([`allow()`](../../access-control/#the-algorithm)), otherwise `403 entitlement_required`. An app key may only pass its own Application. |
| `provider` | Required. `apns`, `fcm`, or `web_push`. No credential needs to exist yet. A device registered before its credential is configured simply isn't reachable until it is. |
| `token` | Required for `apns` (the device token as hex, 64–200 characters) and `fcm` (the registration token, 1–4,096 characters). Not allowed for `web_push`. |
| `subscription` | Required for `web_push`, and only for it: `endpoint` (an `https` URL on a public host, the same SSRF rules as a [webhook URL](../webhooks/#post-webhooks)) and `keys.p256dh` and `keys.auth` (base64url). |
| `environment` | `apns` only, required: `production` or `sandbox` (a development build, delivered through `api.sandbox.push.apple.com`). |
| `name`, `app_version` | Optional labels, up to 100 and 50 characters, for the User's own device list and for support. |

A malformed `token` or `subscription` is `422 invalid_device_token`.

**What makes a registration "the same" device** is the token: one Device per (`application_id`, `provider`, token) and mode, where a `web_push` device's token is its `endpoint`. Registering an existing token:

- **for the same User** refreshes it and returns `200`. `last_registered_at` moves forward, `name`, `app_version`, and `environment` take the new values, and `session_id` becomes the caller's current session (`null` when an app key registers it). An `inactive` device becomes `active` again and fires `device.registered`.
- **for a different User** (someone else signed in on that install) deletes the previous registration and creates a new Device for the caller (`201`). A device never carries one User's history over to another. The previous owner's Device is gone, and `device.deleted` fires for it.

**The token itself is never returned.** It's stored KMS-encrypted, and responses carry only `token_hint`, its last four characters. Device tokens aren't bearer credentials, but they are personal data, and a client never needs to read one back.

**At most 20 active devices per User per Application.** Registering a 21st deactivates the one registered longest ago (`inactive_reason: limit`).

Self-service and app-key registrations write no Audit Event. They're ordinary account activity, the same as a login.

### `GET /users/{id}/devices`

```json
// Response — 200
{
  "data": [ { "id": "dvc_01JAH2B3C4D5E6F7G8H9J0K1M2", "application_id": "app_timetrack", "provider": "apns", "status": "active", "...": "..." } ],
  "page": { "next_cursor": null, "has_more": false }
}
```

Filters: `application_id`, `provider`, `status`, and `include_inactive=true`. Without an explicit `status` or `include_inactive`, only `active` devices are listed, per [Conventions → Filtering](../conventions/#filtering). An app key gets only its own Application's devices, whatever it passes.

### `PATCH /users/{id}/devices/{deviceId}`

```json
// Request — the OS rotated the token
{ "token": "9a8b7c6d5e4f3a2b1c0d9e8f7a6b5c4d3e2f1a0b9c8d7e6f5a4b3c2d1e0f9a8b", "app_version": "2.5.0" }
```

Writable: `name`, `app_version`, `environment` (`apns`), and the token (`token`, or `subscription` for `web_push`). If the new token is already registered as another Device of the same User and Application, that other registration is deleted, so one token never maps to two Devices. Writing a token reactivates an `inactive` device. `application_id`, `provider`, `user_id`, `session_id`, and `status` are read-only (`422 read_only_field`). Returns `200` with the Device.

### `DELETE /users/{id}/devices/{deviceId}`

`204`. Hard delete: the Device is removed, its pending deliveries are dropped, and `device.deleted` fires. Call this when the User turns notifications off in the app, or from the app's own sign-out if it registered the device with its app key. `users.manage` can delete anyone's device, which writes `device.deleted` to the audit log. Self-service and app-key deletes aren't audited. Destructive.

### `POST /users/{id}/devices/{deviceId}/test`

```json
// Request — optional overrides
{ "title": "Test notification", "body": "If you can see this, push works." }
```
```json
// Response — 200
{
  "device_id": "dvc_01JAH2B3C4D5E6F7G8H9J0K1M2",
  "status": "sent",
  "provider_status": 200,
  "error": null,
  "duration_ms": 184
}
```

Sends one push to one device through the real signing and delivery path, waits up to 10 seconds for the provider's answer, and returns it. `status` is `sent` or `failed`, and `error` carries the provider's reason code (for example `BadDeviceToken`, `UNREGISTERED`, or `InvalidProviderToken`) when there is one. It's for proving a credential and a device work together while integrating.

- `title` defaults to `Test notification from Substratal` and `body` to `null`.
- It's sent whatever the User's push preference says (you asked for it), and to an `inactive` device too, so you can check a device the platform deactivated.
- It isn't stored in the inbox and doesn't count toward the per-recipient limit. It has its own limit of 10 per device per hour, on top of the default per-key bucket. Its result updates the device and credential the same way a real delivery would: an invalid token deactivates the device, and a rejected credential marks it `failed`.
- `409 push_not_configured` if the Application has no credential for the device's provider.

## Sending a notification

### `POST /users/{id}/notifications`

```json
// Request
{
  "category": "timesheet.reminder",
  "priority": "normal",
  "title": "Timesheet due tomorrow",
  "body": "Your timesheet for the week of Oct 12 is due Friday at 5 pm.",
  "data": { "timesheet_id": "ts_8841" },
  "action_url": "com.substratal.timetrack:/timesheets/ts_8841",
  "badge": 1,
  "sound": "default",
  "collapse_key": "timesheet-due-2026-10-12",
  "ttl_seconds": 86400
}
```
```json
// Response — 201
{
  "id": "ntf_01JAH3C4D5E6F7G8H9J0K1M2N3",
  "user_id": "usr_01JAG3Z9X8QS3F6K2M4N5P6R7S",
  "application_id": "app_timetrack",
  "source": "application",
  "category": "timesheet.reminder",
  "priority": "normal",
  "title": "Timesheet due tomorrow",
  "body": "Your timesheet for the week of Oct 12 is due Friday at 5 pm.",
  "data": { "timesheet_id": "ts_8841" },
  "action_url": "com.substratal.timetrack:/timesheets/ts_8841",
  "badge": 1,
  "sound": "default",
  "collapse_key": "timesheet-due-2026-10-12",
  "ttl_seconds": 86400,
  "push": { "requested": true, "status": "queued", "devices_targeted": 2, "devices_sent": 0, "devices_failed": 0, "skip_reason": null },
  "state": "unread",
  "read_at": null,
  "archived_at": null,
  "sender": { "type": "api_key", "id": "key_01JAGE5F6G7H8J9K011M2N304P" },
  "test_mode": false,
  "created_at": "2026-10-17T15:10:00Z",
  "updated_at": "2026-10-17T15:10:00Z"
}
```

The caller is the Application's own key with `notifications.send` (an [app-confined permission](../api-keys/#app-confined-permissions)), and the notification is from that Application. A platform key or User holding platform `notifications.send` may send on behalf of any Application by passing `application_id`. Platform alerts aren't sent through this endpoint. The platform raises them itself (see [Platform alerts](#platform-alerts)).

| Field | Rules |
|---|---|
| `title` | Required. 1–120 characters. |
| `body` | Optional. Up to 1,000 characters. |
| `category` | Required. What kind of notification this is, for the client's grouping, actions, and filters: 1–64 characters matching `^[a-z][a-z0-9_]*(\.[a-z0-9_]+)*$`. The Application chooses its own vocabulary. The prefixes `security.`, `account.`, and `billing.` are reserved for [platform alerts](#platform-alerts) (`422 reserved_category`). |
| `priority` | `normal` (default) or `high`. `high` is for time-sensitive alerts the User needs now. It maps to each provider's urgent delivery class, and providers throttle apps that overuse it. `normal` is for other updates. |
| `push` | Boolean, default `true`. `false` writes the notification to the inbox without a push: an update the User should see next time they look, not one that should buzz their phone. |
| `data` | Optional. Up to 20 keys, each string → string, 2 KB serialized. Delivered to the app with the push and stored with the inbox item. String values only, because FCM's data payload only carries strings. The key `substratal` is reserved. |
| `action_url` | Optional. Where the client should take the User when they open it: an `https` URL or a private-use URI scheme, the same formats as [`redirect_uris`](../applications/#the-application-object). Up to 2,048 characters. |
| `badge` | Optional integer, 0–99,999, for the app icon badge (APNs) or notification count (FCM). `null` leaves the badge alone. |
| `sound` | `default` or `null` (silent). Defaults to `default` for `high` priority and `null` for `normal`. |
| `collapse_key` | Optional. Up to 64 characters of `[A-Za-z0-9._-]`. A newer notification with the same key replaces an older one still waiting on the device or provider, so the User sees one current status instead of a stack. It doesn't change the inbox, where both stay. |
| `ttl_seconds` | How long the provider should keep trying to reach an offline device: 0–2,419,200 (28 days), default 86,400. `0` means deliver now or not at all. It doesn't affect how long the notification stays in the inbox. |
| `application_id` | Only for a platform `notifications.send` caller, and required for one. An app key's sends are always from its own Application. |

**The whole push payload must fit in 4 KB**, the limit all three providers share, after the platform adds its own `substratal` block (see [What a device receives](#what-a-device-receives)). A request that doesn't fit is `422 notification_too_large`, with `details.bytes` and `details.max`.

**The recipient must have active access to the Application right now** (`allow()` passes, which also requires the User to be `active`), otherwise `403 entitlement_required`. A User who has lost access to an app stops hearing from it at the same moment, whatever the sender believes.

**Accepts an optional `Idempotency-Key`**, honored as in [Conventions → Idempotency](../conventions/#idempotency). Send one whenever the send is triggered by an event your backend might process twice, such as a queue message, so a retry doesn't buzz the User's phone twice.

**A send always creates the notification, even when no push goes out.** The response is `201` either way, and `push.status` says what happened to the push:

| `push.status` | Meaning |
|---|---|
| `queued` | Deliveries are on their way to `devices_targeted` devices. |
| `sent` | At least one provider accepted it. `devices_sent` and `devices_failed` count the outcome per device. "Accepted" means the provider took it, not that the User saw it: neither APNs nor FCM reports display. |
| `failed` | Every targeted device failed. |
| `skipped` | No push was attempted. `skip_reason` says why. |

`skip_reason` is one of:

- `push_not_requested`: the request set `push: false`.
- `push_disabled`: the User turned push off, globally or for this Application (see [Preferences](#preferences)).
- `no_devices`: the User has no active device for this Application.
- `no_credentials`: the Application has no credential for any provider the User's devices use.
- `credential_failed`: the matching credentials are all `failed`.
- `expired`: `ttl_seconds` ran out before any attempt could be made.

The notification is in the User's inbox in every case. That's what makes the inbox a reliable place to check, and what lets an Application fall back to another channel when a push can't go out (see [`notification.push_failed`](#webhook-events)).

### Preferences

There's no separate preferences resource. Push follows the `push` channel in the User's existing [Settings](../settings/) `notifications` map, resolved per Application:

- A resolved `notifications.push` of `false`, from global Settings or from this Application's [AppSettings override](../settings/#patch-usersidappsappidsettings), means no push for this Application. The inbox still gets the notification, with `skip_reason: push_disabled`.
- A missing `push` key counts as **on**. Registering a device follows the User granting the OS's own notification permission, which is the real opt-in.

The OS-level permission stays the User's final word. If they revoke it on the device, the provider reports the token invalid on the next send, and the platform deactivates the device.

### Rate limits

Two limits, on top of the [default per-key bucket](../../non-functional-requirements/#rate-limiting):

- **Per sender:** 600 sends per minute per API Key.
- **Per recipient:** 60 notifications per User per Application per rolling hour. Platform alerts don't count toward it. A send over the limit is `429 rate_limited` with `details.scope: "recipient"`, so a bug in one Application's backend can't flood one person's phone.

## The inbox

Every notification a User receives is kept for 90 days, whether or not its push arrived, so a client can always show what the User missed. A client renders it from these endpoints and keeps it in step with what the User does.

### `GET /users/{id}/notifications`

```json
// Response — 200
{
  "data": [
    {
      "id": "ntf_01JAH3C4D5E6F7G8H9J0K1M2N3",
      "application_id": "app_timetrack",
      "source": "application",
      "category": "timesheet.reminder",
      "priority": "normal",
      "title": "Timesheet due tomorrow",
      "body": "Your timesheet for the week of Oct 12 is due Friday at 5 pm.",
      "action_url": "com.substratal.timetrack:/timesheets/ts_8841",
      "state": "unread",
      "created_at": "2026-10-17T15:10:00Z",
      "...": "..."
    }
  ],
  "page": { "next_cursor": null, "has_more": false }
}
```

Newest first. Filters: `state` (`unread`, `read`, `archived`), `application_id`, `source` (`application` or `platform`), `category`, `priority`, `created_after`, `created_before`, and `include_inactive=true`. Archived notifications are excluded unless the caller passes `state=archived` or `include_inactive=true`, the same rule as other terminal states in [Conventions → Filtering](../conventions/#filtering).

Who sees what:

- **The User themself** sees everything addressed to them: every Application's notifications and every platform alert.
- **An app key** sees only its own Application's notifications. The `application_id` filter is forced, and `source=platform` returns nothing.
- **`users.list`** (support) can read any User's inbox, for investigations.

### `GET /users/{id}/notifications/{notificationId}`

The full Notification, as in the send response, plus `deliveries`, one entry per targeted device:

```json
{
  "id": "ntf_01JAH3C4D5E6F7G8H9J0K1M2N3",
  "push": { "requested": true, "status": "sent", "devices_targeted": 2, "devices_sent": 1, "devices_failed": 1, "skip_reason": null },
  "deliveries": [
    { "id": "ndl_01JAH3D4E5F6G7H8J9K0M1N2P3", "device_id": "dvc_01JAH2B3C4D5E6F7G8H9J0K1M2", "provider": "apns", "status": "sent", "attempts": 1, "error": null, "sent_at": "2026-10-17T15:10:02Z" },
    { "id": "ndl_01JAH3D4E5F6G7H8J9K0M1N2P4", "device_id": null, "provider": "fcm", "status": "failed", "attempts": 1, "error": "UNREGISTERED", "sent_at": null }
  ],
  "...": "..."
}
```

`device_id` is `null` once that Device has been deleted. Delivery details are kept for 30 days, after which `deliveries` is `[]` and only the `push` summary remains.

### `PATCH /users/{id}/notifications/{notificationId}`

```json
// Request
{ "state": "read" }
```

`state` is the only writable field. Every transition is allowed:

| To | Effect |
|---|---|
| `read` | Sets `read_at` (or keeps it). From `archived`, also clears `archived_at`. |
| `unread` | Clears `read_at` and `archived_at`. |
| `archived` | Sets `archived_at`, and `read_at` too if it was unset. The notification leaves the default list. |

Returns `200` with the Notification. No Audit Event: this is the User's own reading state, not an access change.

### `POST /users/{id}/notifications/mark-read`

```json
// Request — everything from TimeTrack up to a point in time
{ "application_id": "app_timetrack", "before": "2026-10-17T15:10:00Z" }
```
```json
// Response — 200
{ "updated_count": 4 }
```

Marks every `unread` notification matching the filters as `read`. Each filter is optional: `application_id`, `source`, `category`, and `before` (only notifications created at or before this time, so items that arrive while the User is looking aren't swept up). An empty body marks everything the caller can see. An app key's `application_id` is forced to its own.

### `GET /users/{id}/notifications/unread-count`

```json
// Response — 200
{
  "unread_count": 3,
  "applications": [
    { "application_id": "app_timetrack", "unread_count": 2 },
    { "application_id": null, "unread_count": 1 }
  ]
}
```

Unread, unarchived notifications. `application_id: null` is the platform alerts. Accepts the `application_id` and `source` filters. An app key gets only its own Application's count. A client typically calls it on launch and after handling a push, to set its badge.

## Platform alerts

The platform raises a small, fixed set of notifications itself, about the User's account rather than any one Application. They use the same Notification object, with `source: platform`, `application_id: null`, `sender: {"type": "system", "id": "system"}`, and a category from this catalog:

| `category` | `priority` | Raised when | `data` |
|---|---|---|---|
| `security.new_sign_in` | `high` | A login creates a session from a user agent not seen in the User's sessions during the last 90 days. A User's first session, at signup, raises nothing. | `session_id`, `user_agent`, `ip_address` |
| `security.password_changed` | `high` | `POST /users/{id}/password`, or a password reset. | — |
| `security.email_changed` | `high` | An email change completes. | — |
| `security.mfa_disabled` | `high` | TOTP is turned off by the User, or reset by support. | — |
| `security.session_reuse_detected` | `high` | A reused refresh token revokes a session ([`refresh_token_reused`](../auth/#post-authtokenrefresh)). | `session_id` |
| `account.organization_invitation` | `normal` | A `pending` membership is created for an existing User ([Organizations → Adding members](../organizations/#post-organizationsidmembers)). | `organization_id`, `invited_by` |
| `account.access_granted` | `normal` | The User gains resolved access to an Application (the same moment `access.granted` fires). Raised only for access a developer or admin grants (`purchase`, `trial`, `admin_grant`, or `org_seat`), never for [platform-managed](../../domain-model/entitlements/#platform-managed-entitlements) `free` or `subscription` Entitlements: signing up, subscribing to Standard, or a new free Application would otherwise push one alert per app. Also not raised within 60 seconds of signup. | `application_id` |

`billing.` categories are reserved for [subscription](../billing/) alerts, such as a failed payment. None are sent yet; Stripe emails the customer directly.

These run alongside the emails the platform already sends for the same events, and don't replace them. A User who has never registered a device still gets the email.

**Which devices get a platform alert's push.** A device belongs to an Application, and only an Application's own credentials can reach it. So a platform alert is pushed only to devices of Applications that have opted in with [`receives_platform_notifications: true`](../applications/#the-application-object), typically a first-party Substratal app. Without that, a platform alert goes to the inbox (and the email above) but makes no push, and `skip_reason` is `no_devices`. Opting in is the owner's choice, because the alert arrives under that app's name and icon. The `push` preference applies per device, resolved for that device's Application.

**Targets are chosen when the alert is raised.** An action that also revokes sessions, such as a password reset or a detected token reuse, raises its alert before the revocation in the same transaction. The User's devices still get the warning, even though they're about to be signed out. That exception applies only to `security.*` alerts. Any other pending delivery to a device that has since become `inactive` is skipped.

Platform alerts never reach an Application through webhooks, and an app key can't list them. They're between the platform and the User.

## What a device receives

The platform builds each provider's payload from the Notification. Every payload carries a `substratal` block, so the app can match the push to its inbox item and mark it read:

```json
{
  "substratal": {
    "notification_id": "ntf_01JAH3C4D5E6F7G8H9J0K1M2N3",
    "application_id": "app_timetrack",
    "source": "application",
    "category": "timesheet.reminder",
    "action_url": "com.substratal.timetrack:/timesheets/ts_8841"
  }
}
```

| Notification field | APNs | FCM (HTTP v1) | Web Push |
|---|---|---|---|
| `title`, `body` | `aps.alert.title`, `aps.alert.body` | `message.notification.title`, `.body` | In the encrypted JSON payload |
| `priority: high` / `normal` | `apns-priority: 10` with `aps.interruption-level: time-sensitive` / `apns-priority: 5` with `active` | `android.priority: HIGH` / `NORMAL` | `Urgency: high` / `normal` |
| `ttl_seconds` | `apns-expiration` | `android.ttl` | `TTL` header |
| `collapse_key` | `apns-collapse-id` | `android.collapse_key` | `Topic` header |
| `badge` | `aps.badge` | `android.notification.notification_count` | In the payload |
| `sound` | `aps.sound` | `android.notification.sound` | Not applicable |
| `category` | `aps.category`, so an app can register actionable categories | In `message.data` | In the payload |
| `data` and the `substratal` block | Top-level keys beside `aps` | `message.data`, with `substratal` as a JSON string | In the payload |

Web Push payloads are encrypted for the subscription's keys (RFC 8291, `aes128gcm`), and every request is signed with the Application's VAPID key (RFC 8292). APNs requests use token-based authentication with the Application's key, and FCM requests use OAuth 2.0 access tokens minted from its service account. The platform caches and refreshes each of these, so an Application only supplies the long-lived credential.

**Keep sensitive content out of `title`, `body`, and `data`.** Apple, Google, and the browser vendors' push services carry the payload, and a lock screen shows the title and body to anyone holding the phone. For anything private, send a generic title, put an id in `data`, and let the app fetch the details once it's open and authenticated. The payload is also what the [Compliance](../../compliance/#gdpr-and-ccpa--build-for-it-now) page counts these providers as sub-processors for.

## Delivery

- **Latency target:** p95 under 10 seconds from the committing write to the first provider attempt, the same as webhooks. Deliveries are written in the same transaction as the notification, so a committed send can never lose its push.
- **Retries:** a provider `429`, a `5xx`, or a timeout (5 seconds) is retried at +30 seconds, +2 minutes, and +10 minutes, for 4 attempts in total, within `ttl_seconds`. Then that delivery is `failed`. A provider's `Retry-After` is honored when it's longer.
- **A token the provider reports as gone deactivates the device** (`inactive_reason: unregistered`) and fires `device.deactivated`. That's APNs `410 Unregistered` or `400 BadDeviceToken`/`DeviceTokenNotForTopic`, FCM `404 UNREGISTERED` or a `400 INVALID_ARGUMENT` naming the token, and Web Push `404`/`410`. Nothing is retried for that device.
- **A credential the provider rejects marks it `failed`**, as described under [Push credentials](#push-credentials). Every pending delivery that depends on it is skipped.
- **No ordering guarantee,** and a provider may deliver late or not at all. Clients should treat a push as a prompt to refresh from the API, not as the source of truth. `collapse_key` is the tool for "show only the latest".
- **No delivery receipts.** `sent` means the provider accepted the push. Whether the User saw it isn't knowable from the provider, and the inbox's `read` state is the only "seen" signal.

## Sessions and device lifecycle

A device a User registers with their own token is tied to the session that registered it (`session_id`), so signing out stops its pushes:

| Event | Effect on devices |
|---|---|
| Logout, or [revoking that session](../auth/#delete-usersidsessionssessionid) | Devices registered under that session become `inactive` (`session_revoked`). Logout from every session deactivates all of them. |
| Password reset, refresh-token reuse, or MFA reset (each revokes sessions) | The same, after the [`security.*` alert](#platform-alerts) is raised. |
| The User is suspended or soft-deleted, or a test-mode User expires | Every device they hold becomes `inactive` (`user_suspended`, `user_deleted`, or `test_user_expired`), including devices an app key registered. |
| The User signs in again on the same install | The client re-registers its token, which reactivates the device under the new session. |
| An app-key registration (`session_id: null`) | Not affected by logout, because the platform can't see the app's own sessions. The Application's backend should `DELETE` the device when its user signs out. |
| No re-registration for 270 days | `inactive` (`stale`). Providers expire tokens from installs that are never opened, so a client should re-register on launch, at most once a day. |
| The Application's `web_push` credential is deleted | Its `web_push` devices become `inactive` (`credential_removed`). |
| The User loses access to the Application | Devices stay `active`, but sends to the User are rejected (`403 entitlement_required`) until access returns. |

An `inactive` device is deleted 30 days after it was deactivated, unless it's re-registered first.

## Test mode

A test-mode key registers test-mode devices and sends test-mode notifications. Test-mode notifications reach only test-mode devices, and live notifications never reach a test-mode device, the same isolation as every other record (see [API Keys → Test vs. live](../api-keys/#test-vs-live)). Pushes are real: a test-mode delivery goes through the actual provider, using the Application's shared credentials. Point test-mode devices at development builds (`environment: sandbox` on APNs) so test traffic never reaches a production install.

## Webhook events

Five events tell an Application's backend about its devices, credentials, and failed pushes. They're delivered to the Application's [webhook subscriptions](../webhooks/#event-types) like any other event, never include a token or secret, and are scoped to their `application_id`:

| Event | Fires when | `data` |
|---|---|---|
| `device.registered` | A Device is created, or an `inactive` one is reactivated. | `device` |
| `device.deactivated` | A Device becomes `inactive`. | `device` (with `inactive_reason`) |
| `device.deleted` | A Device is deleted, by `DELETE`, by a registration of its token for another User, or 30 days after deactivation. | `device` (last state) |
| `notification.push_failed` | One of the Application's own notifications ends with `push.status` `failed`, or `skipped` for `no_devices`, `no_credentials`, or `credential_failed`. Not for `push_disabled` or `push_not_requested`, which are choices rather than failures, and never for a platform alert. | `notification_id`, `user_id`, `category`, `push` |
| `push_credential.failed` | A credential becomes `failed`. | `provider`, `failure_reason` |

`notification.push_failed` is the hook for falling back to another channel, such as email from your own backend, when a push can't go out.

## Errors specific to this resource

| Code | Status | When |
|---|---|---|
| `push_credential_not_found` | 404 | `{provider}` isn't configured for the Application, or isn't one of `apns`, `fcm`, `web_push`. |
| `push_credential_exists` | 409 | `POST` for a provider the Application already has. |
| `invalid_push_credential` | 422 | Key material that doesn't parse, or a service account missing a required field. |
| `push_not_configured` | 409 | A device test for a provider the Application has no credential for. |
| `device_not_found` | 404 | `{deviceId}` doesn't resolve, isn't the User's, or (for an app key) belongs to another Application. |
| `invalid_device_token` | 422 | A `token` or `subscription` in the wrong format for its provider, or a `web_push` endpoint that isn't a public `https` URL. |
| `notification_not_found` | 404 | `{notificationId}` doesn't resolve or isn't visible to the caller. |
| `reserved_category` | 422 | An Application's send uses a `security.`, `account.`, or `billing.` category. |
| `notification_too_large` | 422 | The push payload would exceed 4 KB. `details.bytes` and `details.max` say by how much. |
| `entitlement_required` | 403 | The recipient (or the User registering a device) doesn't have active access to the Application. |
| `user_token_required` | 400 | An API Key tried to create, change, or remove a push credential, or used `me`. |
| `read_only_field` | 422 | A `PATCH` touched a read-only field, such as a device's `provider` or a credential's `application_id`. |
| `invalid_status_transition` | 409 | A credential `PATCH status` other than `failed → active`. |
| `rate_limited` | 429 | Over the per-sender or per-recipient limit. `details.scope` is `sender` or `recipient`. |
