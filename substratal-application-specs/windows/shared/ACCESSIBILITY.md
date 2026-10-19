# Accessibility & Internationalization

Target: **WCAG 2.2 AA** on every platform, plus each platform's own accessibility guidance.

## Requirements (all platforms)

| Area | Requirement |
|---|---|
| Screen readers | VoiceOver (macOS/iOS, Safari), Narrator (Windows), TalkBack (Android), Orca (Linux via AT-SPI), NVDA/JAWS (Web on Windows). Every control labeled; StatusBadge reads "{App}, {status label}". AccessSwitch reads "Access to {app}, on/off, switch". |
| Status | Never color-only (icon + text label). Badge text meets 4.5:1 in every theme. Theme contrast for every theme is tabulated in THEMES.md §4. |
| Text scaling | Dynamic Type (iOS/macOS where applicable) up to AX5; Android font scale up to 200 %; Windows text scaling 225 %; GNOME Large Text. Grids reflow to fewer columns; no truncation of status labels. |
| Keyboard | Full keyboard operation on desktop, iPad and Android with hardware keyboards, and Web (WAI-ARIA Authoring Practices patterns for grid, tabs, menu, dialog, combobox). Visible focus ring (platform native). Tab order follows reading order. Shortcuts listed in each platform spec. |
| Targets | ≥ 44×44 pt (Apple), 48×48 dp (Android), 40×40 epx (Windows touch: 44), 34 px min (GNOME). |
| Motion | Respect Reduce Motion / Animations off; toggles and sheets cross-fade instead of slide. |
| Contrast modes | Windows High Contrast themes (use `SystemColor*` resources, no hard-coded colors), macOS/iOS Increase Contrast, GNOME High Contrast, Android high-contrast text, Web `prefers-contrast` / `forced-colors`. |
| Timeouts | MFA challenge countdown announced at 60 s and 10 s; user can request a new challenge. |
| Secrets | Secrets are selectable text, not images; screen readers can read them character by character. |

## Internationalization

- All strings externalized: String Catalogs (`.xcstrings`) on Apple, `.resw` on Windows, `strings.xml` on Android, gettext `.po` on Linux.
- Dates/times rendered in the user's Profile `timezone` and `locale` (from `GET /users/{id}/profile`), falling back to OS settings. Show relative times in lists ("3 min ago") with absolute time on hover/long-press.
- Support RTL layout mirroring (Arabic, Hebrew): icons with direction (chevrons, arrows) mirror; app icons and QR codes do not.
- IDs (`usr_…`) are never localized or mirrored.

## Gesture and shortcut parity

Every gesture (swipe, long-press, pinch, drag) and every keyboard shortcut is an accelerator for an action that also has a visible button or menu item reachable by keyboard and screen reader. Screen-reader users get the same actions as **custom actions** (iOS/macOS `accessibilityActions`, Android `customActions`, Windows `AutomationProperties` + context menu, GTK accessible actions, Web `aria-*` + menu button).

## Focus management

- Opening a sheet/dialog moves focus to its first field (or title for confirms); closing returns focus to the invoker.
- After a destructive action removes a row, focus moves to the next row (or the list heading if empty).
- Route changes move focus to the page heading and announce the page title.

## Notifications

- The unread count is part of the Inbox item's accessible name ("Inbox, 3 unread"), not color or a dot alone.
- New notifications arriving while the app is open are announced politely once ("New notification from Timetrack"), never repeatedly; security alerts use assertive announcement only when the Inbox is focused.
- Notification actions (Accept, Review sessions) are also available inside NTF-02, so people who dismiss system notifications or use switch access can reach them.
