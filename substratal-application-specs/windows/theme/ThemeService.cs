// ThemeService.cs — applies Settings.theme + the user's light/dark theme choice (generated ids match shared/themes/themes.json)
using Microsoft.UI.Xaml;
using Microsoft.UI.Composition.SystemBackdrops;

namespace Substratal.Hub.Theming;

public enum ThemeMode { Light, Dark, System }
public enum LightTheme { TealLedger, IndigoRoot, MossPaper, CoralClay, SporePink }
public enum DarkTheme { NightSoil, DeepTeal, GraphiteSpore, ForestFloor, EmberClay }

public sealed class ThemeService
{
    public static readonly LightTheme DefaultLight = LightTheme.MossPaper;
    public static readonly DarkTheme DefaultDark = DarkTheme.ForestFloor;

    /// <summary>Kebab-case ids as stored in app_hub settings (appearance.light_theme / appearance.dark_theme).</summary>
    public static string Id(System.Enum e) => System.Text.RegularExpressions.Regex.Replace(e.ToString(), "(?<!^)([A-Z])", "-$1").ToLowerInvariant();
    public static T Parse<T>(string? id, T fallback) where T : struct, System.Enum
    {
        foreach (var v in System.Enum.GetValues<T>()) if (Id(v) == id) return v;
        return fallback;   // unknown or missing → default
    }

    public void Apply(FrameworkElement root, ThemeMode mode, LightTheme light, DarkTheme dark)
    {
        var themes = Application.Current.Resources.MergedDictionaries[0].ThemeDictionaries;  // SubstratalThemes.xaml
        themes["Light"] = Load($"Light/{light}");
        themes["Dark"] = Load($"Dark/{dark}");
        root.RequestedTheme = mode switch { ThemeMode.Light => ElementTheme.Light, ThemeMode.Dark => ElementTheme.Dark, _ => ElementTheme.Default };
        // Force re-evaluation of ThemeResource lookups:
        var current = root.RequestedTheme;
        root.RequestedTheme = current == ElementTheme.Dark ? ElementTheme.Light : ElementTheme.Dark;
        root.RequestedTheme = current;
    }

    static ResourceDictionary Load(string name) =>
        new() { Source = new System.Uri($"ms-appx:///Theming/Themes/{name}.xaml") };

    /// <summary>Tint Mica with the theme background so title bar and panes match.</summary>
    public static void TintBackdrop(MicaController mica, Windows.UI.Color background)
    {
        mica.TintColor = background; mica.TintOpacity = 0.8f; mica.LuminosityOpacity = 0.9f;
    }
}
