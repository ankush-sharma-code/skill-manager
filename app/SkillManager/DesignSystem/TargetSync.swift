import Foundation

enum TargetSyncState: String, Equatable, Hashable {
    case inSync
    case drift
    case missing
    case conflict

    var kind: StatusKind {
        switch self {
        case .inSync: .ok
        case .drift, .missing: .attention
        case .conflict: .conflict
        }
    }

    var label: String {
        switch self {
        case .inSync: "In sync"
        case .drift: "Drift"
        case .missing: "Missing"
        case .conflict: "Conflict"
        }
    }
}
