import Foundation

enum SidebarSection: String, CaseIterable, Identifiable, Hashable {
    case skills
    case conflicts
    case targets
    case usage
    case settings

    var id: String { rawValue }

    var title: String {
        switch self {
        case .skills: "Skills"
        case .conflicts: "Conflicts"
        case .targets: "Targets"
        case .usage: "Usage"
        case .settings: "Settings"
        }
    }

    var symbol: String {
        switch self {
        case .skills: "square.stack.3d.up"
        case .conflicts: "exclamationmark.triangle"
        case .targets: "arrow.triangle.branch"
        case .usage: "chart.bar"
        case .settings: "gearshape"
        }
    }

    var emptyTitle: String {
        switch self {
        case .skills: "No skills yet"
        case .conflicts: "No open conflicts"
        case .targets: "No tools yet"
        case .usage: "No usage yet"
        case .settings: "Settings"
        }
    }

    var emptyMessage: String {
        switch self {
        case .skills: "Add a skill once. Skill Manager syncs it to every tool you turned on."
        case .conflicts: "When a tool's copy drifts from the folder, it shows up here."
        case .targets: "Run setup to detect Claude Code, Cursor, and Codex."
        case .usage: "Usage appears after a tool reads a skill."
        case .settings: "Launch at login, the idle threshold, and setup live here."
        }
    }
}

enum SkillFlag: String, Equatable, Hashable {
    case invalid
    case conflict
    case idle

    var title: String {
        switch self {
        case .invalid: "Invalid"
        case .conflict: "Conflict"
        case .idle: "Idle"
        }
    }

    var kind: StatusKind {
        switch self {
        case .invalid, .idle: .attention
        case .conflict: .conflict
        }
    }
}

struct SkillChip: Identifiable, Equatable, Hashable {
    var key: String
    var value: String
    var id: String { key }
}

struct TargetSlot: Identifiable, Equatable, Hashable {
    var tool: String
    var state: TargetSyncState
    var id: String { tool }
}

enum ReaderBlock: Equatable, Identifiable {
    case heading(String)
    case paragraph(String)
    case bullets([String])
    case code(language: String, source: String)

    var id: String {
        switch self {
        case let .heading(text):
            "h-\(text)"
        case let .paragraph(text):
            "p-\(text)"
        case let .bullets(items):
            "b-\(items.joined(separator: "|"))"
        case let .code(language, source):
            "c-\(language)-\(source)"
        }
    }
}

struct SkillFixture: Identifiable, Equatable, Hashable {
    var name: String
    var summary: String
    var detail: String
    var chips: [SkillChip]
    var flags: [SkillFlag]
    var targets: [TargetSlot]
    var usesThisWeek: Int
    var lastUsed: String
    var series: [Int]
    var body: [ReaderBlock]
    var id: String { name }
}

enum SyncModeChoice: String, CaseIterable, Identifiable, Hashable {
    case symlink
    case copy

    var id: String { rawValue }
    var title: String { self == .symlink ? "Symlink" : "Copy" }
}

struct SyncTargetOption: Identifiable, Equatable, Hashable {
    var id: String
    var name: String
    var scope: String
    var mode: SyncModeChoice
}

struct DetectedTool: Identifiable, Equatable {
    var id: String
    var name: String
    var path: String
    var detected: Bool
    var warning: String?
}
