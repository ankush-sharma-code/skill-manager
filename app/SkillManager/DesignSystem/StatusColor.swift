import SwiftUI

enum StatusKind: Equatable, Hashable {
    case ok
    case attention
    case conflict

    var title: String {
        switch self {
        case .ok: "In sync"
        case .attention: "Needs a look"
        case .conflict: "Conflict"
        }
    }
}

struct StatusPaint: ShapeStyle {
    var kind: StatusKind
    var opacity: Double = 1

    func resolve(in environment: EnvironmentValues) -> Color {
        let scheme = environment.colorScheme
        let base: Color
        switch kind {
        case .ok:
            base = scheme == .dark
                ? Color(red: 0.188, green: 0.820, blue: 0.345)
                : Color(red: 0.204, green: 0.780, blue: 0.349)
        case .attention:
            base = scheme == .dark
                ? Color(red: 1.0, green: 0.702, blue: 0.251)
                : Color(red: 1.0, green: 0.624, blue: 0.039)
        case .conflict:
            base = scheme == .dark
                ? Color(red: 1.0, green: 0.271, blue: 0.227)
                : Color(red: 1.0, green: 0.231, blue: 0.188)
        }
        return base.opacity(opacity)
    }
}

enum StatusColor {
    static func paint(_ kind: StatusKind, opacity: Double = 1) -> StatusPaint {
        StatusPaint(kind: kind, opacity: opacity)
    }
}
