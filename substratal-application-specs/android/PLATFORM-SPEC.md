# Substratal Hub for Android — Platform Spec

**Standard:** Material Design 3 (Expressive), Android app quality and large-screen guidelines.
**Read with:** `SCREENS.md` (all 75 screens with Compose controls), `shared/`.

## 1. Technology

| Concern | Choice |
|---|---|
| UI | Kotlin, Jetpack Compose, Material 3 + `material3-adaptive` (`NavigationSuiteScaffold`, `ListDetailPaneScaffold`, `SupportingPaneScaffold`) |
| SDK | minSdk 26, targetSdk 36; edge-to-edge (mandatory at target 35+); predictive back enabled |
| Architecture | MVVM + UDF (ViewModel + `StateFlow`), Hilt DI, Navigation Compose with type-safe routes mirroring SCREEN-SPECS |
| API client | openapi-generator (Kotlin + OkHttp/Ktor) from `openapi.yaml`; one `Authenticator` does single-flight refresh |
| Secure storage | Android Keystore AES-GCM key → Tink-encrypted DataStore for the refresh token; excluded from backup (`dataExtractionRules`) |
| Cache | Room (catalog, entitlements, profile) |
| Paging | Paging 3 with cursor `RemoteMediator`-free `PagingSource` |
| Distribution | Google Play (AAB); ChromeOS and large screens declared |

## 2. Navigation

`NavigationSuiteScaffold` chooses the container by window size class:

| Width | Container | Destinations |
|---|---|---|
| Compact < 600 dp | bottom `NavigationBar` | Home · Inbox · Orgs · Manage · Account |
| Medium 600–839 dp | `NavigationRail` (FAB in rail header) | same 5 |
| Expanded ≥ 840 dp | permanent drawer / wide rail | full tree (sections as on desktop) |

Manage is the GLB-02 hub (Administration + Developer) on compact/medium and is removed without rights; Inbox carries a `BadgedBox` unread count; Plan & Billing and Settings live in Account. List-detail screens use `ListDetailPaneScaffold` (two panes when expanded, predictive-back between panes on compact). Inspector-style content (AUD-02, APP-02 on large screens) uses `SupportingPaneScaffold`. Top bars: `MediumFlexibleTopAppBar` on destination roots, `TopAppBar` with back on pushed screens. Primary action per screen = Extended FAB (collapses on scroll).

Foldables: book posture → list | detail across the hinge; tabletop posture → content top, actions bottom.

## 3. Interaction

| Interaction | Behavior |
|---|---|
| Long-press | `combinedClickable(onLongClick)` → `DropdownMenu` (same items as ⋯ / top-bar actions); haptic `LongPress` |
| Swipe | `SwipeToDismissBox` for row actions: start-to-end safe (Accept, Turn on, Mark read), end-to-start destructive (Turn off, Remove, Revoke → confirm, row snaps back) |
| Pull to refresh | `PullToRefreshBox` on lists |
| Multi-select | long-press first item enters selection mode (checkboxes, contextual top bar with count + actions); keyboard/mouse: Shift/Ctrl-click |
| Drag & drop | `dragAndDropSource`/`dragAndDropTarget`: drop CSV/TXT/images/JSON from other apps (multi-window, ChromeOS, desktop mode); drag export/audit JSON out as content URI; reorder pinned tiles from a drag handle |
| Mouse/trackpad (ChromeOS, desktop mode, tablets) | right-click = context menu; hover states; hover preview cards on names (`pointerHoverIcon`, tooltip box) |
| Keyboard | Ctrl+K palette, Ctrl+F search, Ctrl+N new, Ctrl+R refresh, Ctrl+Z undo, Delete destructive (confirm), Ctrl+Enter submit; published via `onProvideKeyboardShortcuts` (Meta+/ helper) |
| Stylus | precise pointer; handwriting into text fields (Android 14+ default) |
| Toast | `Snackbar` (with Undo for access off, 10 s) |
| Secret reveal | non-dismissable dialog + `FLAG_SECURE` on the window while visible; clipboard `EXTRA_IS_SENSITIVE` |

## 4. Platform integration

| Concern | Implementation |
|---|---|
| Credentials | Credential Manager (`GetPasswordOption`, passkeys when flag on); save after sign-in |
| Autofill | Compose `contentType` semantics on AUTH fields |
| Biometric lock | `BiometricPrompt` (`BIOMETRIC_STRONG or DEVICE_CREDENTIAL`) bound to Keystore key (opt-in) |
| Deep links | verified App Links (`assetlinks.json`) + `substratal://` |
| App launch / PKCE / billing | Custom Tabs (Auth Tab where available) |
| Push | FCM as `app_hub` — see §4a |
| Share | Sharesheet (`Intent.ACTION_SEND`) for links/files; **share target**: receive CSV/TXT/vCard → bulk invite review |
| Files | Storage Access Framework (`CreateDocument`, `OpenDocument`); Photo Picker + camera intent for avatar |
| App shortcuts | static/dynamic shortcuts: Home, Users, API Keys, Audit Log, recent users |
| Widgets | Glance widget "Quick launch" (pinned apps) |
| Billing | BIL-01/02 status in-app; Checkout/Portal in Custom Tabs with App Link return and polling. **Upgrade** gated by `billing_upgrade_android`, default off until Play payments-policy review (OPEN-QUESTIONS #10). |
| Play policies | account deletion in-app (ACC-08) + web link; Data safety form declares push tokens and Firebase |

## 4a. Push notifications (FCM)

| Step | Implementation |
|---|---|
| Setup | Firebase project for `app_hub`; its service account JSON is the `fcm` credential (APL-07, staff) |
| Channels | **Security alerts** (`IMPORTANCE_HIGH`, `security.*`), **Account & invitations** (`IMPORTANCE_DEFAULT`, `account.*`), **App updates** (everything else); created on first launch |
| Permission | Android 13+: `POST_NOTIFICATIONS` runtime request from the NTF-04 primer at a meaningful moment; below 13 notifications are on by default |
| Register | `FirebaseMessaging.getInstance().token` → `POST /users/me/devices {application_id: "app_hub", provider: "fcm", token, name: device name, app_version}`; store the `dvc_` id |
| Rotate | `FirebaseMessagingService.onNewToken` → `PATCH /users/me/devices/{id}`; daily re-register on launch |
| Receive | `onMessageReceived` reads `message.data["substratal"]` (JSON string): foreground → in-app banner + refresh; background notification messages display on the channel chosen by category |
| Tap / actions | `PendingIntent` to the route in `action_url` (App Links) or NTF-02; `NotificationCompat.Action`s per category (Accept invitation runs a WorkManager job calling `POST …/members/me/accept`, requiring unlock) |
| Badge | launcher badge follows active notifications; in-app badges from `unread-count` |
| Turn off | NTF-03 → `DELETE /users/me/devices/{id}` and `deleteToken()` |

## 4b. Brand & app icon

Assets: `brand-assets/res/` merges straight into `app/src/main/res` (see `shared/brand/BRAND.md`).

| Item | File | Wiring |
|---|---|---|
| Adaptive icon | `mipmap-anydpi-v26/ic_launcher(.xml, _round.xml)`, `drawable/ic_launcher_foreground.xml` (vector, mark inside the 66 dp safe zone), `values/ic_launcher_background.xml` (`#E8E8F7`; night `#15132E` with the on-dark foreground) | `android:icon="@mipmap/ic_launcher"`, `android:roundIcon="@mipmap/ic_launcher_round"` |
| Themed icon (13+) | `drawable/ic_launcher_monochrome.xml` | Already referenced by `<monochrome>` |
| Legacy launchers | `mipmap-{m…xxxh}dpi/ic_launcher(.png, _round.png)` | — |
| Notification small icon | `drawable/ic_stat_substratal.xml` (vector) and `drawable-*dpi/ic_stat_substratal.png` | `NotificationCompat.Builder.setSmallIcon(R.drawable.ic_stat_substratal)` and the FCM `default_notification_icon` meta-data |
| Splash | `values/splash.xml` (`Theme.SplashScreen`) | Set as the launcher activity theme and call `installSplashScreen()` |
| Logo in UI | `drawable/brand_lockup.xml`, `drawable/brand_mark.xml` (+ `-night`) | `Image(painterResource(R.drawable.brand_lockup), contentDescription = "Substratal Apps")` |
| Play listing | `play-store/ic_launcher-playstore.png` (512), `feature-graphic-1024x500.png` | Upload in Play Console |

All crossing gaps are `clip-path` cut-outs, so the foreground and monochrome layers are truly transparent at the gaps.


## 4c. Theming

`theme/SubstratalThemes.kt` and `theme/res/values{,-night}/substratal_theme.xml`.

- Wrap content in `SubstratalTheme(mode, light, dark)`. All ten themes are full Material 3 `ColorScheme`s, including the `surfaceContainer*` roles.
- **Dynamic colour (Material You) is off**, because the Substratal themes are the product palette.
- Set the system bar icon colours from the resolved scheme with `enableEdgeToEdge(SystemBarStyle.auto(...))`.
- The splash screen uses the XML defaults (Moss Paper / Forest Floor).
- SET-01 swatches: `LazyVerticalGrid(GridCells.Fixed(3))` with `selectable(role = Role.RadioButton)`.

## 5. Layout metrics

- Margins 16 dp (compact), 24 dp (medium/expanded); list items 56/72 dp; targets ≥ 48 dp.
- M3 type scale (`titleLarge` app bar, `headlineMedium` destination titles, `bodyLarge`, `bodyMedium`, `labelLarge`); IDs monospace.
- M3 Expressive shapes: grouped lists 20 dp outer / 4 dp inner, cards 24 dp, dialogs 28 dp, FAB 16 dp.
- Icons: Material Symbols per INFORMATION-ARCHITECTURE §5.

## 6. Accessibility

TalkBack: row-level `toggleable` for switches with role Switch; `customActions` mirror long-press and swipe actions; `stateDescription` for badges. Font scale 200 % (non-linear) reflows the grid to 1 column. Switch Access and keyboard-only navigation for every flow. Remove animations honored.

## 7. Testing checklist

- [ ] FCM end to end: POST_NOTIFICATIONS grant/deny, channels, `onNewToken` PATCH, sign-out deactivation, notification actions, device test.
- [ ] Billing round trip via Custom Tabs + App Link return; Upgrade hidden while `billing_upgrade_android` is off.
- [ ] Pixel compact, foldable (folded/unfolded/book/tabletop), 10" tablet, ChromeOS with mouse + keyboard.
- [ ] 11 flows with TalkBack, Switch Access, keyboard only, font scale 200 %.
- [ ] Persona matrix — destinations and actions.
- [ ] Predictive back across panes and dialogs; process death restores route + form state.
- [ ] App Links verification for verify/reset/invite.
