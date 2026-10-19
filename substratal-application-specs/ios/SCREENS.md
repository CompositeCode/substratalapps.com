# iOS / iPadOS — Screen Specs

All 75 screens with the native iOS / iPadOS container and controls for each element. Behavior, data and copy are identical to `shared/SCREEN-SPECS.md`; construction rules for this platform are in `PLATFORM-SPEC.md`.

## Container per archetype

| Archetype | Container |
|---|---|
| auth | `fullScreenCover` |
| grid | Tab root `NavigationStack`, `LazyVGrid` |
| list | `List` in `NavigationStack` (iPad: split view) |
| table | `List` (iPad regular width: `Table`) |
| detail | Pushed view; segmented `Picker`; toolbar actions |
| tab | Segment content |
| form | `.sheet` with `NavigationStack` + `Form` (Cancel / primary in toolbar) |
| settings | Inset-grouped `List`/`Form` |
| confirm | `.alert` (with text field) or `.confirmationDialog` |
| secret | Sheet, `interactiveDismissDisabled`, hide on screenshot |
| wizard | Sheet with `NavigationStack` push per step |
| popover | `.sheet` with `.presentationDetents([.medium, .large])` |
| handoff | `openURL` / `ASWebAuthenticationSession` |
| overlay | Sheet or search tab |
| hub | Admin / Developer tab root `List` |

## D1 · Sign-in & Identity

### AUTH-01 — Sign in
**Domain:** D1 Sign-in & Identity · **Route:** `/sign-in` · **Archetype:** auth · **Phase:** MVP  
**Who:** Unauthenticated  
**Purpose:** Exchange email + password for a session, or start an MFA challenge.

**iOS / iPadOS container:** `fullScreenCover`

**Entry points:** App launch with no stored session; Session expired / revoked (preserves `?next=` destination); Sign out

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Email | email-field | TextField .textContentType(.username) .keyboardType(.emailAddress) | body.email | required; RFC 5322; trim; autofill username |
| Password | password-field | SecureField .textContentType(.password/.newPassword) | body.password | required; reveal toggle; autofill current-password |
| Sign in | primary-button | Button .buttonStyle(.glassProminent/.borderedProminent) | POST /auth/login | disabled until both fields non-empty |
| Forgot password? | link | Button .borderless | → AUTH-05 |  |
| Create an account | link | Button .borderless | → AUTH-03 |  |
| Continue with SSO | button | Button .bordered | POST /auth/sso/{provider}/callback | hidden behind flag `sso` (API returns 501 today) |
| Sign in with a passkey | button | Button .bordered | — | hidden behind flag `passkeys`; reserved for when the API supports WebAuthn |

| Action | Call | Result handling |
|---|---|---|
| Sign in | `POST /auth/login` | 200 AuthSession → store tokens → go to `next` or APP-01. 200 MfaChallenge → AUTH-02 with mfa_token. |
| — |  | 401 → inline form error "Email or password is incorrect." (never reveal which). 403 → account suspended/locked message. 429 → disable button, countdown from Retry-After. |

**States:** Submitting: button spinner, fields read-only; Locked out (5 failures / 15 min): explain wait time

**Notes:** `test_mode` is never exposed in end-user UI; developer builds may show a hidden toggle (long-press logo / ⌥-click) for test-mode sign-in. The horizontal logo lockup (BrandLockup, `brand/BRAND.md`) sits above the form.

### AUTH-02 — Two-factor challenge
**Domain:** D1 Sign-in & Identity · **Route:** `/sign-in/mfa` · **Archetype:** auth · **Phase:** MVP  
**Who:** Holder of mfa_token  
**Purpose:** Complete sign-in with a TOTP code or a recovery code.

**iOS / iPadOS container:** `fullScreenCover`

**Entry points:** AUTH-01 returned MfaChallenge

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Code | otp-field | TextField .textContentType(.oneTimeCode) | body.code | 6 digits; paste fills all; auto-submit on 6th digit; OS one-time-code autofill |
| Use a recovery code | link | Button .borderless | swaps to Recovery code field | only if `methods` includes recovery_code |
| Recovery code | text-field | TextField | body.recovery_code | format xxxx-xxxx; case-insensitive |
| Expiry countdown | caption | Text .footnote | MfaChallenge.expires_in | announce at 60 s and 10 s |
| Verify | primary-button | Button .buttonStyle(.glassProminent/.borderedProminent) | POST /auth/mfa/verify |  |

| Action | Call | Result handling |
|---|---|---|
| Verify | `POST /auth/mfa/verify` | 200 → same as AUTH-01 success. 401 → "That code didn't work." clear field. Expired → back to AUTH-01 with message. 429 → countdown. |

**States:** Challenge expired

### AUTH-03 — Create account
**Domain:** D1 Sign-in & Identity · **Route:** `/sign-up` · **Archetype:** auth · **Phase:** MVP  
**Who:** Unauthenticated  
**Purpose:** Self-service signup with email and password.

**iOS / iPadOS container:** `fullScreenCover`

**Entry points:** AUTH-01 link; Marketing/app deep link with optional `application_id`

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Display name | text-field | TextField | body.display_name | optional; ≤ 100 |
| Email | email-field | TextField .textContentType(.username) .keyboardType(.emailAddress) | body.email | required |
| Password | password-field | SecureField .textContentType(.password/.newPassword) | body.password | required; strength meter; autofill new-password; show policy hint |
| Create account | primary-button | Button .buttonStyle(.glassProminent/.borderedProminent) | POST /auth/signup |  |
| Terms / Privacy | links | Link | external |  |

| Action | Call | Result handling |
|---|---|---|
| Create account | `POST /auth/signup` | 201 AuthSession → AUTH-04 (verify pending) then APP-01. 409 email_taken → "An account with this email may already exist. Sign in or reset your password." (with details.reason invitation_pending the API re-sends the invitation: say "Check your email for your invitation.") 422 → field errors. |

**Notes:** Pass `application_id` through when signup started from an app's link; it is recorded as signup_application_id. Every new User starts on the Free plan and receives platform-managed `free` Entitlements to every free Application, including the Hub itself (`app_hub`). No plan choice at signup; upgrading is BIL-01.

### AUTH-04 — Verify your email
**Domain:** D1 Sign-in & Identity · **Route:** `/verify-email` · **Archetype:** auth · **Phase:** MVP  
**Who:** Signed in (unverified) or token holder  
**Purpose:** Tell the user to check email; complete verification from the emailed link.

**iOS / iPadOS container:** `fullScreenCover`

**Entry points:** After signup; Persistent banner on APP-01 while `email_verified` is false; Deep link `/verify-email?token=…`

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Instruction | text | Text | User.email | "We sent a link to {email}." |
| Resend email | button | Button .bordered | POST /auth/email/verify/resend | 30 s cooldown after each send |
| Continue to Substratal | button | Button .bordered | → APP-01 | unverified users may continue |
| Change email | link | Button .borderless | → ACC-02 |  |

| Action | Call | Result handling |
|---|---|---|
| Deep link opened | `POST /auth/email/verify {token}` | 204 → success state, refresh User. 400 → "This link has expired." + Resend. |
| Resend email | `POST /auth/email/verify/resend` | Always 202 → toast "If that address needs verifying, we sent a new link." |

### AUTH-05 — Forgot password
**Domain:** D1 Sign-in & Identity · **Route:** `/forgot-password` · **Archetype:** auth · **Phase:** MVP  
**Who:** Unauthenticated  
**Purpose:** Request a password-reset email.

**iOS / iPadOS container:** `fullScreenCover`

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Email | email-field | TextField .textContentType(.username) .keyboardType(.emailAddress) | body.email | prefilled from AUTH-01 |
| Send reset link | primary-button | Button .buttonStyle(.glassProminent/.borderedProminent) | POST /auth/password/forgot |  |

| Action | Call | Result handling |
|---|---|---|
| Send reset link | `POST /auth/password/forgot` | Always 202 → confirmation "If an account exists for {email}, we sent a reset link." 429 → countdown. |

### AUTH-06 — Set a new password
**Domain:** D1 Sign-in & Identity · **Route:** `/reset-password` · **Archetype:** auth · **Phase:** MVP  
**Who:** Reset-token holder  
**Purpose:** Choose a new password from the emailed reset link.

**iOS / iPadOS container:** `fullScreenCover`

**Entry points:** Deep link `/reset-password?token=…`

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| New password | password-field | SecureField .textContentType(.password/.newPassword) | body.new_password | strength meter; autofill new-password |
| Confirm password | password-field | SecureField .textContentType(.password/.newPassword) | client only | must match |
| Notice | caption | Text .footnote |  | "You'll be signed out on all devices." |
| Save password | primary-button | Button .buttonStyle(.glassProminent/.borderedProminent) | POST /auth/password/reset |  |

| Action | Call | Result handling |
|---|---|---|
| Save password | `POST /auth/password/reset` | 204 → clear local session → AUTH-01 with success banner. 400 → expired link + link to AUTH-05. 422 → field errors. |

### AUTH-07 — Accept invitation
**Domain:** D1 Sign-in & Identity · **Route:** `/invite` · **Archetype:** auth · **Phase:** MVP  
**Who:** Invitation-token holder  
**Purpose:** Activate an account an admin created: set password and sign in.

**iOS / iPadOS container:** `fullScreenCover`

**Entry points:** Deep link `/invite?token=…` from the invitation email

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Display name | text-field | TextField | body.display_name | optional |
| Password | password-field | SecureField .textContentType(.password/.newPassword) | body.password | strength meter |
| Accept and continue | primary-button | Button .buttonStyle(.glassProminent/.borderedProminent) | POST /auth/invitations/accept |  |

| Action | Call | Result handling |
|---|---|---|
| Accept and continue | `POST /auth/invitations/accept` | 200 AuthSession → APP-01. 400 → "This invitation is no longer valid. Ask your admin to resend it." |


## D2 · Account & Security

### ACC-01 — Account & profile
**Domain:** D2 Account & Security · **Route:** `/account` · **Archetype:** settings · **Phase:** MVP  
**Who:** Self  
**Purpose:** View and edit the signed-in user's global profile.

**iOS / iPadOS container:** Inset-grouped `List`/`Form`

**Loads:** `GET /users/me` · `GET /users/me/profile`

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Avatar | image-picker | PhotosPicker + camera | Profile.avatar_url | camera / photo library / file / paste; client uploads to app asset storage then PATCHes URL* |
| Display name | text-field | TextField | Profile.display_name | required; ≤ 100; save on blur/Return |
| Email | row → ACC-02 | NavigationLink row | User.email + email_verified + pending_email | shows "Pending: {pending_email}" when set |
| User ID | id-chip | Text(.monospaced) + copy | User.id | copy |
| Contact email | email-field | TextField .textContentType(.username) .keyboardType(.emailAddress) | Profile.contact_email | optional |
| Contact phone | phone-field | TextField .keyboardType(.phonePad) | Profile.contact_phone | optional; E.164 formatting by locale |
| Plan | row → BIL-01 | NavigationLink row | User.plan free\|standard + plan_sources | "Free" or "Standard · via {org}" / "Standard · your subscription" |
| Member since / last sign-in | caption | Text .footnote | User.created_at, last_login_at | locale formatted |
| Sign out | button | Button .bordered | POST /auth/logout | always visible at the bottom of Account (mobile) / account menu (desktop) |

| Action | Call | Result handling |
|---|---|---|
| Edit field | `PATCH /users/me/profile (If-Match)` | Save per field; 409 version_conflict → reload prompt; 422 → field error. |
| Sign out | `POST /auth/logout` | best effort; always wipe local credentials and cache → AUTH-01 |

**States:** Loading: skeleton after 300 ms; Error: inline with Retry; codes per COPY-AND-STATES

**Notes:** *The API stores `avatar_url` only. Upload target is a client-side decision (spec gap — see OPEN-QUESTIONS).

### ACC-02 — Change email
**Domain:** D2 Account & Security · **Route:** `/account/email` · **Archetype:** form · **Phase:** MVP  
**Who:** Self  
**Purpose:** Start an email change; the new address must be verified.

**iOS / iPadOS container:** `.sheet` with `NavigationStack` + `Form` (Cancel / primary in toolbar)

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| New email | email-field | TextField .textContentType(.username) .keyboardType(.emailAddress) | body.email | required; differs from current |
| Save | primary-button | Button .buttonStyle(.glassProminent/.borderedProminent) | PATCH /users/me |  |

| Action | Call | Result handling |
|---|---|---|
| Save | `PATCH /users/me {email}` | 200 → User.pending_email set; show AUTH-04-style "check your new inbox". 409 not_unique → "That email is already in use." |

### ACC-03 — Change password
**Domain:** D2 Account & Security · **Route:** `/account/security/password` · **Archetype:** form · **Phase:** MVP  
**Who:** Self  
**Purpose:** Change password knowing the current one.

**iOS / iPadOS container:** `.sheet` with `NavigationStack` + `Form` (Cancel / primary in toolbar)

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Current password | password-field | SecureField .textContentType(.password/.newPassword) | body.current_password | autofill current-password |
| New password | password-field | SecureField .textContentType(.password/.newPassword) | body.new_password | strength meter; autofill new-password |
| Save | primary-button | Button .buttonStyle(.glassProminent/.borderedProminent) | POST /users/me/password |  |

| Action | Call | Result handling |
|---|---|---|
| Save | `POST /users/me/password` | 204 → toast "Password changed". 401 → "Current password is incorrect." 422 → policy errors. |

### ACC-04 — Security
**Domain:** D2 Account & Security · **Route:** `/account/security` · **Archetype:** settings · **Phase:** MVP  
**Who:** Self  
**Purpose:** Password, two-factor and active sessions in one place.

**iOS / iPadOS container:** Inset-grouped `List`/`Form`

**Loads:** `GET /users/me` · `GET /users/me/sessions`

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Password | row → ACC-03 | NavigationLink row |  | "Change…" |
| Two-factor authentication | toggle | Toggle | User.mfa_enabled | on → ACC-05; off → ACC-06 |
| Sessions | list | List | Session[]: user_agent → device name/icon, ip_address → approximate location*, last_seen_at, current | current session marked "This device" and not revocable here |
| Sign out | row-button | trailing Button/swipe action | DELETE /users/me/sessions/{sessionId} | per non-current row |
| Sign out of all other sessions | destructive-button | Button(role:.destructive) | DELETE /users/me/sessions/{sessionId} for each non-current session | confirm first; also stops push to those sessions' devices |
| Devices & notifications | row → NTF-03 | NavigationLink row | count of active devices |  |

| Action | Call | Result handling |
|---|---|---|
| Revoke one | `DELETE /users/me/sessions/{sessionId}` | Optimistic remove; 404 → already gone, refresh. |
| Sign out everywhere else | `DELETE each non-current session` | Confirm first (COPY). Uses per-session deletes so the current session survives; `logout all_sessions` signs this device out too. Revoking a session deactivates the push devices it registered. |

**States:** Loading: skeleton after 300 ms; Error: inline with Retry; codes per COPY-AND-STATES

**Notes:** *IP → city lookup is client-side/optional; never show the raw IP to non-staff unless the user expands details.

### ACC-05 — Turn on two-factor
**Domain:** D2 Account & Security · **Route:** `/account/security/2fa/enable` · **Archetype:** wizard · **Phase:** MVP  
**Who:** Self  
**Purpose:** Enroll an authenticator app (TOTP) and save recovery codes.

**iOS / iPadOS container:** Sheet with `NavigationStack` push per step

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Step 1 — Scan | qr + secret | Image (CoreImage QR) | POST /users/me/mfa/totp → otpauth_uri, secret | QR, "Copy setup key", "Open in authenticator" (otpauth:// link on mobile) |
| Step 2 — Confirm | otp-field | TextField .textContentType(.oneTimeCode) | body.code | 6 digits |
| Step 3 — Recovery codes | secret-list | SecretReveal + ShareLink/Save to Files | confirm response: 10 recovery codes | Copy all · Save as file · Print · Share (mobile); "I've saved these" checkbox gates Done |

| Action | Call | Result handling |
|---|---|---|
| Start | `POST /users/me/mfa/totp` | 409 → already enabled; close. |
| Confirm | `POST /users/me/mfa/totp/confirm` | 200 → step 3. 401 → wrong code. |

**States:** Cancel before confirm leaves MFA off (no server cleanup needed)

### ACC-06 — Turn off two-factor
**Domain:** D2 Account & Security · **Route:** `/account/security/2fa/disable` · **Archetype:** confirm · **Phase:** MVP  
**Who:** Self  
**Purpose:** Disable TOTP with a current code or a recovery code.

**iOS / iPadOS container:** `.alert` (with text field) or `.confirmationDialog`

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Code or recovery code | otp-field / text-field | TextField .textContentType(.oneTimeCode) | body.code \| body.recovery_code |  |
| Turn off | destructive-button | Button(role:.destructive) | POST /users/me/mfa/totp/disable |  |

| Action | Call | Result handling |
|---|---|---|
| Turn off | `POST /users/me/mfa/totp/disable` | 204 → toggle off, toast. 401 → wrong code. 429 → countdown. |

### ACC-07 — Privacy
**Domain:** D2 Account & Security · **Route:** `/account/privacy` · **Archetype:** settings · **Phase:** MVP  
**Who:** Self  
**Purpose:** Export personal data and delete the account.

**iOS / iPadOS container:** Inset-grouped `List`/`Form`

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Download my data | button | Button .bordered | GET /users/me/export | saves `substratal-export-{date}.json` via save dialog / share sheet / download |
| Delete account | destructive-row → ACC-08 | Button(role:.destructive) row |  |  |

| Action | Call | Result handling |
|---|---|---|
| Download | `GET /users/me/export` | 200 → save file. 429 → "You can export again in {n} minutes." |

### ACC-08 — Delete account
**Domain:** D2 Account & Security · **Route:** `/account/privacy/delete` · **Archetype:** confirm · **Phase:** MVP  
**Who:** Self  
**Purpose:** Request erasure: deactivate now, erase after 7 days.

**iOS / iPadOS container:** `.alert` (with text field) or `.confirmationDialog`

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| What happens | text | Text |  | list: signed out everywhere; apps lose access; erased on {date}; support can cancel within 7 days |
| Reason | text-area | TextField(axis:.vertical) | body.reason | optional |
| Type DELETE | text-field | TextField | client only | must equal localized confirm word |
| Delete account | destructive-button | Button(role:.destructive) | POST /users/me/erasure-requests |  |

| Action | Call | Result handling |
|---|---|---|
| Delete account | `POST /users/me/erasure-requests` | 202 ErasureRequest → clear local data → AUTH-01 with "Your account is scheduled for deletion on {scheduled_for}." |

**Notes:** If the User has an individual Standard subscription, warn first and link to "Manage billing" (BIL-03) — erasure doesn't cancel Stripe billing by itself (see OPEN-QUESTIONS).


## D3 · My Apps

### APP-01 — Home — My Apps
**Domain:** D3 My Apps · **Route:** `/` · **Archetype:** grid · **Phase:** MVP  
**Who:** Signed in  
**Purpose:** See every app you can use, its access status, and launch it.

**iOS / iPadOS container:** Tab root `NavigationStack`, `LazyVGrid`

**Loads:** `GET /applications (catalog the caller can see; each has access_tier free|standard)` · `GET /users/me/entitlements?include_inactive=true` · `GET /users/me (plan, plan_sources)` · `GET /organizations?status=active (for pending invites)` · `GET /users/me/notifications/unread-count (bell badge)`

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Search | search | .searchable | client filter on name/description |  |
| Grid / List | segmented | Picker(.segmented) | client preference | persisted locally |
| Invitation banner | banner | inline List section | Organization.my_membership_status = pending | "{org} invited you…" → ORG-09 |
| Verify-email banner | banner | inline List section | User.email_verified = false | → AUTH-04 |
| Your apps | tile-grid | LazyVGrid 2–4 cols | join Application × resolved entitlement per app | tile = icon, name, StatusBadge, helper text (COPY); order: recently launched, then A–Z; pin to top (local) |
| Included with Standard | tile-grid | LazyVGrid 2–4 cols | standard apps with status none while User.plan = free | badge "Standard"; tap → upsell sheet → BIL-01 ("Get every app for $6/month") |
| Available | tile-grid | LazyVGrid 2–4 cols | other public apps with status none | "Get access" → app support/launch page |
| Inbox bell | toolbar button + count | .toolbar items | NotificationUnreadCount.unread_count | → NTF-01 |
| Past apps | row → APP-05 | NavigationLink row | count of revoked/expired |  |

| Action | Call | Result handling |
|---|---|---|
| Open tile (active) | `→ APP-03` | Launch |
| Open tile (other status) | `→ APP-02` | Explain status |
| Context menu / long-press |  | Open · App settings (SET-02) · App profile (APP-04) · Pin · Copy app ID |

**States:** Loading: skeleton after 300 ms; Error: inline with Retry; codes per COPY-AND-STATES; Empty: "No apps yet" (COPY); Offline: show cached grid read-only, launch disabled

**Notes:** Resolved status per app: if any path active → Active; else precedence scheduled > disabled > expired > revoked > none (matches the API's resolved_entitlement_status). Use `GET /users/me/apps/{appId}/effective-permissions` for the authoritative answer when the user opens APP-02. Tile helper text by source of the active path: `free` → "Free"; `subscription` → "Standard" (or "Standard · via {org}"); developer grants as before. The Hub itself (`app_hub`) is hidden from the grid.

### APP-02 — App info
**Domain:** D3 My Apps · **Route:** `/apps/:appId` · **Archetype:** popover · **Phase:** MVP  
**Who:** Signed in  
**Purpose:** Explain an app's access status and how the user has access.

**iOS / iPadOS container:** `.sheet` with `.presentationDetents([.medium, .large])`

**Loads:** `GET /applications/{appId}` · `GET /users/me/apps/{appId}/effective-permissions`

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Header | icon + name + StatusBadge | Image + Text + StatusBadge | Application, EffectivePermissions.entitlement_status |  |
| Description | text | Text | Application.description |  |
| Access paths | list | List | EffectivePermissions.access_paths[]: source, status, organization_id, member_decision | "Free", "Standard · your subscription", "Standard · via Northwind", "Personal · Granted by admin", "Via Northwind (seat)"; member_decision excluded explained |
| Tier | badge | StatusBadge (capsule Label) | Application.access_tier | "Free" or "Standard" |
| Your roles | chips | capsule Buttons row / .searchScopes | EffectivePermissions.roles | Phase 2 |
| Primary action | button | Button .bordered |  | Active → Open; none + standard tier + plan free → Upgrade to Standard (BIL-01); none+public → Get access; disabled/expired/revoked → Contact support (support_url) |
| Notification settings | row → SET-02 | NavigationLink row | resolved notifications.push for this app | "Push notifications: On/Off" |
| App settings / App profile | rows | List rows | → SET-02, APP-04 | only when active |

**States:** Loading: skeleton after 300 ms; Error: inline with Retry; codes per COPY-AND-STATES

### APP-03 — Launch app
**Domain:** D3 My Apps · **Route:** `/apps/:appId/launch` · **Archetype:** handoff · **Phase:** MVP  
**Who:** Active entitlement  
**Purpose:** Mint an app token (or authorization code) and open the app.

**iOS / iPadOS container:** `openURL` / `ASWebAuthenticationSession`

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Progress | inline spinner on tile | ProgressView on tile |  | ≤ 1 s target |
| Organization picker | menu | Menu | body.organization_id | only when the user has access via >1 org and the app needs org context |

| Action | Call | Result handling |
|---|---|---|
| Embedded launch | `POST /auth/app-tokens {application_id, organization_id?}` | 200 AppToken (5-min JWT) → open launch_url with token per app contract; never cache the token. 403 → refresh status, show APP-02. 409 → choose org. |
| Hosted launch (Phase 3) | `POST /auth/oauth/authorization-codes (PKCE S256)` | 201 → open redirect_uri?code=…&state=… in system browser / auth session |

**Notes:** Record launch time locally for "recently launched" ordering.

### APP-04 — App profile
**Domain:** D3 My Apps · **Route:** `/apps/:appId/profile` · **Archetype:** form · **Phase:** Phase 2  
**Who:** Self, active entitlement  
**Purpose:** Per-app display handle and custom profile fields.

**iOS / iPadOS container:** `.sheet` with `NavigationStack` + `Form` (Cancel / primary in toolbar)

**Loads:** `GET /users/me/apps/{appId}/profile`

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Display handle | text-field | TextField | AppProfile.display_handle | optional; shows effective_display_name as placeholder |
| Custom fields | key/value editor | List of key/value rows + Add | AppProfile.custom | only keys the app documents; null removes a key |

| Action | Call | Result handling |
|---|---|---|
| Save | `PATCH /users/me/apps/{appId}/profile (If-Match)` |  |

**States:** Loading: skeleton after 300 ms; Error: inline with Retry; codes per COPY-AND-STATES

### APP-05 — Past apps
**Domain:** D3 My Apps · **Route:** `/apps/past` · **Archetype:** list · **Phase:** MVP  
**Who:** Signed in  
**Purpose:** Apps whose access was revoked or expired.

**iOS / iPadOS container:** `List` in `NavigationStack` (iPad: split view)

**Loads:** `GET /users/me/entitlements?include_inactive=true&status=revoked,expired`

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Rows | list | List | Application name/icon, status, ended/revoked date, source | tap → APP-02 |

**States:** Loading: skeleton after 300 ms; Error: inline with Retry; codes per COPY-AND-STATES


## D4 · Settings

### SET-01 — Settings — Global
**Domain:** D4 Settings · **Route:** `/settings` · **Archetype:** settings · **Phase:** MVP  
**Who:** Self  
**Purpose:** Preferences that apply to every app.

**iOS / iPadOS container:** Inset-grouped `List`/`Form`

**Loads:** `GET /users/me/settings` · `GET /users/me/apps/app_hub/settings` (theme choices)

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Language | select | Picker (menu / navigationLink) | Settings.locale | BCP 47; default = OS language; changing re-localizes the UI live |
| Time zone | searchable select | navigationLink to searchable List | Settings.timezone | IANA; default = OS zone; "Use device time zone" option |
| Appearance | segmented | Picker(.segmented) | Settings.theme light/dark/system | Light · Dark · Match system (default). Applies instantly; Match system follows OS changes live. See THEMES.md |
| Light theme | swatch picker | swatch grid row in Form (radio group, `.isSelected` trait) | `app_hub` setting `appearance.light_theme` | 5 themes: Teal Ledger, Indigo Root, **Moss Paper (default)**, Coral Clay, Spore Pink. Each swatch previews background, sidebar, accent and text; selected swatch shows a check and accent ring; accessible name "{name} light theme, selected" |
| Dark theme | swatch picker | swatch grid row in Form (radio group, `.isSelected` trait) | `app_hub` setting `appearance.dark_theme` | 5 themes: Night Soil, Deep Teal, Graphite Spore, **Forest Floor (default)**, Ember Clay. Same behaviour as Light theme |
| Push notifications | toggle | Toggle | Settings.notifications.push (missing = on) | global master switch for push from every app; inbox still fills (NTF-01). Shows OS permission state beneath: "Allowed in system settings" / "Blocked — Open system settings" |
| Email notifications | toggle | Toggle | Settings.notifications.email |  |
| SMS notifications | toggle | Toggle | Settings.notifications.sms | disabled until Profile.contact_phone set |
| Share usage analytics | toggle | Toggle | client preference | default per platform policy; see CLIENT-ARCHITECTURE §10 |
| Scope list | list | List | Global + each app with active access | → SET-02 (desktop: middle column) |

| Action | Call | Result handling |
|---|---|---|
| Change any row | `PATCH /users/me/settings (If-Match)` | Optimistic; revert + toast on error. |
| Choose a light or dark theme | `PATCH /users/me/apps/app_hub/settings (If-Match)` with `appearance.light_theme` / `appearance.dark_theme` | Apply instantly, cache locally, then sync; 409 → refetch and re-apply; 422 → revert + toast "Couldn't save your theme. Try again." |

**States:** Loading: skeleton after 300 ms; Error: inline with Retry; codes per COPY-AND-STATES

### SET-02 — Settings — Per app
**Domain:** D4 Settings · **Route:** `/settings/apps/:appId` · **Archetype:** settings · **Phase:** Phase 2  
**Who:** Self, active entitlement  
**Purpose:** Override global settings and set app-specific preferences from the app's settings_schema.

**iOS / iPadOS container:** Inset-grouped `List`/`Form`

**Loads:** `GET /applications/{appId} (settings_schema)` · `GET /users/me/apps/{appId}/settings`

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Schema form | generated rows | SchemaForm | settings_schema properties × AppSettingsResolved.resolved | string+enum → select; string → text; string date-time → date-time picker; boolean → toggle; integer/number with min/max → stepper/number; object/array → read-only JSON preview |
| Source caption | caption per row | Text .footnote | AppSettingsResolved.sources[key] | app_override → "Changed for this app · Reset"; global → "Using your global setting"; app_default → "App default" |
| Reset | link per overridden row | NavigationLink row | PATCH overrides {key:null} |  |
| Push notifications for this app | toggle | Toggle | resolved notifications.push; override via overrides.notifications.push | source caption as other rows; Reset clears the override |
| Stale overrides | banner | inline List section | AppSettingsResolved.stale_overrides | "Some saved settings no longer apply" → Clear |

| Action | Call | Result handling |
|---|---|---|
| Change row | `PATCH /users/me/apps/{appId}/settings {overrides:{key:value}} (If-Match)` | 422 → field error from details (enum_mismatch, out_of_range, undeclared_key). |

**States:** Loading: skeleton after 300 ms; Error: inline with Retry; codes per COPY-AND-STATES


## D5 · Organizations

### ORG-01 — Organizations
**Domain:** D5 Organizations · **Route:** `/orgs` · **Archetype:** list · **Phase:** MVP  
**Who:** Signed in (all orgs with organizations.manage)  
**Purpose:** Organizations you belong to, plus pending invitations.

**iOS / iPadOS container:** `List` in `NavigationStack` (iPad: split view)

**Loads:** `GET /organizations`

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Pending invitations | section | Section | my_membership_status = pending | Accept / Decline inline → ORG-09 |
| Rows | list | List | Organization: name, member_count, my_role, status | suspended orgs show badge |
| Search | search | .searchable | q | server-side when > 50 |
| New organization | primary-button | Button .buttonStyle(.glassProminent/.borderedProminent) | → ORG-02 |  |

**States:** Loading: skeleton after 300 ms; Error: inline with Retry; codes per COPY-AND-STATES

### ORG-02 — New organization
**Domain:** D5 Organizations · **Route:** `/orgs/new` · **Archetype:** form · **Phase:** MVP  
**Who:** Any active user  
**Purpose:** Create an organization; creator becomes org admin.

**iOS / iPadOS container:** `.sheet` with `NavigationStack` + `Form` (Cancel / primary in toolbar)

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Name | text-field | TextField | body.name | required; ≤ 100 |

| Action | Call | Result handling |
|---|---|---|
| Create | `POST /organizations (Idempotency-Key)` | 201 → ORG-03. 429 → rate limited. |

### ORG-03 — Organization — Members
**Domain:** D5 Organizations · **Route:** `/orgs/:orgId/members` · **Archetype:** detail · **Phase:** MVP  
**Who:** Member (read), org_admin / organizations.manage (write)  
**Purpose:** See and manage members and pending invitations.

**iOS / iPadOS container:** Pushed view; segmented `Picker`; toolbar actions

**Loads:** `GET /organizations/{id}` · `GET /organizations/{id}/members`

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Header | avatar + name + id-chip + status | Text(.monospaced) + copy | Organization | my_role shown as caption |
| Tabs | tabs | Picker(.segmented) | Members · Apps & seats · Subscription · Activity · Settings | Subscription tab = BIL-02 (org_admin / billing.manage only) |
| Filter | chips | capsule Buttons row / .searchScopes | role, membership_status |  |
| Rows | list | List | OrganizationMember: display_name/email, role, membership_status, joined_at/invited_at | pending rows show Invited badge + invited date |
| Invite | primary-button | Button .buttonStyle(.glassProminent/.borderedProminent) | → ORG-04 | org_admin only |

| Action | Call | Result handling |
|---|---|---|
| Row menu | `→ ORG-05` | Change role · Remove · Withdraw invitation |

**States:** Loading: skeleton after 300 ms; Error: inline with Retry; codes per COPY-AND-STATES

### ORG-04 — Invite member
**Domain:** D5 Organizations · **Route:** `/orgs/:orgId/members/invite` · **Archetype:** form · **Phase:** MVP  
**Who:** org_admin; organizations.manage may add by user_id  
**Purpose:** Invite people by email (or add an existing user, staff only).

**iOS / iPadOS container:** `.sheet` with `NavigationStack` + `Form` (Cancel / primary in toolbar)

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Emails | token-field | custom token field | body.email (one request per address) | paste a list separated by commas/newlines |
| Role | radio-group | Picker(.inline) | body.role org_admin \| member | default member |
| Add existing user | user-picker | searchable sheet | body.user_id | organizations.manage only |

| Action | Call | Result handling |
|---|---|---|
| Send invitations | `POST /organizations/{id}/members × n` | Per-address results summary; 409 → "already a member" per address. Responses look identical whether or not the address has an account (by design). |

**Notes:** Existing Users also receive an `account.organization_invitation` platform alert (NTF-01, and a push where the Hub has a device).

### ORG-05 — Member actions
**Domain:** D5 Organizations · **Route:** `/orgs/:orgId/members/:userId` · **Archetype:** confirm · **Phase:** MVP  
**Who:** org_admin / organizations.manage  
**Purpose:** Change a member's role or remove them.

**iOS / iPadOS container:** `.alert` (with text field) or `.confirmationDialog`

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Role | radio-group | Picker(.inline) | body.role |  |
| Remove from organization | destructive-button | Button(role:.destructive) | DELETE …/members/{userId} | COPY confirm |

| Action | Call | Result handling |
|---|---|---|
| Change role | `PATCH /organizations/{id}/members/{userId}` | 409 → last admin can't be demoted (explain). |
| Remove | `DELETE /organizations/{id}/members/{userId}` | 409 → last admin can't leave/remove. |

### ORG-06 — Organization — Apps & seats
**Domain:** D5 Organizations · **Route:** `/orgs/:orgId/apps` · **Archetype:** tab · **Phase:** Phase 2  
**Who:** Member (read), org_admin (write)  
**Purpose:** Org-wide app grants and who they include.

**iOS / iPadOS container:** Segment content

**Loads:** `GET /organizations/{id}/entitlements`

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Rows | list | List | Entitlement (org): app, status, source (org_seat \| subscription), member_scope, included_member_count, starts_at/ends_at | AccessSwitch for status (org_admin with rights); `subscription` rows read "Included with the organization's Standard subscription" and their scope is edited in BIL-02 |
| Grant app | button | Button .bordered | → ORG-07 |  |

| Action | Call | Result handling |
|---|---|---|
| Toggle | `PATCH /entitlements/{id} {status}` | Off requires confirm (COPY). On platform-managed rows only active ⇄ disabled is allowed; anything else 409 entitlement_platform_managed. |
| Edit scope | `→ ORG-07 (org_seat) / BIL-02 (subscription)` |  |

**States:** Loading: skeleton after 300 ms; Error: inline with Retry; codes per COPY-AND-STATES

### ORG-07 — Grant app to organization / member scope
**Domain:** D5 Organizations · **Route:** `/orgs/:orgId/apps/grant` · **Archetype:** form · **Phase:** Phase 2  
**Who:** entitlements.manage or app owner; scope edits by org_admin  
**Purpose:** Create an org-wide grant or change who it includes.

**iOS / iPadOS container:** `.sheet` with `NavigationStack` + `Form` (Cancel / primary in toolbar)

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| App | select | Picker (menu / navigationLink) | body.application_id | create only |
| Who gets access | radio-group | Picker(.inline) | member_scope all_members \| allowlist \| denylist | COPY labels |
| Members | member-picker | searchable sheet | member_overrides | visible for allowlist/denylist |
| Starts / Ends | date-fields | DatePicker(.compact) | starts_at, ends_at | optional |
| Order ID | text-field | TextField | order_id | optional; staff/app owner |

| Action | Call | Result handling |
|---|---|---|
| Grant | `POST /organizations/{id}/entitlements (Idempotency-Key)` | 409 entitlement_already_exists → already granted (view it). 422 → field errors. |
| Save scope | `PATCH /entitlements/{id} {member_scope, member_overrides}` |  |

### ORG-08 — Organization — Settings
**Domain:** D5 Organizations · **Route:** `/orgs/:orgId/settings` · **Archetype:** settings · **Phase:** MVP  
**Who:** org_admin (rename), organizations.manage (suspend), any member (leave)  
**Purpose:** Rename, leave, or (staff) suspend the organization.

**iOS / iPadOS container:** Inset-grouped `List`/`Form`

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Name | text-field | TextField | Organization.name |  |
| Leave organization | destructive-row | Button(role:.destructive) row | DELETE …/members/me |  |
| Suspend / Reactivate | destructive-row | Button(role:.destructive) row | PATCH {status, reason} | staff only; reason required |

| Action | Call | Result handling |
|---|---|---|
| Rename | `PATCH /organizations/{id} {name}` |  |
| Leave | `DELETE /organizations/{id}/members/me` | 409 last admin |

### ORG-09 — Invitation
**Domain:** D5 Organizations · **Route:** `/orgs/:orgId/invitation` · **Archetype:** confirm · **Phase:** MVP  
**Who:** Invited user  
**Purpose:** Accept or decline an organization invitation.

**iOS / iPadOS container:** `.alert` (with text field) or `.confirmationDialog`

**Entry points:** Invitation banner on APP-01; ORG-01 pending section; `account.organization_invitation` notification (NTF-01 / push action "Accept")

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Org name + inviter | text | Text | Organization |  |
| Accept | primary-button | Button .buttonStyle(.glassProminent/.borderedProminent) |  |  |
| Decline | button | Button .bordered |  |  |

| Action | Call | Result handling |
|---|---|---|
| Accept | `POST /organizations/{id}/members/me/accept` | 200 → ORG-03; apps granted via org (and the org's Standard subscription, if it covers the member) appear on APP-01. Mark the related notification read. |
| Decline | `DELETE /organizations/{id}/members/me` |  |


## D6 · Users

### USR-01 — Users
**Domain:** D6 Users · **Route:** `/admin/users` · **Archetype:** table · **Phase:** MVP  
**Who:** users.list (write actions need users.manage)  
**Purpose:** Find and manage user accounts.

**iOS / iPadOS container:** `List` (iPad regular width: `Table`)

**Loads:** `GET /users?limit=50&cursor&status&q&application_id&organization_id&created_after&created_before`

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Search | search | .searchable | q (name, email, ID prefix usr_) | debounced 300 ms; exact ID jumps to USR-03 |
| Status filter | chips | capsule Buttons row / .searchScopes | status active \| invited \| suspended \| deleted | deleted visible to users.manage only |
| More filters | menu | Menu | application_id, organization_id, created range |  |
| Columns | table | List (iPad: Table) | display name + email, status, id, last_login_at, created_at | sortable client-side within loaded page; server order = created_at desc |
| Multi-select | selection | Picker (menu / navigationLink) |  | bulk: Suspend, Resend invitation, Export CSV (client-side of loaded rows) |
| Invite user | primary-button | Button .buttonStyle(.glassProminent/.borderedProminent) | → USR-02 | users.manage |

| Action | Call | Result handling |
|---|---|---|
| Open row | `→ USR-03` |  |
| Row menu |  | Suspend · Reactivate · Resend invitation · Copy ID · Copy email |

**States:** Loading: skeleton after 300 ms; Error: inline with Retry; codes per COPY-AND-STATES

### USR-02 — Invite user
**Domain:** D6 Users · **Route:** `/admin/users/new` · **Archetype:** form · **Phase:** MVP  
**Who:** users.manage  
**Purpose:** Create a user and send an invitation (or create active).

**iOS / iPadOS container:** `.sheet` with `NavigationStack` + `Form` (Cancel / primary in toolbar)

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Email | email-field | TextField .textContentType(.username) .keyboardType(.emailAddress) | body.email | required |
| Display name | text-field | TextField | body.display_name | optional |
| Send invitation email | toggle | Toggle | body.send_invitation | default on |
| Create as active | toggle | Toggle | body.status active\|invited | default off (invited) |

| Action | Call | Result handling |
|---|---|---|
| Invite | `POST /users (Idempotency-Key)` | 201 → USR-03. 409 → "A user with that email exists" + link. |

### USR-03 — User — Overview
**Domain:** D6 Users · **Route:** `/admin/users/:userId` · **Archetype:** detail · **Phase:** MVP  
**Who:** users.list (read), users.manage (write)  
**Purpose:** One user's account state and administrative actions.

**iOS / iPadOS container:** Pushed view; segmented `Picker`; toolbar actions

**Loads:** `GET /users/{id}` · `GET /users/{id}/profile` · `GET /users/{id}/subscription (billing.manage only)`

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Header | avatar, display name, email, id-chip, StatusBadge | Text(.monospaced) + copy | User + Profile |  |
| Tabs | tabs | Picker(.segmented) | Overview · Access · Roles · Sessions · Notifications · Activity | Activity = AUD-01 filtered target_user_id; Notifications = NTF-05 |
| Facts | key-value list | LabeledContent rows | email_verified, pending_email, mfa_enabled, plan, plan_sources, created_at, last_login_at, signup_application_id, test_mode | plan_sources link to the org or to BIL-01 subscription view (billing.manage) |
| Actions | toolbar/menu | Menu |  | Suspend (USR-05) · Reactivate · Resend invitation · Reset two-factor · Export data · Delete · Restore · Cancel scheduled erasure |

| Action | Call | Result handling |
|---|---|---|
| Reactivate / Restore | `PATCH /users/{id} {status:active} (If-Match)` |  |
| Resend invitation | `POST /users/{id}/invitation` | 202 toast; 409 → not in invited state |
| Reset two-factor | `DELETE /users/{id}/mfa/totp` | confirm |
| Export | `GET /users/{id}/export` | save file |
| Delete | `DELETE /users/{id}` | confirm |
| Cancel erasure | `DELETE /users/{id}/erasure-requests/current` | visible during 7-day window |

**States:** Loading: skeleton after 300 ms; Error: inline with Retry; codes per COPY-AND-STATES

### USR-04 — User — Sessions
**Domain:** D6 Users · **Route:** `/admin/users/:userId/sessions` · **Archetype:** tab · **Phase:** MVP  
**Who:** users.manage  
**Purpose:** See and revoke a user's sessions.

**iOS / iPadOS container:** Segment content

**Loads:** `GET /users/{id}/sessions`

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Rows | list | List | Session: user_agent, ip_address, amr, created_at, last_seen_at, expires_at | staff see full IP |

| Action | Call | Result handling |
|---|---|---|
| Revoke | `DELETE /users/{id}/sessions/{sessionId}` |  |

**States:** Loading: skeleton after 300 ms; Error: inline with Retry; codes per COPY-AND-STATES

### USR-05 — Suspend user
**Domain:** D6 Users · **Route:** `/admin/users/:userId/suspend` · **Archetype:** confirm · **Phase:** MVP  
**Who:** users.manage  
**Purpose:** Suspend an account (signs them out everywhere).

**iOS / iPadOS container:** `.alert` (with text field) or `.confirmationDialog`

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Reason | text-area | TextField(axis:.vertical) | body.reason | optional, written to audit |

| Action | Call | Result handling |
|---|---|---|
| Suspend | `POST /users/{id}/suspend` | 200 → badge Suspended. 409 → already suspended. |

**Notes:** Suspension revokes sessions and deactivates every push device the user holds; say so in the confirm body.


## D7 · Access

### ACX-01 — User — Access
**Domain:** D7 Access · **Route:** `/admin/users/:userId/access` · **Archetype:** tab · **Phase:** MVP  
**Who:** users.list (read), entitlements.manage or app owner (write)  
**Purpose:** Turn apps on or off for one person; see every path that grants access.

**iOS / iPadOS container:** Segment content

**Loads:** `GET /users/{id}/entitlements?include_inactive=true` · `GET /applications`

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Rows (one per app) | list | List | resolved status per app + paths (personal Entitlement, org grants) | StatusBadge + AccessSwitch on the personal path |
| Paths disclosure | expandable | DisclosureGroup | Entitlement.granted_via, source, subscription_id, organization_id, member_decision, starts_at/ends_at, disabled_reason | platform-managed paths (`free`, `subscription`) show a "Managed by Substratal" caption; developer grants and platform rows can coexist |
| Org-only access | caption + link | Text .footnote |  | switch replaced by "Via {org}" → ORG-06 |
| Grant access | primary-button | Button .buttonStyle(.glassProminent/.borderedProminent) | → ACX-02 |  |
| Why? | button | Button .bordered | → ACX-04 |  |

| Action | Call | Result handling |
|---|---|---|
| Switch off | `→ ACX-03 confirm → PATCH /entitlements/{id} {status:disabled, disabled_reason}` | Optimistic with Undo (PATCH status:active) for 10 s |
| Switch on | `PATCH /entitlements/{id} {status:active}` | No confirm |
| Remove grant made in error | `DELETE /entitlements/{id}` | Only developer grants with no order_id and < 24 h old; never on `free`/`subscription` rows (409 entitlement_platform_managed); 409 otherwise |

**States:** Loading: skeleton after 300 ms; Error: inline with Retry; codes per COPY-AND-STATES

**Notes:** The switch on a platform-managed row only disables / re-enables it (the platform never re-enables a row an admin disabled). Term, source and scope edits are hidden for those rows.

### ACX-02 — Grant access
**Domain:** D7 Access · **Route:** `/admin/users/:userId/access/grant` · **Archetype:** form · **Phase:** MVP  
**Who:** entitlements.manage or app owner (own app)  
**Purpose:** Give a person access to an app.

**iOS / iPadOS container:** `.sheet` with `NavigationStack` + `Form` (Cancel / primary in toolbar)

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| App | select | Picker (menu / navigationLink) | body.application_id | app owners see only their apps |
| Type | radio-group | Picker(.inline) | body.source purchase \| trial \| admin_grant | `free` and `subscription` are never offered — the platform grants them |
| Order ID | text-field | TextField | body.order_id | required feel for purchase |
| Starts | date-time | DatePicker(.compact) | body.starts_at | default now |
| Ends | date-time | DatePicker(.compact) | body.ends_at | optional; required for trial |

| Action | Call | Result handling |
|---|---|---|
| Grant | `POST /users/{id}/entitlements (Idempotency-Key, required)` | 201 → row Active. 409 entitlement_already_exists → "Already has access — view it". A platform-managed row doesn't block a grant (they coexist). |

### ACX-03 — Turn off / edit access
**Domain:** D7 Access · **Route:** `/admin/entitlements/:entId` · **Archetype:** confirm · **Phase:** MVP  
**Who:** entitlements.manage or app owner  
**Purpose:** Confirm turning off access, or edit term/source.

**iOS / iPadOS container:** `.alert` (with text field) or `.confirmationDialog`

**Loads:** `GET /entitlements/{id}`

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Title | text | Text |  | "Turn off {app} for {name}?" |
| Reason | text-field | TextField | body.disabled_reason | optional; audit |
| Edit term (advanced) | disclosure | DisclosureGroup | ends_at, source trial→purchase, order_id | hidden for platform-managed rows |

| Action | Call | Result handling |
|---|---|---|
| Turn off | `PATCH /entitlements/{id} {status:disabled, disabled_reason} (If-Match)` | 409 version_conflict → reload and re-apply |
| Save changes | `PATCH /entitlements/{id}` |  |

**States:** Loading: skeleton after 300 ms; Error: inline with Retry; codes per COPY-AND-STATES

### ACX-04 — Access check (Why?)
**Domain:** D7 Access · **Route:** `/admin/users/:userId/apps/:appId/why` · **Archetype:** popover · **Phase:** MVP  
**Who:** users.list or app owner  
**Purpose:** Explain exactly why a person can or can't use an app.

**iOS / iPadOS container:** `.sheet` with `.presentationDetents([.medium, .large])`

**Loads:** `GET /users/{id}/apps/{appId}/effective-permissions`

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Verdict | banner | inline List section | allowed | "Dana can use Timetrack" / "can't" + first failing reason (user_status, entitlement_status) |
| Checks | checklist | List with status symbols | user_status, entitlement_status, access_paths[], roles, effective_permissions | each with pass/fail icon |
| Computed at | caption | Text .footnote | computed_at |  |

**States:** Loading: skeleton after 300 ms; Error: inline with Retry; codes per COPY-AND-STATES

### ACX-05 — Access (all)
**Domain:** D7 Access · **Route:** `/admin/access` · **Archetype:** table · **Phase:** MVP  
**Who:** entitlements.manage or app owner (confined to own apps)  
**Purpose:** Every grant across users: audit expirations, trials, revocations.

**iOS / iPadOS container:** `List` (iPad regular width: `Table`)

**Loads:** `GET /entitlements?application_id&status&source&user_id&organization_id&order_id&ends_before&include_inactive`

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Filters | chips/menus | capsule Buttons row / .searchScopes | app, status, source, ends before (Expiring in 7/30 days presets) |  |
| Columns | table | List (iPad: Table) | user, app, status, source (incl. free / subscription), subscription_id, starts_at, ends_at, order_id, granted_via | source filter defaults to developer grants; "Show platform-managed" chip adds free/subscription rows |
| Multi-select | selection | Picker (menu / navigationLink) |  | bulk Turn off (confirm, one PATCH per row, progress + per-row result) |

| Action | Call | Result handling |
|---|---|---|
| Open row | `→ ACX-01 for that user` |  |

**States:** Loading: skeleton after 300 ms; Error: inline with Retry; codes per COPY-AND-STATES


## D8 · Roles & Permissions

### ROL-01 — Roles
**Domain:** D8 Roles & Permissions · **Route:** `/admin/roles` · **Archetype:** list · **Phase:** MVP  
**Who:** users.list (read), roles.manage (write)  
**Purpose:** Platform and app roles.

**iOS / iPadOS container:** `List` in `NavigationStack` (iPad: split view)

**Loads:** `GET /roles?scope` · `GET /permissions (includes notifications.send)`

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Scope | segmented | Picker(.segmented) | scope platform \| {application_id} | app picker when App |
| Rows | list | List | Role: name, scope, seed, assignment_count, description | seed → lock icon |
| New role | primary-button | Button .buttonStyle(.glassProminent/.borderedProminent) | → ROL-03 | roles.manage |

**States:** Loading: skeleton after 300 ms; Error: inline with Retry; codes per COPY-AND-STATES

### ROL-02 — Role detail
**Domain:** D8 Roles & Permissions · **Route:** `/admin/roles/:roleId` · **Archetype:** detail · **Phase:** MVP  
**Who:** users.list (read), roles.manage (write)  
**Purpose:** View or edit a role's permissions.

**iOS / iPadOS container:** Pushed view; segmented `Picker`; toolbar actions

**Loads:** `GET /roles/{id}` · `GET /permissions?scope`

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Header | name, id-chip, scope, assignment_count | Text(.monospaced) + copy |  |  |
| Description | text-field | TextField | Role.description | read-only when seed |
| Permissions | checkbox-list grouped by prefix | List with checkmark rows, Sections | Role.permissions ⊂ Permission[].key | read-only when seed; show Permission.description |
| Holders | link | Button .borderless | → USR-01 filtered | Phase 2 if API adds filter; else hidden |
| Delete role | destructive-button | Button(role:.destructive) | DELETE /roles/{id} | only non-seed with assignment_count 0 |

| Action | Call | Result handling |
|---|---|---|
| Save | `PATCH /roles/{id} {description, permissions} (If-Match)` |  |
| Delete | `DELETE /roles/{id}` | 409 → still assigned |

**States:** Loading: skeleton after 300 ms; Error: inline with Retry; codes per COPY-AND-STATES

### ROL-03 — New role
**Domain:** D8 Roles & Permissions · **Route:** `/admin/roles/new` · **Archetype:** form · **Phase:** MVP  
**Who:** roles.manage  
**Purpose:** Define a custom role.

**iOS / iPadOS container:** `.sheet` with `NavigationStack` + `Form` (Cancel / primary in toolbar)

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Name | text-field | TextField | body.name | ^[a-z][a-z0-9_]{1,40}$; live validation |
| Scope | select | Picker (menu / navigationLink) | body.scope | platform or an application |
| Description | text-area | TextField(axis:.vertical) | body.description |  |
| Permissions | checkbox-list | List with checkmark rows, Sections | body.permissions | ≥ 1 |

| Action | Call | Result handling |
|---|---|---|
| Create | `POST /roles` | 409 → name taken in scope |

### ROL-04 — User — Roles
**Domain:** D8 Roles & Permissions · **Route:** `/admin/users/:userId/roles` · **Archetype:** tab · **Phase:** MVP  
**Who:** users.list (read), roles.manage (write)  
**Purpose:** Assign and remove roles for a user.

**iOS / iPadOS container:** Segment content

**Loads:** `GET /users/{id}/roles`

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Rows | list | List | UserRoleAssignment: role, application_id, assigned_at, assigned_by, implicit | implicit → "Granted with app access", not removable |
| Assign role | button → role picker | searchable sheet |  |  |

| Action | Call | Result handling |
|---|---|---|
| Assign | `POST /users/{id}/roles/{roleId}` | idempotent; 200 already held |
| Remove | `DELETE /users/{id}/roles/{roleId}` | 409 → implicit role |

**States:** Loading: skeleton after 300 ms; Error: inline with Retry; codes per COPY-AND-STATES


## D9 · Applications

### APL-01 — Applications
**Domain:** D9 Applications · **Route:** `/dev/apps` · **Archetype:** list · **Phase:** MVP  
**Who:** Any (catalog), owner/applications.manage (manage)  
**Purpose:** Applications you own or administer.

**iOS / iPadOS container:** `List` in `NavigationStack` (iPad: split view)

**Loads:** `GET /applications?owned=true (staff: all, filter review_status)`

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Rows | list | List | ApplicationSummary: icon, name, slug, visibility, review_status |  |
| Register app | primary-button | Button .buttonStyle(.glassProminent/.borderedProminent) | → APL-02 | applications.manage (Phase 3: self-service) |

**States:** Loading: skeleton after 300 ms; Error: inline with Retry; codes per COPY-AND-STATES

### APL-02 — Register application
**Domain:** D9 Applications · **Route:** `/dev/apps/new` · **Archetype:** wizard · **Phase:** MVP  
**Who:** applications.manage  
**Purpose:** Register a new application.

**iOS / iPadOS container:** Sheet with `NavigationStack` push per step

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Name | text-field | TextField | name |  |
| Slug | text-field | TextField | slug | ^[a-z][a-z0-9-]{1,63}$; 'platform' reserved; immutable — warn |
| Launch URL | url-field | TextField .keyboardType(.URL) | launch_url | https |
| Redirect URIs | url-list | List with .onDelete + Add row | redirect_uris |  |
| Visibility | radio-group | Picker(.inline) | visibility public \| invite_only \| internal |  |
| Access tier | radio-group | Picker(.inline) | access_tier free \| standard (default standard) | applications.manage only; explains who gets access |
| Owner | select | Picker (menu / navigationLink) | owner_user_id \| owner_organization_id |  |
| Icon | image-picker | PhotosPicker + camera | icon_url |  |
| Support URL | url-field | TextField .keyboardType(.URL) | support_url |  |
| Email from name | text-field | TextField | email_from_name |  |

| Action | Call | Result handling |
|---|---|---|
| Register | `POST /applications (Idempotency-Key)` | 409 slug taken; 422 field errors. Registration grants the platform-managed Entitlements its access_tier calls for. |

### APL-03 — Application — Overview
**Domain:** D9 Applications · **Route:** `/dev/apps/:appId` · **Archetype:** detail · **Phase:** MVP  
**Who:** Owner (configure), applications.manage (anything)  
**Purpose:** Configure an application.

**iOS / iPadOS container:** Pushed view; segmented `Picker`; toolbar actions

**Loads:** `GET /applications/{id}`

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Header | icon, name, id-chip, review StatusBadge | Text(.monospaced) + copy |  |  |
| Tabs | tabs | Picker(.segmented) | Overview · Settings schema · Permissions & roles · Push · Keys · Webhooks · Access · Activity | Push = APL-07; Keys/Webhooks/Access = KEY-01/WHK-01/ACX-05 filtered to app |
| Fields | settings rows | Form rows | name, description, launch_url, redirect_uris, support_url, email_from_name, icon_url (owner); visibility, access_tier (applications.manage) | slug read-only with lock |
| Platform alerts | toggle | Toggle | receives_platform_notifications | owner-writable; explain that security/account alerts will push under this app's name and icon |
| Review notes | banner | inline List section | review_status rejected/suspended + review_notes |  |

| Action | Call | Result handling |
|---|---|---|
| Save field | `PATCH /applications/{id} (If-Match)` | 422 → field errors |

**States:** Loading: skeleton after 300 ms; Error: inline with Retry; codes per COPY-AND-STATES

### APL-04 — Settings schema
**Domain:** D9 Applications · **Route:** `/dev/apps/:appId/schema` · **Archetype:** tab · **Phase:** Phase 2  
**Who:** Owner / applications.manage  
**Purpose:** Declare per-user settings the app stores in Substratal.

**iOS / iPadOS container:** Segment content

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Properties | list | List | settings_schema.properties: key, type, enum, default, min/max, maxLength | add/edit via form |
| JSON editor | code-editor | TextEditor (iPad + keyboard) | settings_schema | JSON validation; diff before save |
| Preview | rendered SET-02 form | SchemaForm preview |  | shows exactly what users will see |
| Stats | caption | Text .footnote | settings_schema_stats |  |

| Action | Call | Result handling |
|---|---|---|
| Save | `PATCH /applications/{id} {settings_schema}` | 422 → schema rule errors |

### APL-05 — Permissions & roles
**Domain:** D9 Applications · **Route:** `/dev/apps/:appId/roles` · **Archetype:** tab · **Phase:** Phase 2  
**Who:** Owner / applications.manage  
**Purpose:** Declare app permissions, available app roles and the default role.

**iOS / iPadOS container:** Segment content

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Permissions | editable list | List (iPad: Table) | permissions[] {key, description} | key app.<slug>.* |
| Available app roles | token list | custom token field | available_app_roles (≤ 20) |  |
| Default app role | select | Picker (menu / navigationLink) | default_app_role | held implicitly by everyone with access |

| Action | Call | Result handling |
|---|---|---|
| Save | `PATCH /applications/{id}` |  |

### APL-06 — Review
**Domain:** D9 Applications · **Route:** `/dev/apps/:appId/review` · **Archetype:** form · **Phase:** Phase 3  
**Who:** applications.manage  
**Purpose:** Staff moderation of an application.

**iOS / iPadOS container:** `.sheet` with `NavigationStack` + `Form` (Cancel / primary in toolbar)

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Review status | radio-group | Picker(.inline) | review_status |  |
| Notes | text-area | TextField(axis:.vertical) | review_notes | visible to owner |

| Action | Call | Result handling |
|---|---|---|
| Save | `PATCH /applications/{id} {review_status, review_notes}` |  |

### APL-07 — Push credentials
**Domain:** D9 Applications · **Route:** `/dev/apps/:appId/push` · **Archetype:** tab · **Phase:** Phase 2  
**Who:** Owner or applications.manage with a User token (write); app key / owner (read)  
**Purpose:** Configure APNs, FCM and Web Push for an Application so it can send push notifications.

**iOS / iPadOS container:** Segment content

**Loads:** `GET /applications/{id}/push-credentials`

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Providers | list | List | PushCredential per provider apns \| fcm \| web_push: status active\|failed, failure_reason, failed_at, last_used_at; apns team_id/key_id/topic; fcm project_id/client_email; web_push subject + vapid_public_key | one row per provider with Add / Edit / Remove; failed → warning banner with reason and Fix |
| Add APNs | form | sheet with Form | POST body: provider apns, team_id (10), key_id (10), topic (bundle id), private_key (.p8) | file picker / drop / paste for the .p8; key never shown again |
| Add FCM | form | sheet with Form | POST body: provider fcm, service_account (JSON file) | drop/pick the downloaded JSON; validate shape client-side |
| Add Web Push | form | sheet with Form | POST body: provider web_push, subject (mailto:/https:) | platform generates VAPID keys; show vapid_public_key with Copy for the web client |
| Rotate / fix | form | sheet with Form | PATCH /applications/{id}/push-credentials/{provider} {key material \| status: active} | web_push can't rotate in place — explain delete + recreate |
| Remove | destructive-button | Button(role:.destructive) | DELETE /applications/{id}/push-credentials/{provider} | confirm; web_push removal deactivates every browser subscription |
| Platform alerts | toggle | Toggle | Application.receives_platform_notifications | mirror of APL-03 toggle |

| Action | Call | Result handling |
|---|---|---|
| Add | `POST /applications/{id}/push-credentials` | 201; 409 push_credential_exists → Edit; 422 invalid_push_credential → field error; 400 user_token_required never happens in the Hub (User token) |
| Fetch one | `GET /applications/{id}/push-credentials/{provider}` | on Edit |
| Save | `PATCH /applications/{id}/push-credentials/{provider}` | 409 invalid_status_transition |
| Remove | `DELETE /applications/{id}/push-credentials/{provider}` | 204 |

**States:** Loading: skeleton after 300 ms; Error: inline with Retry; codes per COPY-AND-STATES

**Notes:** To prove a credential end to end, the owner registers a device of their own app and uses its device test (NTF-03 lists only devices the owner holds).


## D10 · Developer Credentials

### KEY-01 — API keys
**Domain:** D10 Developer Credentials · **Route:** `/dev/keys` · **Archetype:** table · **Phase:** MVP  
**Who:** api_keys.manage or app owner  
**Purpose:** Service and agent credentials.

**iOS / iPadOS container:** `List` (iPad regular width: `Table`)

**Loads:** `GET /api-keys?scope&mode&intended_use&include_inactive`

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Filters | chips | capsule Buttons row / .searchScopes | mode live\|test, intended_use, include revoked | TEST pill on test keys |
| Columns | table | List (iPad: Table) | name, scope, secret_hint, mode, intended_use, restrict_destructive, last_used_at, expires_at, status(revoked_at) |  |
| Create key | primary-button | Button .buttonStyle(.glassProminent/.borderedProminent) | → KEY-02 |  |

**States:** Loading: skeleton after 300 ms; Error: inline with Retry; codes per COPY-AND-STATES

### KEY-02 — Create API key
**Domain:** D10 Developer Credentials · **Route:** `/dev/keys/new` · **Archetype:** form · **Phase:** MVP  
**Who:** api_keys.manage or app owner (app-scoped)  
**Purpose:** Create a key with least-privilege permissions.

**iOS / iPadOS container:** `.sheet` with `NavigationStack` + `Form` (Cancel / primary in toolbar)

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Name | text-field | TextField | name |  |
| Scope | select | Picker (menu / navigationLink) | scope platform \| application | owners: own apps only |
| Mode | segmented | Picker(.segmented) | mode live \| test | fixed after creation — say so |
| Used by | radio-group | Picker(.inline) | intended_use service \| agent | agent defaults restrict_destructive on |
| Block destructive operations | toggle | Toggle | restrict_destructive |  |
| Permissions | checkbox-list | List with checkmark rows, Sections | permissions | filtered by scope via GET /permissions; app-scoped keys offer the five app-confined platform permissions incl. notifications.send |
| Expires | date-field | DatePicker(.compact) | expires_at | optional; presets 30/90/365 days |

| Action | Call | Result handling |
|---|---|---|
| Create | `POST /api-keys` | 201 ApiKeyWithSecret → KEY-03 |

### KEY-03 — Secret reveal
**Domain:** D10 Developer Credentials · **Route:** `(modal)` · **Archetype:** secret · **Phase:** MVP  
**Who:** Creator  
**Purpose:** Show a new API key secret, webhook signing secret, or recovery codes exactly once.

**iOS / iPadOS container:** Sheet, `interactiveDismissDisabled`, hide on screenshot

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Secret | monospace selectable text | Text(.monospaced).textSelection(.enabled) | secret | never logged, never persisted, excluded from screenshots where the OS allows |
| Copy | button | Button .bordered | clipboard (sensitive) | auto-clear after 60 s where supported |
| Acknowledge | checkbox | Toggle (checkmark style) | client | gates Done; modal not dismissible until checked |

### KEY-04 — API key detail
**Domain:** D10 Developer Credentials · **Route:** `/dev/keys/:keyId` · **Archetype:** detail · **Phase:** MVP  
**Who:** api_keys.manage or app owner  
**Purpose:** Edit, rotate or revoke a key.

**iOS / iPadOS container:** Pushed view; segmented `Picker`; toolbar actions

**Loads:** `GET /api-keys/{id}` · `GET /audit-events?actor_id={id}`

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Fields | settings rows | Form rows | name, permissions, restrict_destructive (editable); scope, mode, intended_use, expires_at, created_by, last_used_at (read-only) |  |
| Rotate | button | Button .bordered | POST /api-keys/{id}/rotate | confirm: no overlap |
| Revoke | destructive-button | Button(role:.destructive) | DELETE /api-keys/{id} | confirm |
| Recent activity | list | List | audit events by this key |  |

| Action | Call | Result handling |
|---|---|---|
| Save | `PATCH /api-keys/{id}` |  |
| Rotate | `POST …/rotate` | 200 → KEY-03 |
| Revoke | `DELETE /api-keys/{id}` | 204 → row shows Revoked |

**States:** Loading: skeleton after 300 ms; Error: inline with Retry; codes per COPY-AND-STATES

### WHK-01 — Webhooks
**Domain:** D10 Developer Credentials · **Route:** `/dev/webhooks` · **Archetype:** list · **Phase:** Phase 2  
**Who:** webhooks.manage or app owner  
**Purpose:** Endpoints that receive access-change events.

**iOS / iPadOS container:** `List` in `NavigationStack` (iPad: split view)

**Loads:** `GET /webhooks?application_id`

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Rows | list | List | Webhook: url, events count, status healthy\|unhealthy\|disabled, consecutive_failures, last_delivery_at, scope |  |
| Add webhook | primary-button | Button .buttonStyle(.glassProminent/.borderedProminent) | → WHK-02 |  |

**States:** Loading: skeleton after 300 ms; Error: inline with Retry; codes per COPY-AND-STATES

### WHK-02 — Add / edit webhook
**Domain:** D10 Developer Credentials · **Route:** `/dev/webhooks/new` · **Archetype:** form · **Phase:** Phase 2  
**Who:** webhooks.manage or app owner  
**Purpose:** Subscribe a URL to events.

**iOS / iPadOS container:** `.sheet` with `NavigationStack` + `Form` (Cancel / primary in toolbar)

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| URL | url-field | TextField .keyboardType(.URL) | url | https required |
| Events | checkbox-list grouped | List with checkmark rows, Sections | events (WebhookEvent enum: access.*, entitlement.*, role.*, user.*, organization.*, application.*, device.*, notification.push_failed, push_credential.failed, webhook.test) | presets: Access changes (access.granted, access.revoked) · Push health (notification.push_failed, push_credential.failed, device.deactivated) |
| Description | text-field | TextField | description |  |
| Scope | select | Picker (menu / navigationLink) | scope |  |
| API version | select | Picker (menu / navigationLink) | api_version | edit only |
| Enabled | toggle | Toggle | status disabled ↔ healthy | edit only (re-enable) |

| Action | Call | Result handling |
|---|---|---|
| Create | `POST /webhooks` | 201 WebhookWithSecret → KEY-03 |
| Save | `PATCH /webhooks/{id}` |  |

### WHK-03 — Webhook — Deliveries
**Domain:** D10 Developer Credentials · **Route:** `/dev/webhooks/:hookId` · **Archetype:** detail · **Phase:** Phase 2  
**Who:** webhooks.manage or app owner  
**Purpose:** Delivery history, testing and secret rotation.

**iOS / iPadOS container:** Pushed view; segmented `Picker`; toolbar actions

**Loads:** `GET /webhooks/{id}` · `GET /webhooks/{id}/deliveries?status&since&event_id`

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Header actions | toolbar | .toolbar items |  | Send test · Rotate secret · Edit · Delete |
| Filter | chips | capsule Buttons row / .searchScopes | status pending\|succeeded\|failed |  |
| Rows | table | List (iPad: Table) | WebhookDelivery: status, response_status, event_type, event_id, attempt, duration_ms, created_at, next_retry_at, error | failed → Redeliver |

| Action | Call | Result handling |
|---|---|---|
| Send test | `POST /webhooks/{id}/test` | 202 → new pending row appears on refresh (poll 2 s × 5) |
| Rotate secret | `POST /webhooks/{id}/rotate-secret` | confirm (24 h overlap) → KEY-03 |
| Redeliver | `POST /webhooks/{id}/deliveries/{deliveryId}/redeliver` | 202 toast; same event_id |
| Delete | `DELETE /webhooks/{id}` | confirm |

**States:** Loading: skeleton after 300 ms; Error: inline with Retry; codes per COPY-AND-STATES

### WHK-04 — Delivery detail
**Domain:** D10 Developer Credentials · **Route:** `/dev/webhooks/:hookId/deliveries/:deliveryId` · **Archetype:** popover · **Phase:** Phase 2  
**Who:** webhooks.manage or app owner  
**Purpose:** One delivery attempt in detail.

**iOS / iPadOS container:** `.sheet` with `.presentationDetents([.medium, .large])`

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Facts | key-value | LabeledContent rows | all WebhookDelivery fields |  |
| Redeliver | button | Button .bordered |  |  |


## D11 · Plan & Billing

### BIL-01 — My plan
**Domain:** D11 Plan & Billing · **Route:** `/billing` · **Archetype:** settings · **Phase:** Phase 2  
**Who:** Self; billing.manage can view any User's (read-only)  
**Purpose:** Free or Standard: what covers you, your own subscription, upgrade and manage billing.

**iOS / iPadOS container:** Inset-grouped `List`/`Form`

**Loads:** `GET /users/me/subscription`

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Plan card | card | Section | UserSubscription.plan free\|standard | Free: "Free apps and services"; Standard: "Every app and service" |
| Covered by | list | List | plan_sources[]: {type user\|organization, id} | "Your subscription" / "Northwind Survey Co." (→ ORG-03); both listed when doubly covered |
| Double coverage tip | banner | inline List section | plan_sources has both user and organization | "Your organization already covers you. You can cancel your own subscription in Manage billing." (no automatic refund) |
| Your subscription | key-value | LabeledContent rows | Subscription.status, current_period_end, cancel_at_period_end, unit_amount, currency | status badge via status-semantics; past_due → warning banner (Stripe retries ~3 weeks) |
| Upgrade to Standard | primary-button | Button .buttonStyle(.glassProminent/.borderedProminent) | → BIL-03 checkout | when no covering subscription, or own subscription is incomplete/incomplete_expired/canceled/unpaid; price "$6 per month" from unit_amount when known, else from pricing config; store flags on iOS/Android |
| Manage billing | button | Button .bordered | → BIL-03 portal | when the user has completed a checkout (own subscription exists and isn't incomplete) |
| What Standard includes | link | Button .borderless | standard apps from GET /applications?access_tier=standard | count + list |

**States:** Loading: skeleton after 300 ms; Error: inline with Retry; codes per COPY-AND-STATES; After return from checkout: poll GET /users/me/subscription every 2 s (max 30 s) until plan = standard, showing "Finishing up…"; Downgraded: "Your Standard access ended on {date}. Your data is kept."

**Notes:** Business/Enterprise aren't offered (422 plan_requires_sales); show "Contact sales" link only. Annual plans and a Standard trial are undecided in the API (DECISIONS #31) — no UI for them.

### BIL-02 — Organization subscription
**Domain:** D11 Plan & Billing · **Route:** `/orgs/:orgId/subscription` · **Archetype:** tab · **Phase:** Phase 2  
**Who:** org_admin (manage); billing.manage (read + portal)  
**Purpose:** An organization's Standard subscription and which members it covers.

**iOS / iPadOS container:** Segment content

**Loads:** `GET /organizations/{id}/subscription` · `GET /organizations/{id}/members (for the member picker)`

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Plan card | card | Section | OrganizationSubscription.plan + Subscription.status, current_period_end, cancel_at_period_end |  |
| Billed users | key-value | LabeledContent rows | quantity.current (live), quantity.last_synced + last_synced_at, unit_amount | "31 covered · 30 billed on next invoice (synced daily)" |
| Who's covered | radio-group | Picker(.inline) | member_scope all_members \| allowlist \| denylist | COPY labels |
| Members | member-picker | searchable sheet | member_overrides (active members only) | visible for allowlist/denylist |
| Subscribe the organization | primary-button | Button .buttonStyle(.glassProminent/.borderedProminent) | → BIL-03 org checkout (member_scope optional) | org_admin; when no subscription or not in force |
| Manage billing | button | Button .bordered | → BIL-03 org portal | org_admin or billing.manage |

| Action | Call | Result handling |
|---|---|---|
| Save coverage | `PATCH /organizations/{id}/subscription {member_scope, member_overrides} (If-Match)` | effective immediately (access.granted/revoked); billed from next daily sync. 422 member_override_not_a_member → mark rows. 404 subscription_not_found → show Subscribe. |

**States:** Loading: skeleton after 300 ms; Error: inline with Retry; codes per COPY-AND-STATES

**Notes:** Members gained or lost take effect at once; say so before saving.

### BIL-03 — Checkout / billing portal
**Domain:** D11 Plan & Billing · **Route:** `/billing/return` · **Archetype:** handoff · **Phase:** Phase 2  
**Who:** Self (individual); org_admin (org checkout); org_admin or billing.manage (org portal)  
**Purpose:** Hand off to Stripe in the browser and confirm the result on return.

**iOS / iPadOS container:** `openURL` / `ASWebAuthenticationSession`

| Action | Call | Result handling |
|---|---|---|
| Upgrade (individual) | `POST /users/me/billing/checkout-sessions {plan:standard, success_url, cancel_url} (Idempotency-Key)` | 201 {url, expires_at} → open url externally. 409 subscription_exists → open portal instead. 409 test_mode_billing_unavailable / 503 billing_unavailable → explain. |
| Manage billing (individual) | `POST /users/me/billing/portal-sessions {return_url}` | 201 → open url externally. 409 no_billing_account → hide button. |
| Subscribe organization | `POST /organizations/{id}/billing/checkout-sessions {plan:standard, success_url, cancel_url, member_scope?, member_overrides?} (Idempotency-Key)` | as individual |
| Manage organization billing | `POST /organizations/{id}/billing/portal-sessions {return_url}` | as individual |
| Return | `deep link /billing/return?checkout=success\|cancelled (or /orgs/:orgId/subscription?…)` | success: poll the matching GET …/subscription until plan = standard (coverage starts when Stripe's webhook lands, usually seconds); cancelled: toast "Checkout cancelled" |

**Notes:** success_url / cancel_url / return_url must be https: use https://dashboard.substratalapps.com/billing/return… (Universal/App Links open the native app).

### BIL-04 — Tenant & infrastructure
**Domain:** D11 Plan & Billing · **Route:** `/dev/tenant` · **Archetype:** detail · **Phase:** Phase 2  
**Who:** Tenant owner (read), tenants.manage (write)  
**Purpose:** Where an Application owner's data lives, and support-run tier changes. Tenants aren't billed.

**iOS / iPadOS container:** Pushed view; segmented `Picker`; toolbar actions

**Loads:** `GET /tenants` · `GET /tenants/{id}` · `GET /tenants/{id}/tier-change-requests`

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Facts | key-value | LabeledContent rows | Tenant: tier, region, status, owner, application_count | status migrating → banner, writes may 503 |
| Tier change requests | list | List | TenantTierChangeRequest: from→requested tier/region, status, scheduled_for, reason, notes |  |
| Request change | button | Button .bordered | → form: requested_tier, requested_region, reason | tenants.manage only; owners see "isolated and dedicated-region tiers come with business pricing — contact sales" |

| Action | Call | Result handling |
|---|---|---|
| Open request | `POST /tenants/{id}/tier-change-requests` | 422/409 errors |
| Open request detail | `GET /tenants/{id}/tier-change-requests/{requestId}` | polls every 30 s while visible |
| Advance request | `PATCH /tenants/{id}/tier-change-requests/{requestId} {status, scheduled_for, notes} (If-Match)` | staff only; status scheduled → in_progress → completed, or cancelled |

**States:** Loading: skeleton after 300 ms; Error: inline with Retry; codes per COPY-AND-STATES


## D12 · Audit Log

### AUD-01 — Audit log
**Domain:** D12 Audit Log · **Route:** `/admin/audit` · **Archetype:** table · **Phase:** MVP  
**Who:** audit.view  
**Purpose:** Who changed what, when.

**iOS / iPadOS container:** `List` (iPad regular width: `Table`)

**Loads:** `GET /audit-events?since&until&actor_type&actor_id&action&target_type&target_id&target_user_id&application_id&organization_id&tenant_id`

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Filters | search + menus | .searchable | time range (presets 1h/24h/7d/custom), actor type, action (grouped by prefix, wildcard), resource | filters encoded in URL/state for sharing |
| Rows | table | List (iPad: Table) | timestamp, actor (type icon + name/id), action, target (type + link), app/org | relative time with absolute on hover/long-press |
| Inspector | side panel | pushed view | → AUD-02 | desktop: trailing pane; compact: push |

**States:** Loading: skeleton after 300 ms; Error: inline with Retry; codes per COPY-AND-STATES

**Notes:** Hot window is one year for every Tenant; read the `Audit-Hot-Window-Start` response header and show it in an info caption. Export is Phase 3. New actions to group in the filter: subscription.*, push_credential.*, device.deleted.

### AUD-02 — Audit event
**Domain:** D12 Audit Log · **Route:** `/admin/audit/:eventId` · **Archetype:** popover · **Phase:** MVP  
**Who:** audit.view  
**Purpose:** One event with before/after diff.

**iOS / iPadOS container:** `.sheet` with `.presentationDetents([.medium, .large])`

**Loads:** `GET /audit-events/{id}`

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Facts | key-value | LabeledContent rows | action, actor, target, target_user_id, application_id, organization_id, tenant_id, request_id, timestamp | each id links to its screen |
| Diff | diff view | Text(.monospaced) | before / after | JSON diff, changed keys highlighted |

**States:** Loading: skeleton after 300 ms; Error: inline with Retry; codes per COPY-AND-STATES


## D13 · Notifications

### NTF-01 — Inbox
**Domain:** D13 Notifications · **Route:** `/inbox` · **Archetype:** list · **Phase:** Phase 2  
**Who:** Self  
**Purpose:** Every notification the person has received — from every app and from Substratal — with read and archived state.

**iOS / iPadOS container:** `List` in `NavigationStack` (iPad: split view)

**Entry points:** Bell in the toolbar / Inbox in the sidebar (desktop) / Inbox tab (compact); Tapping a push; Badges (app icon, Dock/taskbar, PWA)

**Loads:** `GET /users/me/notifications?state&application_id&source&category&priority&created_after&created_before&include_inactive` · `GET /users/me/notifications/unread-count` · `GET /applications (names, icons)`

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Filter | segmented | Picker(.segmented) | Unread · All · Archived (state) | default All (unarchived) |
| App filter | menu | Menu | application_id, source=platform ("Substratal") | per-app unread counts from unread-count.applications[] |
| Rows | list | List | Notification: app icon (or Substratal mark for source=platform), title, body (2 lines), category, priority high → "Important" marker, created_at relative, unread dot | grouped Today / Yesterday / Earlier; high priority security.* rows show a shield icon |
| Mark all as read | button | Button .bordered | POST /users/me/notifications/mark-read {before: newest loaded created_at, application_id?} | respects current app filter; uses `before` so items arriving now aren't swept |
| Multi-select | selection | Picker (menu / navigationLink) |  | bulk Mark read / Mark unread / Archive (one PATCH per row) |
| Row actions | row-button | trailing Button/swipe action | PATCH /users/me/notifications/{notificationId} {state} | swipe/hover: Read ⇄ Unread, Archive |

| Action | Call | Result handling |
|---|---|---|
| Open row | `→ NTF-02 and PATCH {state:read}` | optimistic; badge decrements |
| Archive | `PATCH /users/me/notifications/{notificationId} {state:archived}` | toast with Undo (PATCH state:read) |
| Mark read / unread | `PATCH /users/me/notifications/{notificationId} {state}` |  |

**States:** Loading: skeleton after 300 ms; Error: inline with Retry; codes per COPY-AND-STATES; Empty: "You're all caught up"; Feature off (501 not_implemented): hide Inbox entry points; badges fall back to none; Retention: items older than 90 days disappear (caption in empty Archived view)

**Notes:** The Inbox shows notifications from every Application plus platform alerts, but push banners for another app's notifications arrive through that app's own install — the Hub only pushes its own (`app_hub`) notifications and platform alerts.

### NTF-02 — Notification
**Domain:** D13 Notifications · **Route:** `/inbox/:notificationId` · **Archetype:** popover · **Phase:** Phase 2  
**Who:** Self (users.list via NTF-05)  
**Purpose:** One notification in full, with where it leads and how its push went.

**iOS / iPadOS container:** `.sheet` with `.presentationDetents([.medium, .large])`

**Loads:** `GET /users/me/notifications/{notificationId}`

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Header | icon + app name + time | Image + Text + StatusBadge | application_id / source, created_at |  |
| Title & body | text | Text | title, body | full text |
| Open | primary-button | Button .buttonStyle(.glassProminent/.borderedProminent) | action_url | Hub route → navigate in-app; https → open in browser / installed app (Universal/App Link); private scheme → hand to OS; hidden when null |
| Contextual actions | buttons | Button .bordered | by category | security.new_sign_in → "Review sessions" (ACC-04) · security.password_changed / email_changed / mfa_disabled / session_reuse_detected → "Secure my account" (ACC-04) · account.organization_invitation → Accept / Decline (ORG-09) · account.access_granted → Open app (APP-03) |
| Delivery | disclosure | DisclosureGroup | push.status, devices_targeted/sent/failed, skip_reason, deliveries[] (30 days) | plain-language: "Pushed to 2 devices", "Not pushed — notifications are off for this app" |
| Mark unread / Archive | toolbar | .toolbar items | PATCH {state} |  |

| Action | Call | Result handling |
|---|---|---|
| Opened | `PATCH /users/me/notifications/{notificationId} {state:read}` | on open if unread |

**States:** Loading: skeleton after 300 ms; Error: inline with Retry; codes per COPY-AND-STATES

### NTF-03 — Devices & notifications
**Domain:** D13 Notifications · **Route:** `/account/devices` · **Archetype:** settings · **Phase:** Phase 2  
**Who:** Self  
**Purpose:** Where push notifications reach this person, and controls for this device.

**iOS / iPadOS container:** Inset-grouped `List`/`Form`

**Loads:** `GET /users/me/devices?include_inactive=true` · `GET /users/me/settings (notifications.push)`

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| This device | card | Section | Hub Device for this install (matched by stored device id) | status, "Notifications are on" / off / blocked by OS; button Turn on (→ NTF-04) or Turn off |
| Push notifications (all apps) | toggle | Toggle | Settings.notifications.push | same control as SET-01 |
| Devices | list | List | Device: name, application_id (app icon), provider, environment, status, inactive_reason, last_registered_at, last_delivery_at, token_hint | grouped by app; inactive rows dimmed with reason ("Signed out", "Not opened in 9 months"…) |
| Rename | row-button | trailing Button/swipe action | PATCH /users/me/devices/{deviceId} {name} |  |
| Send test notification | row-button | trailing Button/swipe action | POST /users/me/devices/{deviceId}/test | Hub devices only; shows provider result inline (sent / failed + error) within 10 s |
| Remove | row-button | trailing Button/swipe action | DELETE /users/me/devices/{deviceId} | confirm; that install stops receiving pushes until it registers again |

| Action | Call | Result handling |
|---|---|---|
| Turn off on this device | `DELETE /users/me/devices/{deviceId}` | and stop re-registering until the user turns it back on |
| Test | `POST /users/me/devices/{deviceId}/test` | 429 → "Try again later (10 tests per hour)"; 409 push_not_configured → "Push isn't set up for this app yet" |

**States:** Loading: skeleton after 300 ms; Error: inline with Retry; codes per COPY-AND-STATES

**Notes:** Device tokens are never displayed (only token_hint).

### NTF-04 — Turn on notifications
**Domain:** D13 Notifications · **Route:** `/account/devices/enable` · **Archetype:** wizard · **Phase:** Phase 2  
**Who:** Self  
**Purpose:** Explain, request the OS permission, and register this install for push.

**iOS / iPadOS container:** Sheet with `NavigationStack` push per step

**Entry points:** First time a notification-worthy moment happens (org invite, first launch after sign-in on mobile, enabling in NTF-03/SET-01) — never on cold start before sign-in

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Primer | text + illustration | Text |  | what you'll get: security alerts, invitations, access changes; "You can change this anytime" |
| Turn on | primary-button | Button .buttonStyle(.glassProminent/.borderedProminent) | OS permission prompt | platform API per PLATFORM-SPEC |
| Not now | button | Button .bordered |  | snooze 14 days |

| Action | Call | Result handling |
|---|---|---|
| Permission granted | `POST /users/me/devices {application_id: app_hub, provider, token \| subscription, environment (apns), name, app_version}` | 201 new / 200 refresh → store device id; 403 entitlement_required → Hub access disabled for this user: show inbox-only mode; 422 invalid_device_token → log + retry next launch |
| Token rotated by OS | `PATCH /users/me/devices/{deviceId} {token \| subscription}` |  |
| Permission denied | `—` | Settings shows "Blocked — Open system settings" deep link |

**Notes:** Re-register on launch at most once a day (keeps the device from going stale after 270 days). Windows and Linux have no supported push provider in the API: this screen is replaced by an in-app explanation and local notifications from polling (see their PLATFORM-SPEC).

### NTF-05 — User — Notifications & devices
**Domain:** D13 Notifications · **Route:** `/admin/users/:userId/notifications` · **Archetype:** tab · **Phase:** Phase 2  
**Who:** users.list (read), users.manage (delete devices)  
**Purpose:** Support view of a person's inbox and push devices for investigations.

**iOS / iPadOS container:** Segment content

**Loads:** `GET /users/{id}/notifications?include_inactive=true` · `GET /users/{id}/notifications/{notificationId}` · `GET /users/{id}/devices?include_inactive=true` · `GET /users/{id}/devices/{deviceId}`

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Inbox | table | List (iPad: Table) | Notification: created_at, application/source, category, priority, title, state, push.status, skip_reason | read-only; never mark the user's items read |
| Delivery detail | side panel | pushed view | deliveries[]: provider, status, attempts, error, sent_at |  |
| Devices | table | List (iPad: Table) | Device: app, provider, environment, status, inactive_reason, name, app_version, last_registered_at, last_delivery_at |  |
| Delete device | destructive-button | Button(role:.destructive) | DELETE /users/{id}/devices/{deviceId} | users.manage; audited (device.deleted) |

**States:** Loading: skeleton after 300 ms; Error: inline with Retry; codes per COPY-AND-STATES

### NTF-06 — Send notification
**Domain:** D13 Notifications · **Route:** `/admin/notifications/new` · **Archetype:** form · **Phase:** Phase 2  
**Who:** platform notifications.send  
**Purpose:** Staff tool: send one notification to one person on behalf of an Application.

**iOS / iPadOS container:** `.sheet` with `NavigationStack` + `Form` (Cancel / primary in toolbar)

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Recipient | picker | searchable sheet | user (GET /users?q) | must have active access to the chosen app |
| Application | select | Picker (menu / navigationLink) | body.application_id | required for platform senders |
| Category | text-field | TextField | body.category | ^[a-z][a-z0-9_]*(\.[a-z0-9_]+)*$; security./account./billing. reserved (422 reserved_category) |
| Priority | segmented | Picker(.segmented) | body.priority normal \| high | high explains provider throttling |
| Title | text-field | TextField | body.title | 1–120 |
| Body | text-area | TextField(axis:.vertical) | body.body | ≤ 1,000 |
| Data | key/value editor | List of key/value rows + Add | body.data | ≤ 20 string pairs, 2 KB; key `substratal` reserved |
| Open link | url-field | TextField .keyboardType(.URL) | body.action_url | https or private scheme ≤ 2,048 |
| Advanced | disclosure | DisclosureGroup | push (bool), badge, sound default\|silent, collapse_key, ttl_seconds |  |
| Payload size | meters | Gauge | client estimate of the 4 KB push payload | 422 notification_too_large → details.bytes/max |
| Preview | card | Section | lock-screen style preview per platform | warns: lock screens show title/body — keep sensitive content out |

| Action | Call | Result handling |
|---|---|---|
| Send | `POST /users/{id}/notifications (Idempotency-Key)` | 201 → result panel with push.status / skip_reason; 403 entitlement_required → recipient has no access; 429 scope sender\|recipient → countdown |


## G · Global / Shell

### GLB-01 — Command palette
**Domain:** G Global / Shell · **Route:** `(overlay)` · **Archetype:** overlay · **Phase:** MVP  
**Who:** Signed in  
**Purpose:** Jump anywhere and run any action by typing.

**iOS / iPadOS container:** Sheet or search tab

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Query | search | .searchable |  | Cmd/Ctrl+K (desktop), search button (touch) |
| Results | grouped list | List Sections | Navigation, Apps (launch), Users (GET /users?q), Orgs, Inbox (recent notifications), Actions (Invite user, Create key, Upgrade to Standard, Mark all read…) | permission-filtered; recent items first; ID prefixes resolve directly |

### GLB-02 — Manage hub
**Domain:** G Global / Shell · **Route:** `/manage` · **Archetype:** hub · **Phase:** MVP  
**Who:** Any admin or developer permission  
**Purpose:** Compact-layout landing list for the Administration and Developer sections.

**iOS / iPadOS container:** Admin / Developer tab root `List`

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Administration | list | List | Users, Access, Roles, Audit Log, Send notification — those the caller may open, each with a live count | section hidden when no rights |
| Developer | list | List | Applications, API Keys, Webhooks, Tenant — with counts and health (unhealthy webhooks, failed push credentials) | section hidden when no rights |

### GLB-03 — System states
**Domain:** G Global / Shell · **Route:** `(global)` · **Archetype:** overlay · **Phase:** MVP  
**Who:** —  
**Purpose:** App-wide banners and blocking states.

**iOS / iPadOS container:** Sheet or search tab

| Element | Kind | iOS / iPadOS control | Data / API | Rules |
|---|---|---|---|---|
| Offline | banner | inline List section | network status | reads from cache, writes disabled with explanation |
| Maintenance / tenant migrating | banner | inline List section | 503 service_unavailable + Retry-After | countdown, auto-retry reads |
| Session ended | modal → AUTH-01 | Alert → AUTH-01 | 401 after failed refresh | preserve destination |
| Forbidden | inline state | ProgressView on tile | 403 | COPY |
| Not built yet | inline state | ProgressView on tile | 501 not_implemented (Notifications today) | hide the feature behind its flag; never show an error to end users |
| Update required | blocking screen | fullScreenCover | API version deprecation header (if present) |  |

| Action | Call | Result handling |
|---|---|---|
| Silent token refresh | `POST /auth/token/refresh` | single-flight; rotate refresh token atomically (CLIENT-ARCHITECTURE §2) |
| Connectivity probe | `GET /health` | used after offline/503 to decide when to clear the banner |
| Forced sign-out | `POST /auth/logout` | on session_revoked or user request; the server deactivates this session's push devices |
| Badge refresh | `GET /users/me/notifications/unread-count` | on launch, foreground, after a push arrives, and every 60 s while visible (Windows/Linux: also in background) → app icon/Dock/taskbar/PWA badge |

**Notes:** `POST /auth/oauth/token` is called by the launched Application (PKCE exchange), never by the Hub.
