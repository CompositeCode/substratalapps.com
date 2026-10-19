// SubstratalTheme.swift — generated from shared/themes/themes.json. Lives in SubstratalKit (shared by iOS and macOS).
// Five light and five dark themes. Defaults: Moss Paper (light) and Forest Floor (dark).
import SwiftUI
import Observation

public struct ThemePalette: Equatable, Sendable {
    public let background: Color
    public let sidebar: Color
    public let surface: Color
    public let raised: Color
    public let text: Color
    public let textSecondary: Color
    public let textTertiary: Color
    public let separator: Color
    public let accent: Color
    public let accentStrong: Color
    public let onAccent: Color
    public let accentSubtle: Color
    public let link: Color
    public let selection: Color
    public let controlFill: Color
    public let controlFillStrong: Color
    public let focusRing: Color
}

/// Settings.theme from the API (`PATCH /users/me/settings`).
public enum ThemeMode: String, Codable, CaseIterable, Sendable { case light, dark, system }

public enum LightTheme: String, Codable, CaseIterable, Identifiable, Sendable {
    case tealLedger = "teal-ledger"
    case indigoRoot = "indigo-root"
    case mossPaper = "moss-paper"
    case coralClay = "coral-clay"
    case sporePink = "spore-pink"
    public static let `default`: LightTheme = .mossPaper
    public var id: String { rawValue }
    public var displayName: String {
        switch self {
        case .tealLedger: "Teal Ledger"
        case .indigoRoot: "Indigo Root"
        case .mossPaper: "Moss Paper"
        case .coralClay: "Coral Clay"
        case .sporePink: "Spore Pink"
        }
    }
    public var palette: ThemePalette {
        switch self {
        case .tealLedger: ThemePalette(background: Color(hex: 0xF3F6F6), sidebar: Color(hex: 0xEAF0F0), surface: Color(hex: 0xF6F9F9), raised: Color(hex: 0xFFFFFF), text: Color(hex: 0x10272A), textSecondary: Color(hex: 0x56686A), textTertiary: Color(hex: 0x9BA4A6), separator: Color(hex: 0xD9E2E2), accent: Color(hex: 0x137C7A), accentStrong: Color(hex: 0x0E5F5D), onAccent: Color(hex: 0xFFFFFF), accentSubtle: Color(hex: 0xDDF0EE), link: Color(hex: 0x137C7A), selection: Color(hex: 0xDDF0EE), controlFill: Color(hex: 0xEEF0F0), controlFillStrong: Color(hex: 0xE2E5E5), focusRing: Color(hex: 0x137C7A))
        case .indigoRoot: ThemePalette(background: Color(hex: 0xF4F4FA), sidebar: Color(hex: 0xEBEBF5), surface: Color(hex: 0xF7F7FC), raised: Color(hex: 0xFFFFFF), text: Color(hex: 0x1A1838), textSecondary: Color(hex: 0x5D5B78), textTertiary: Color(hex: 0x9F9EAB), separator: Color(hex: 0xDEDEEB), accent: Color(hex: 0x3D3BB8), accentStrong: Color(hex: 0x2C2A8F), onAccent: Color(hex: 0xFFFFFF), accentSubtle: Color(hex: 0xE6E6FA), link: Color(hex: 0x3D3BB8), selection: Color(hex: 0xE6E6FA), controlFill: Color(hex: 0xEFEFF1), controlFillStrong: Color(hex: 0xE4E3E7), focusRing: Color(hex: 0x3D3BB8))
        case .mossPaper: ThemePalette(background: Color(hex: 0xF5F4EE), sidebar: Color(hex: 0xEDECE3), surface: Color(hex: 0xF9F8F3), raised: Color(hex: 0xFFFFFE), text: Color(hex: 0x22261A), textSecondary: Color(hex: 0x5F6455), textTertiary: Color(hex: 0xA2A49E), separator: Color(hex: 0xE1DFD2), accent: Color(hex: 0x4A7322), accentStrong: Color(hex: 0x375719), onAccent: Color(hex: 0xFFFFFF), accentSubtle: Color(hex: 0xE6EFD8), link: Color(hex: 0x4A7322), selection: Color(hex: 0xE6EFD8), controlFill: Color(hex: 0xF0F0EE), controlFillStrong: Color(hex: 0xE4E5E3), focusRing: Color(hex: 0x4A7322))
        case .coralClay: ThemePalette(background: Color(hex: 0xF9F5F2), sidebar: Color(hex: 0xF1EBE6), surface: Color(hex: 0xFBF8F6), raised: Color(hex: 0xFFFFFF), text: Color(hex: 0x2A1D17), textSecondary: Color(hex: 0x6E5D55), textTertiary: Color(hex: 0xA6A09E), separator: Color(hex: 0xEAE0D8), accent: Color(hex: 0xB44529), accentStrong: Color(hex: 0x90361F), onAccent: Color(hex: 0xFFFFFF), accentSubtle: Color(hex: 0xF7E3DB), link: Color(hex: 0xB44529), selection: Color(hex: 0xF7E3DB), controlFill: Color(hex: 0xF0EFEF), controlFillStrong: Color(hex: 0xE5E4E3), focusRing: Color(hex: 0xB44529))
        case .sporePink: ThemePalette(background: Color(hex: 0xF9F6F8), sidebar: Color(hex: 0xF2ECF0), surface: Color(hex: 0xFBF8FA), raised: Color(hex: 0xFFFFFF), text: Color(hex: 0x2A1622), textSecondary: Color(hex: 0x6E5A66), textTertiary: Color(hex: 0xA69DA2), separator: Color(hex: 0xEADFE5), accent: Color(hex: 0xB3285B), accentStrong: Color(hex: 0x8F1F48), onAccent: Color(hex: 0xFFFFFF), accentSubtle: Color(hex: 0xF7E0E9), link: Color(hex: 0xB3285B), selection: Color(hex: 0xF7E0E9), controlFill: Color(hex: 0xF0EFF0), controlFillStrong: Color(hex: 0xE5E3E4), focusRing: Color(hex: 0xB3285B))
        }
    }
}

public enum DarkTheme: String, Codable, CaseIterable, Identifiable, Sendable {
    case nightSoil = "night-soil"
    case deepTeal = "deep-teal"
    case graphiteSpore = "graphite-spore"
    case forestFloor = "forest-floor"
    case emberClay = "ember-clay"
    public static let `default`: DarkTheme = .forestFloor
    public var id: String { rawValue }
    public var displayName: String {
        switch self {
        case .nightSoil: "Night Soil"
        case .deepTeal: "Deep Teal"
        case .graphiteSpore: "Graphite Spore"
        case .forestFloor: "Forest Floor"
        case .emberClay: "Ember Clay"
        }
    }
    public var palette: ThemePalette {
        switch self {
        case .nightSoil: ThemePalette(background: Color(hex: 0x15132E), sidebar: Color(hex: 0x1B1938), surface: Color(hex: 0x211F42), raised: Color(hex: 0x25234B), text: Color(hex: 0xECEBFA), textSecondary: Color(hex: 0xA6A4C8), textTertiary: Color(hex: 0x797794), separator: Color(hex: 0x34325C), accent: Color(hex: 0x8E8CFF), accentStrong: Color(hex: 0xB4B3FF), onAccent: Color(hex: 0x15132E), accentSubtle: Color(hex: 0x2C2A63), link: Color(hex: 0x8E8CFF), selection: Color(hex: 0x2C2A63), controlFill: Color(hex: 0x333157), controlFillStrong: Color(hex: 0x3D3B60), focusRing: Color(hex: 0x8E8CFF))
        case .deepTeal: ThemePalette(background: Color(hex: 0x0D1819), sidebar: Color(hex: 0x122021), surface: Color(hex: 0x162729), raised: Color(hex: 0x1A2E30), text: Color(hex: 0xE3F1F0), textSecondary: Color(hex: 0x93ADAC), textTertiary: Color(hex: 0x6E8081), separator: Color(hex: 0x26403F), accent: Color(hex: 0x55CFC9), accentStrong: Color(hex: 0x8BE3DE), onAccent: Color(hex: 0x062423), accentSubtle: Color(hex: 0x133C3B), link: Color(hex: 0x55CFC9), selection: Color(hex: 0x133C3B), controlFill: Color(hex: 0x283C3D), controlFillStrong: Color(hex: 0x324547), focusRing: Color(hex: 0x55CFC9))
        case .graphiteSpore: ThemePalette(background: Color(hex: 0x161618), sidebar: Color(hex: 0x1D1D20), surface: Color(hex: 0x222226), raised: Color(hex: 0x28282C), text: Color(hex: 0xF1EFF1), textSecondary: Color(hex: 0xA3A0A7), textTertiary: Color(hex: 0x7C7C7F), separator: Color(hex: 0x34343A), accent: Color(hex: 0xFF86AE), accentStrong: Color(hex: 0xFFB0CA), onAccent: Color(hex: 0x3A0A1E), accentSubtle: Color(hex: 0x3E2030), link: Color(hex: 0xFF86AE), selection: Color(hex: 0x3E2030), controlFill: Color(hex: 0x36363A), controlFillStrong: Color(hex: 0x404044), focusRing: Color(hex: 0xFF86AE))
        case .forestFloor: ThemePalette(background: Color(hex: 0x10150F), sidebar: Color(hex: 0x151C14), surface: Color(hex: 0x1A2219), raised: Color(hex: 0x1F291E), text: Color(hex: 0xE8EFE2), textSecondary: Color(hex: 0x9DAA95), textTertiary: Color(hex: 0x737C70), separator: Color(hex: 0x2C3929), accent: Color(hex: 0xB8E26A), accentStrong: Color(hex: 0xD3F09A), onAccent: Color(hex: 0x1A2608), accentSubtle: Color(hex: 0x2C3B19), link: Color(hex: 0xB8E26A), selection: Color(hex: 0x2C3B19), controlFill: Color(hex: 0x2D372C), controlFillStrong: Color(hex: 0x374136), focusRing: Color(hex: 0xB8E26A))
        case .emberClay: ThemePalette(background: Color(hex: 0x191412), sidebar: Color(hex: 0x201916), surface: Color(hex: 0x261E1A), raised: Color(hex: 0x2C231F), text: Color(hex: 0xF4ECE8), textSecondary: Color(hex: 0xB3A39B), textTertiary: Color(hex: 0x807773), separator: Color(hex: 0x3B302B), accent: Color(hex: 0xFF9A80), accentStrong: Color(hex: 0xFFBCA9), onAccent: Color(hex: 0x3D1206), accentSubtle: Color(hex: 0x44261D), link: Color(hex: 0xFF9A80), selection: Color(hex: 0x44261D), controlFill: Color(hex: 0x3A312D), controlFillStrong: Color(hex: 0x443B37), focusRing: Color(hex: 0xFF9A80))
        }
    }
}

/// Holds the user's choice. Persist `light`/`dark` to the app_hub settings keys
/// `appearance.light_theme` / `appearance.dark_theme` and cache them locally for launch.
@MainActor @Observable public final class ThemeStore {
    public var mode: ThemeMode = .system
    public var light: LightTheme = .default
    public var dark: DarkTheme = .default
    public init() {}
    /// Unknown ids from the server fall back to the defaults.
    public func apply(mode: String?, light: String?, dark: String?) {
        self.mode = mode.flatMap(ThemeMode.init(rawValue:)) ?? .system
        self.light = light.flatMap(LightTheme.init(rawValue:)) ?? .default
        self.dark = dark.flatMap(DarkTheme.init(rawValue:)) ?? .default
    }
    public var preferredColorScheme: ColorScheme? { mode == .light ? .light : mode == .dark ? .dark : nil }
    public func palette(for scheme: ColorScheme) -> ThemePalette { scheme == .dark ? dark.palette : light.palette }
}

private struct PaletteKey: EnvironmentKey { static let defaultValue = LightTheme.default.palette }
public extension EnvironmentValues { var palette: ThemePalette { get { self[PaletteKey.self] } set { self[PaletteKey.self] = newValue } } }

/// Apply once at the root: `ContentView().substratalTheme(store)`.
public struct SubstratalThemeModifier: ViewModifier {
    @Environment(\.colorScheme) private var scheme
    @Environment(\.colorSchemeContrast) private var contrast
    let store: ThemeStore
    public func body(content: Content) -> some View {
        let p = store.palette(for: scheme)
        content
            .environment(\.palette, p)
            .tint(contrast == .increased ? p.accentStrong : p.accent)
            .preferredColorScheme(store.preferredColorScheme)
    }
}
public extension View { func substratalTheme(_ store: ThemeStore) -> some View { modifier(SubstratalThemeModifier(store: store)) } }

extension Color {
    init(hex: UInt32) {
        self.init(.sRGB, red: Double((hex >> 16) & 0xFF) / 255, green: Double((hex >> 8) & 0xFF) / 255, blue: Double(hex & 0xFF) / 255, opacity: 1)
    }
}
