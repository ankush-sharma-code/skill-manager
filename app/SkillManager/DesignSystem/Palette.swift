import SwiftUI

struct SurfacePaint: ShapeStyle {
    enum Tone {
        case sidebar
        case content
        case page
        case elevated
        case code
        case stroke
        case scrim
        case search
        case chip
        case track
    }

    var tone: Tone

    func resolve(in environment: EnvironmentValues) -> Color {
        let dark = environment.colorScheme == .dark
        switch tone {
        case .sidebar:
            return dark ? Color(red: 0.09, green: 0.09, blue: 0.11) : Color(red: 0.933, green: 0.933, blue: 0.953)
        case .content:
            return dark ? Color(red: 0.12, green: 0.12, blue: 0.14) : Color(red: 0.965, green: 0.965, blue: 0.973)
        case .page:
            return dark ? Color(red: 0.145, green: 0.149, blue: 0.176) : Color(red: 0.992, green: 0.992, blue: 0.996)
        case .elevated:
            return dark ? Color(red: 0.18, green: 0.185, blue: 0.21) : Color.white
        case .code:
            return dark ? Color(red: 0.10, green: 0.11, blue: 0.13) : Color(red: 0.945, green: 0.948, blue: 0.96)
        case .stroke:
            return dark ? Color.white.opacity(0.10) : Color.black.opacity(0.08)
        case .scrim:
            return dark ? Color.black.opacity(0.55) : Color(red: 0.10, green: 0.12, blue: 0.16).opacity(0.28)
        case .search:
            return dark ? Color.white.opacity(0.06) : Color.white
        case .chip:
            return dark ? Color.white.opacity(0.06) : Color.black.opacity(0.04)
        case .track:
            return dark ? Color.white.opacity(0.12) : Color.black.opacity(0.10)
        }
    }
}

struct AccentWash: ShapeStyle {
    func resolve(in environment: EnvironmentValues) -> some ShapeStyle {
        Color.accentColor.opacity(environment.colorScheme == .dark ? 0.32 : 0.16)
    }
}

struct HoverWash: ShapeStyle {
    func resolve(in environment: EnvironmentValues) -> some ShapeStyle {
        Color.primary.opacity(environment.colorScheme == .dark ? 0.08 : 0.05)
    }
}

enum Palette {
    static let sidebar = SurfacePaint(tone: .sidebar)
    static let content = SurfacePaint(tone: .content)
    static let page = SurfacePaint(tone: .page)
    static let elevated = SurfacePaint(tone: .elevated)
    static let codeBackground = SurfacePaint(tone: .code)
    static let stroke = SurfacePaint(tone: .stroke)
    static let scrim = SurfacePaint(tone: .scrim)
    static let search = SurfacePaint(tone: .search)
    static let chip = SurfacePaint(tone: .chip)
    static let track = SurfacePaint(tone: .track)
    static let selection = AccentWash()
    static let hover = HoverWash()

    static func shadow(_ scheme: ColorScheme) -> Color {
        scheme == .dark ? Color.black.opacity(0.5) : Color.black.opacity(0.16)
    }
}
