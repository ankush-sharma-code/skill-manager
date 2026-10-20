import Foundation

enum SidebarSection: String, CaseIterable, Identifiable, Hashable {
    case all
    case conflicts
    case idle
    case tools
    case usage

    var id: String { rawValue }

    var title: String {
        switch self {
        case .all: "All"
        case .conflicts: "Conflicts"
        case .idle: "Idle"
        case .tools: "Tools"
        case .usage: "Usage"
        }
    }

    var symbol: String {
        switch self {
        case .all: "square.stack.3d.up"
        case .conflicts: "exclamationmark.triangle"
        case .idle: "moon.zzz"
        case .tools: "wrench.and.screwdriver"
        case .usage: "chart.bar"
        }
    }

    var emptyTitle: String {
        switch self {
        case .all: "No skills yet"
        case .conflicts: "No open conflicts"
        case .idle: "No idle skills"
        case .tools: "No tools yet"
        case .usage: "No usage yet"
        }
    }

    var emptyMessage: String {
        switch self {
        case .all: "Add a skill once. Skill Manager syncs it to every tool you turned on."
        case .conflicts: "When a tool's copy drifts from the folder, it shows up here."
        case .idle: "A skill lands here after it goes unused past the idle threshold."
        case .tools: "Run setup to detect Claude Code, Cursor, and Codex."
        case .usage: "Usage appears after a tool reads a skill."
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

struct SkillFixture: Identifiable, Equatable {
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
