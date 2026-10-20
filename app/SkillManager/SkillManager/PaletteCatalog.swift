import Foundation

enum PaletteCatalog {
    static func groups(skills: [SkillFixture], query: String) -> [PaletteGroup] {
        let trimmed = query.trimmingCharacters(in: .whitespacesAndNewlines)
        func matches(_ text: String) -> Bool {
            trimmed.isEmpty || text.localizedCaseInsensitiveContains(trimmed)
        }

        let skillEntries = skills
            .sorted { $0.usesThisWeek > $1.usesThisWeek }
            .filter { matches($0.name) || matches($0.summary) }
            .prefix(trimmed.isEmpty ? 5 : 8)
            .map { skill in
                PaletteEntry(
                    id: skill.id,
                    kind: .skill,
                    title: skill.name,
                    subtitle: skill.summary,
                    symbol: "doc.text",
                    shortcut: nil
                )
            }

        let targetEntries = [
            ("claude-user", "Claude Code", "~/.claude/skills"),
            ("cursor-user", "Cursor", "~/.cursor/skills"),
            ("codex-user", "Codex", "~/.codex/skills")
        ]
        .filter { matches($0.1) || matches($0.2) }
        .map { item in
            PaletteEntry(
                id: item.0,
                kind: .target,
                title: item.1,
                subtitle: item.2,
                symbol: "folder",
                shortcut: nil
            )
        }

        let actionEntries = [
            PaletteEntry(
                id: "action-sync",
                kind: .action,
                title: "Sync now",
                subtitle: "Write skills out to each tool",
                symbol: "arrow.clockwise",
                shortcut: "⌘R"
            ),
            PaletteEntry(
                id: "action-add",
                kind: .action,
                title: "Add skill",
                subtitle: "Create or import a skill",
                symbol: "plus",
                shortcut: "⌘N"
            ),
            PaletteEntry(
                id: "action-setup",
                kind: .action,
                title: "Run setup again",
                subtitle: "Detect tools and choose a mode",
                symbol: "checklist",
                shortcut: nil
            ),
            PaletteEntry(
                id: "action-settings",
                kind: .action,
                title: "Settings",
                subtitle: "Launch at login and the idle threshold",
                symbol: "gearshape",
                shortcut: "⌘,"
            )
        ]
        .filter { matches($0.title) || matches($0.subtitle) }

        return [
            PaletteGroup(title: "Skills", entries: Array(skillEntries)),
            PaletteGroup(title: "Targets", entries: targetEntries),
            PaletteGroup(title: "Actions", entries: actionEntries)
        ]
        .filter { !$0.entries.isEmpty }
    }
}
