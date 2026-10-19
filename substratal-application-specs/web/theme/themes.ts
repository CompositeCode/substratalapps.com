// themes.ts — theme resolution for the web app (generated ids match shared/themes/themes.json)
export const LIGHT_THEMES = ["teal-ledger", "indigo-root", "moss-paper", "coral-clay", "spore-pink"] as const;
export const DARK_THEMES = ["night-soil", "deep-teal", "graphite-spore", "forest-floor", "ember-clay"] as const;
export type LightTheme = typeof LIGHT_THEMES[number];
export type DarkTheme = typeof DARK_THEMES[number];
export type ThemeMode = "light" | "dark" | "system";               // Settings.theme
export const DEFAULT_LIGHT: LightTheme = "moss-paper";
export const DEFAULT_DARK: DarkTheme = "forest-floor";
export const THEME_NAMES: Record<LightTheme | DarkTheme, string> = {"teal-ledger": "Teal Ledger", "indigo-root": "Indigo Root", "moss-paper": "Moss Paper", "coral-clay": "Coral Clay", "spore-pink": "Spore Pink", "night-soil": "Night Soil", "deep-teal": "Deep Teal", "graphite-spore": "Graphite Spore", "forest-floor": "Forest Floor", "ember-clay": "Ember Clay"};
const BACKGROUND: Record<LightTheme | DarkTheme, string> = {"teal-ledger": "#F3F6F6", "indigo-root": "#F4F4FA", "moss-paper": "#F5F4EE", "coral-clay": "#F9F5F2", "spore-pink": "#F9F6F8", "night-soil": "#15132E", "deep-teal": "#0D1819", "graphite-spore": "#161618", "forest-floor": "#10150F", "ember-clay": "#191412"};

export interface ThemePrefs { mode: ThemeMode; light: LightTheme; dark: DarkTheme }
export function normalize(p: Partial<{ mode: string; light: string; dark: string }>): ThemePrefs {
  return {
    mode: (["light", "dark", "system"] as const).includes(p.mode as ThemeMode) ? (p.mode as ThemeMode) : "system",
    light: (LIGHT_THEMES as readonly string[]).includes(p.light ?? "") ? (p.light as LightTheme) : DEFAULT_LIGHT,
    dark: (DARK_THEMES as readonly string[]).includes(p.dark ?? "") ? (p.dark as DarkTheme) : DEFAULT_DARK,
  };
}

const mq = typeof matchMedia !== "undefined" ? matchMedia("(prefers-color-scheme: dark)") : undefined;

/** Apply to <html>. Call from an inline script in <head> (cached prefs) to avoid a flash, then again after settings load. */
export function applyTheme(p: ThemePrefs): () => void {
  const set = () => {
    const dark = p.mode === "dark" || (p.mode === "system" && !!mq?.matches);
    const r = document.documentElement;
    r.dataset.scheme = dark ? "dark" : "light";
    r.dataset.lightTheme = p.light;
    r.dataset.darkTheme = p.dark;
    document.querySelector('meta[name="theme-color"]')?.setAttribute("content", BACKGROUND[dark ? p.dark : p.light]);
  };
  set();
  mq?.addEventListener("change", set);
  try { localStorage.setItem("sub.theme", JSON.stringify(p)); } catch {}
  return () => mq?.removeEventListener("change", set);
}
