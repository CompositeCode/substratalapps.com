# Linux theme files

`light/<id>.css` and `dark/<id>.css` redefine libadwaita's named colours (`accent_bg_color`, `window_bg_color`, `view_bg_color`, `sidebar_bg_color`, `card_bg_color` …) plus `sub_*` role colours.

Defaults: `light/moss-paper.css` and `dark/forest-floor.css`.

**How to apply them:**

1. Bundle both folders in the GResource as `/com/substratalapps/Hub/theme/`.
2. Keep one `Gtk.CssProvider` at `GTK_STYLE_PROVIDER_PRIORITY_APPLICATION`.
3. On startup, and on every `notify::dark` from `AdwStyleManager`, load `{dark ? dark/<dark_theme> : light/<light_theme>}.css` into that provider.
4. Map `Settings.theme` to `AdwStyleManager:color-scheme`:
   - `light` → `FORCE_LIGHT`
   - `dark` → `FORCE_DARK`
   - `system` → `PREFER_LIGHT`, which follows the system setting
5. When `AdwStyleManager:high-contrast` is true, do not load a theme file, so the system's high-contrast palette applies.

```rust
let provider = gtk::CssProvider::new();
gtk::style_context_add_provider_for_display(&display, &provider, gtk::STYLE_PROVIDER_PRIORITY_APPLICATION);
let apply = move |sm: &adw::StyleManager| {
    if sm.is_high_contrast() { provider.load_from_string(""); return; }
    let path = if sm.is_dark() { format!("/com/substratalapps/Hub/theme/dark/{}.css", dark_id) }
               else { format!("/com/substratalapps/Hub/theme/light/{}.css", light_id) };
    provider.load_from_resource(&path);
};
```
