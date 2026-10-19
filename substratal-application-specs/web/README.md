# Substratal Hub for Web (PWA) — Team Handoff

**Standard:** Responsive PWA based on the macOS design; WCAG 2.2 · **Stack:** TypeScript, React, React Aria, TanStack, Workbox
**API:** Substratal Apps Platform API v1.4.0 (openapi 1.0.0-draft.3) — https://compositecode.github.io/substratalapps.com/

**Web-specific:** the responsive layout classes, input adaptation, PWA, live system preferences and platform APIs are in PLATFORM-SPEC §3–§8; per-screen behavior at each size is in §12.

## Reading order

1. `shared/CHANGES.md` — what's new in this edition (push notifications, Free/Standard billing)
2. `shared/PRODUCT-SPEC.md` — scope, personas, domains, phases, acceptance criteria
3. `PLATFORM-SPEC.md` — how this app is built on Web (PWA)
4. `SCREENS.md` — all 75 screens with native containers and controls (the build checklist)
5. `shared/CLIENT-ARCHITECTURE.md` — sessions, request conventions, Capabilities, push and inbox (§6), billing hand-off (§9)
6. `shared/UX-FLOWS.md` (+ `shared/flows/*.svg`) — the 16 end-to-end flows
7. `mockups/index.html` / `screens/` — reference renders in the default light (Moss Paper) and dark (Forest Floor) themes; `?mode=dark&light=<id>&dark=<id>` previews any theme
8. While building: `shared/COMPONENTS.md`, `shared/COPY-AND-STATES.md`, `shared/ACCESSIBILITY.md`, `shared/NON-FUNCTIONAL.md`, `shared/OPEN-QUESTIONS.md`, `shared/API-SCREEN-MAP.csv`

**Themes:** `theme/` has this platform's implementation of the 5 light and 5 dark themes. Rules are in `shared/THEMES.md`; wiring is in `PLATFORM-SPEC.md` (Theming section).

**Logo & app icon:** `brand-assets/` has every icon and logo file this app needs. Wiring is in `PLATFORM-SPEC.md` (Brand & app icon section) and rules in `shared/brand/BRAND.md`.

## Definition of done (per screen)

- [ ] Elements, data bindings and API calls match `SCREENS.md`
- [ ] Reachable by its route / deep link; restores state from the route
- [ ] Gated by Capabilities per `shared/INFORMATION-ARCHITECTURE.md` §2
- [ ] Loading, empty, error (401/403/404/409/422/429/503) and offline states
- [ ] Idempotency-Key on creates, If-Match on edits, conflict handling
- [ ] Every gesture/shortcut has a visible equivalent; screen reader and largest text size pass
- [ ] Copy matches `shared/COPY-AND-STATES.md`
- [ ] Correct in both default themes and in every other theme (colours only from theme roles, no literals); high-contrast modes fall back to system colours
- [ ] Logo and icons come only from `brand-assets/` (no redraws), with accessible name "Substratal Apps"
- [ ] Flag-gated features (notifications, push, billing) hide cleanly on `501`

## Reference screens

- `desktop-ACC-04-security`: [light](screens/light/desktop-ACC-04-security.png) · [dark](screens/dark/desktop-ACC-04-security.png)
- `desktop-ACX-01-03-access-turn-off`: [light](screens/light/desktop-ACX-01-03-access-turn-off.png) · [dark](screens/dark/desktop-ACX-01-03-access-turn-off.png)
- `desktop-APL-03-application`: [light](screens/light/desktop-APL-03-application.png) · [dark](screens/dark/desktop-APL-03-application.png)
- `desktop-APL-07-push-credentials`: [light](screens/light/desktop-APL-07-push-credentials.png) · [dark](screens/dark/desktop-APL-07-push-credentials.png)
- `desktop-APP-01-home`: [light](screens/light/desktop-APP-01-home.png) · [dark](screens/dark/desktop-APP-01-home.png)
- `desktop-AUD-01-02-audit`: [light](screens/light/desktop-AUD-01-02-audit.png) · [dark](screens/dark/desktop-AUD-01-02-audit.png)
- `desktop-AUTH-01-02-sign-in`: [light](screens/light/desktop-AUTH-01-02-sign-in.png) · [dark](screens/dark/desktop-AUTH-01-02-sign-in.png)
- `desktop-BIL-01-my-plan`: [light](screens/light/desktop-BIL-01-my-plan.png) · [dark](screens/dark/desktop-BIL-01-my-plan.png)
- `desktop-GLB-01-command-palette`: [light](screens/light/desktop-GLB-01-command-palette.png) · [dark](screens/dark/desktop-GLB-01-command-palette.png)
- `desktop-KEY-01-03-api-keys-secret`: [light](screens/light/desktop-KEY-01-03-api-keys-secret.png) · [dark](screens/dark/desktop-KEY-01-03-api-keys-secret.png)
- `desktop-NTF-01-02-inbox`: [light](screens/light/desktop-NTF-01-02-inbox.png) · [dark](screens/dark/desktop-NTF-01-02-inbox.png)
- `desktop-NTF-03-devices`: [light](screens/light/desktop-NTF-03-devices.png) · [dark](screens/dark/desktop-NTF-03-devices.png)
- `desktop-ORG-06-org-apps-seats`: [light](screens/light/desktop-ORG-06-org-apps-seats.png) · [dark](screens/dark/desktop-ORG-06-org-apps-seats.png)
- `desktop-ROL-02-role`: [light](screens/light/desktop-ROL-02-role.png) · [dark](screens/dark/desktop-ROL-02-role.png)
- `desktop-SET-01-appearance-themes`: [light](screens/light/desktop-SET-01-appearance-themes.png) · [dark](screens/dark/desktop-SET-01-appearance-themes.png)
- `desktop-SET-02-app-settings`: [light](screens/light/desktop-SET-02-app-settings.png) · [dark](screens/dark/desktop-SET-02-app-settings.png)
- `desktop-USR-01-multiselect-preview`: [light](screens/light/desktop-USR-01-multiselect-preview.png) · [dark](screens/dark/desktop-USR-01-multiselect-preview.png)
- `desktop-USR-01-users`: [light](screens/light/desktop-USR-01-users.png) · [dark](screens/dark/desktop-USR-01-users.png)
- `desktop-WHK-03-deliveries`: [light](screens/light/desktop-WHK-03-deliveries.png) · [dark](screens/dark/desktop-WHK-03-deliveries.png)
- `phone-ACC-04-security`: [light](screens/light/phone-ACC-04-security.png) · [dark](screens/dark/phone-ACC-04-security.png)
- `phone-ACX-01-03-access-turn-off`: [light](screens/light/phone-ACX-01-03-access-turn-off.png) · [dark](screens/dark/phone-ACX-01-03-access-turn-off.png)
- `phone-ACX-02-sheet`: [light](screens/light/phone-ACX-02-sheet.png) · [dark](screens/dark/phone-ACX-02-sheet.png)
- `phone-APL-03-application`: [light](screens/light/phone-APL-03-application.png) · [dark](screens/dark/phone-APL-03-application.png)
- `phone-APL-07-push-credentials`: [light](screens/light/phone-APL-07-push-credentials.png) · [dark](screens/dark/phone-APL-07-push-credentials.png)
- `phone-APP-01-home`: [light](screens/light/phone-APP-01-home.png) · [dark](screens/dark/phone-APP-01-home.png)
- `phone-APP-01-long-press`: [light](screens/light/phone-APP-01-long-press.png) · [dark](screens/dark/phone-APP-01-long-press.png)
- `phone-AUD-01-02-audit`: [light](screens/light/phone-AUD-01-02-audit.png) · [dark](screens/dark/phone-AUD-01-02-audit.png)
- `phone-AUTH-01-02-sign-in`: [light](screens/light/phone-AUTH-01-02-sign-in.png) · [dark](screens/dark/phone-AUTH-01-02-sign-in.png)
- `phone-BIL-01-my-plan`: [light](screens/light/phone-BIL-01-my-plan.png) · [dark](screens/dark/phone-BIL-01-my-plan.png)
- `phone-GLB-02-manage-hub`: [light](screens/light/phone-GLB-02-manage-hub.png) · [dark](screens/dark/phone-GLB-02-manage-hub.png)
- `phone-KEY-01-03-api-keys-secret`: [light](screens/light/phone-KEY-01-03-api-keys-secret.png) · [dark](screens/dark/phone-KEY-01-03-api-keys-secret.png)
- `phone-NTF-01-02-inbox`: [light](screens/light/phone-NTF-01-02-inbox.png) · [dark](screens/dark/phone-NTF-01-02-inbox.png)
- `phone-NTF-03-devices`: [light](screens/light/phone-NTF-03-devices.png) · [dark](screens/dark/phone-NTF-03-devices.png)
- `phone-ORG-06-org-apps-seats`: [light](screens/light/phone-ORG-06-org-apps-seats.png) · [dark](screens/dark/phone-ORG-06-org-apps-seats.png)
- `phone-ROL-02-role`: [light](screens/light/phone-ROL-02-role.png) · [dark](screens/dark/phone-ROL-02-role.png)
- `phone-SET-01-appearance-themes`: [light](screens/light/phone-SET-01-appearance-themes.png) · [dark](screens/dark/phone-SET-01-appearance-themes.png)
- `phone-SET-02-app-settings`: [light](screens/light/phone-SET-02-app-settings.png) · [dark](screens/dark/phone-SET-02-app-settings.png)
- `phone-USR-01-swipe-pull`: [light](screens/light/phone-USR-01-swipe-pull.png) · [dark](screens/dark/phone-USR-01-swipe-pull.png)
- `phone-USR-01-users`: [light](screens/light/phone-USR-01-users.png) · [dark](screens/dark/phone-USR-01-users.png)
- `phone-WHK-03-deliveries`: [light](screens/light/phone-WHK-03-deliveries.png) · [dark](screens/dark/phone-WHK-03-deliveries.png)
- `tablet-ACC-04-security`: [light](screens/light/tablet-ACC-04-security.png) · [dark](screens/dark/tablet-ACC-04-security.png)
- `tablet-ACX-01-03-access-turn-off`: [light](screens/light/tablet-ACX-01-03-access-turn-off.png) · [dark](screens/dark/tablet-ACX-01-03-access-turn-off.png)
- `tablet-ACX-01-two-pane`: [light](screens/light/tablet-ACX-01-two-pane.png) · [dark](screens/dark/tablet-ACX-01-two-pane.png)
- `tablet-APL-03-application`: [light](screens/light/tablet-APL-03-application.png) · [dark](screens/dark/tablet-APL-03-application.png)
- `tablet-APL-07-push-credentials`: [light](screens/light/tablet-APL-07-push-credentials.png) · [dark](screens/dark/tablet-APL-07-push-credentials.png)
- `tablet-APP-01-home`: [light](screens/light/tablet-APP-01-home.png) · [dark](screens/dark/tablet-APP-01-home.png)
- `tablet-AUD-01-02-audit`: [light](screens/light/tablet-AUD-01-02-audit.png) · [dark](screens/dark/tablet-AUD-01-02-audit.png)
- `tablet-AUTH-01-02-sign-in`: [light](screens/light/tablet-AUTH-01-02-sign-in.png) · [dark](screens/dark/tablet-AUTH-01-02-sign-in.png)
- `tablet-BIL-01-my-plan`: [light](screens/light/tablet-BIL-01-my-plan.png) · [dark](screens/dark/tablet-BIL-01-my-plan.png)
- `tablet-KEY-01-03-api-keys-secret`: [light](screens/light/tablet-KEY-01-03-api-keys-secret.png) · [dark](screens/dark/tablet-KEY-01-03-api-keys-secret.png)
- `tablet-NTF-01-02-inbox`: [light](screens/light/tablet-NTF-01-02-inbox.png) · [dark](screens/dark/tablet-NTF-01-02-inbox.png)
- `tablet-NTF-03-devices`: [light](screens/light/tablet-NTF-03-devices.png) · [dark](screens/dark/tablet-NTF-03-devices.png)
- `tablet-ORG-06-org-apps-seats`: [light](screens/light/tablet-ORG-06-org-apps-seats.png) · [dark](screens/dark/tablet-ORG-06-org-apps-seats.png)
- `tablet-ROL-02-role`: [light](screens/light/tablet-ROL-02-role.png) · [dark](screens/dark/tablet-ROL-02-role.png)
- `tablet-SET-01-appearance-themes`: [light](screens/light/tablet-SET-01-appearance-themes.png) · [dark](screens/dark/tablet-SET-01-appearance-themes.png)
- `tablet-SET-02-app-settings`: [light](screens/light/tablet-SET-02-app-settings.png) · [dark](screens/dark/tablet-SET-02-app-settings.png)
- `tablet-USR-01-users`: [light](screens/light/tablet-USR-01-users.png) · [dark](screens/dark/tablet-USR-01-users.png)
- `tablet-WHK-03-deliveries`: [light](screens/light/tablet-WHK-03-deliveries.png) · [dark](screens/dark/tablet-WHK-03-deliveries.png)
- `tablet-sidebar-drawer`: [light](screens/light/tablet-sidebar-drawer.png) · [dark](screens/dark/tablet-sidebar-drawer.png)
