# Copy, States & Error Messages

Use these strings verbatim (localize via each platform's string catalog). Sentence case on every platform.

## Status and value labels

| Domain value | Label | Tile / row helper text |
|---|---|---|
| entitlement `active` | Active | — |
| `scheduled` | Scheduled | Starts {date} |
| `disabled` | Turned off | Turned off by an administrator |
| `expired` | Expired | Access ended {date} |
| `revoked` | Revoked | Access was removed |
| `none` | No access | Get access · (standard app on Free) Included with Standard |
| source `purchase` / `trial` / `admin_grant` / `org_seat` | Purchased / Trial / Granted by admin / Via {org name} | |
| source `free` | Free | Managed by Substratal |
| source `subscription` | Standard | Your subscription · or · Via {org name}'s subscription |
| app `access_tier` `free` / `standard` | Free / Standard | |
| user `plan` `free` / `standard` | Free / Standard | |
| subscription `active` / `trialing` / `past_due` | Active / Trial / Payment issue | — / Trial ends {date} / We couldn't charge your card. Update it in Manage billing. |
| subscription `canceled` / `unpaid` / `incomplete` / `incomplete_expired` / `paused` | Canceled / Unpaid / Checkout not finished / Checkout expired / Paused | |
| `cancel_at_period_end: true` | Ends {date} | Standard access continues until {date}. |
| user `invited` / `suspended` / `deleted` | Invited / Suspended / Deleted | |
| member scope `all_members` / `allowlist` / `denylist` | Everyone in the organization / Only selected members / Everyone except selected members | |
| tier `shared` / `isolated` / `dedicated_region` | Shared / Isolated / Dedicated region | |
| device `active` / `inactive` | On / Off | |
| device `inactive_reason` `unregistered` / `session_revoked` / `user_suspended` / `user_deleted` / `stale` / `limit` / `credential_removed` | Notifications turned off on the device / Signed out / Account suspended / Account deleted / Not opened in 9 months / Replaced by a newer device / Push setup removed by the app | |
| notification `state` `unread` / `read` / `archived` | Unread / Read / Archived | |
| notification `priority` `high` | Important | |
| notification `source` `platform` | Substratal | |
| push `status` `queued` / `sent` / `failed` / `skipped` | Sending / Pushed to {n} devices / Couldn't push / Not pushed | |
| `skip_reason` `push_not_requested` / `push_disabled` / `no_devices` / `no_credentials` / `credential_failed` / `expired` | Sent to inbox only / Notifications are off for this app / No devices to push to / Push isn't set up for this app / Push setup needs attention / Expired before it could be sent | |
| push credential `active` / `failed` | Active / Needs attention | |

## Confirmations

| Action | Title | Body | Destructive button |
|---|---|---|---|
| Access off | Turn off {app} for {name}? | They'll lose access right away. You can turn it back on later. | Turn off |
| Access off (platform-managed) | Turn off {app} for {name}? | This access comes with their {Free plan / Standard subscription}. It stays off until you turn it back on, even if their plan changes. | Turn off |
| Remove grant (<24h) | Remove this grant? | Use this only for a grant made by mistake. It won't appear as revoked — it'll be gone. | Remove |
| Suspend user | Suspend {name}? | They'll be signed out everywhere, stop receiving push notifications, and can't sign in until reactivated. | Suspend |
| Delete user | Delete {name}? | Their account is deactivated. An admin can restore it. | Delete |
| Delete my account | Delete your account? | Your account is deactivated now and permanently erased on {date}. Until then, support can cancel it. | Delete account |
| Delete my account (paying) | Cancel your subscription first? | You have a Standard subscription. Deleting your account doesn't stop billing — cancel it in Manage billing. | Manage billing (secondary: Delete anyway) |
| Rotate API key | Rotate this key? | The current secret stops working immediately. Update every service that uses it first. | Rotate now |
| Revoke API key | Revoke {key name}? | This can't be undone. | Revoke |
| Rotate webhook secret | Rotate signing secret? | Both the old and new secret are valid for 24 hours. | Rotate |
| Sign out everywhere | Sign out of all other sessions? | Every other device will need to sign in again and will stop receiving push notifications until then. | Sign out |
| Remove member | Remove {name} from {org}? | They lose any app access that came from this organization, including its Standard subscription. | Remove |
| Change org coverage | Update who's covered? | {n} people gain Standard and {m} lose it right away. Billing changes on the next daily sync. | Update |
| Remove device | Stop notifications on {device}? | That device stops getting push notifications until it's set up again. | Remove |
| Turn off push (this device) | Turn off notifications on this device? | You'll still see everything in your Inbox. | Turn off |
| Remove push credential | Remove {APNs / FCM / Web Push}? | {app} stops sending push notifications through {provider}. (Web Push: every browser that signed up for notifications has to sign up again.) | Remove |

## Empty states

| Screen | Title | Body | Action |
|---|---|---|---|
| Home (no apps) | No apps yet | When someone gives you access to an app, it shows up here. | — |
| Inbox | You're all caught up | New alerts and updates from your apps show up here. | — |
| Inbox (Archived) | Nothing archived | Notifications are kept for 90 days. | — |
| Devices | No devices yet | Turn on notifications to get alerts on this device. | Turn on |
| Organizations | No organizations | Create one to manage a team and share apps. | New organization |
| Users (search miss) | No results for "{q}" | Check the spelling or search by user ID. | Clear search |
| API Keys | No API keys | Create a key so your backend can call Substratal. | Create key |
| Webhooks | No webhooks | Get notified when access changes. | Add webhook |
| Push credentials | Push isn't set up | Add APNs, FCM or Web Push so this app can send notifications. | Add provider |
| Audit | No events in this range | Try a wider date range. | — |
| Forbidden | You don't have access to this | Ask an administrator if you think you should. | — |

## Upsell and plan copy

| Where | Copy |
|---|---|
| Standard tile on Free | Included with Standard |
| Upsell sheet | Get every app and service for $6 a month. · Upgrade to Standard |
| Org upsell | Cover your team: $6 per person per month. You choose who's included. |
| Covered by org | Your organization covers you with Standard. |
| Double coverage | Your organization already covers you. You can cancel your own subscription in Manage billing. |
| Waiting after checkout | Finishing up… This usually takes a few seconds. |
| Downgraded | Your Standard access ended on {date}. Your data is kept and comes back when you subscribe again. |
| Business / Enterprise | Need more? Contact sales for business and enterprise plans. |

## Error code → message

| `error.code` | HTTP | Message |
|---|---|---|
| `unauthenticated` | 401 | Your session ended. Sign in again. |
| `session_revoked` | 401 | You were signed out from another device. |
| `forbidden` | 403 | You don't have permission to do that. |
| `not_found` | 404 | This item doesn't exist or was removed. |
| `validation_failed` | 422 | (field-level from `details`) |
| `not_unique` | 409 | That {field} is already in use. |
| `email_taken` | 409 | An account with this email may already exist. Sign in or reset your password. |
| `version_conflict` | 409 | This changed since you opened it. Reload · Re-apply |
| `entitlement_already_exists` | 409 | This person already has access. View it |
| `entitlement_platform_managed` | 409 | This access is managed by Substratal. You can only turn it off or back on. |
| `entitlement_required` | 403 | Access to this app is turned off. |
| `invalid_source` | 422 | Free and Standard access can't be granted by hand. |
| `subscription_exists` | 409 | You already have a subscription. Manage billing |
| `no_billing_account` | 409 | There's no billing account yet. Subscribe first. |
| `plan_requires_sales` | 422 | Business and enterprise plans are arranged with sales. Contact sales |
| `subscription_not_found` | 404 | This organization doesn't have a subscription yet. Subscribe |
| `member_override_not_a_member` | 422 | {name} isn't an active member of this organization. |
| `test_mode_billing_unavailable` | 409 | Billing isn't available in test mode here. |
| `billing_unavailable` | 503 | Billing is temporarily unavailable. Try again in {n}s. |
| `push_credential_exists` | 409 | This provider is already set up. Edit it instead. |
| `invalid_push_credential` | 422 | That key file isn't valid for {provider}. |
| `push_not_configured` | 409 | Push isn't set up for this app yet. |
| `invalid_device_token` | 422 | (silent; retry registration on next launch) |
| `reserved_category` | 422 | Categories starting with security., account. or billing. are reserved. |
| `notification_too_large` | 422 | This notification is {n} bytes over the 4 KB push limit. |
| `invalid_status_transition` | 409 | That change isn't allowed. |
| `user_token_required` | 400 | Sign in as a person to do this. |
| `tenant_suspended` | 403 | This workspace is suspended. Contact support. |
| `destructive_operation_restricted` | 403 | This key can't perform destructive actions. |
| `idempotency_key_reused` | 422 | That request was already sent with different details. Try again. |
| `rate_limited` | 429 | Too many attempts. Try again in {n}s. (Notifications to one person: That person has had too many notifications from this app this hour.) |
| `not_implemented` | 501 | (never shown — the feature is hidden behind its flag) |
| `service_unavailable` | 503 | Substratal is briefly unavailable. Retrying in {n}s. |
| `internal_error` | 500 | Something went wrong on our side. Request ID {id} |
| `plan_limit_reached` | 409 | Reserved by the API for future business plans; nothing returns it today. If seen: You've reached a plan limit. Contact sales |

Always show the `X-Request-Id` (or equivalent) in a "Details" disclosure for 5xx errors so support can trace it.

## Push notification text (platform alerts shown by the Hub)

The API supplies title and body; the Hub supplies actions per category:

| Category | Actions on the notification |
|---|---|
| `security.new_sign_in` | It was me · Review sessions |
| `security.password_changed`, `security.email_changed`, `security.mfa_disabled`, `security.session_reuse_detected` | Secure my account |
| `account.organization_invitation` | Accept · View |
| `account.access_granted` | Open {app} |

## Loading

- Lists: skeleton rows (shape of the real row) after 300 ms; never a full-screen spinner for list refreshes.
- Actions: button shows inline progress and is disabled; the rest of the screen stays interactive.
