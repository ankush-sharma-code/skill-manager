import Foundation

enum FixtureLibrary {
    static let lastSync = "2 minutes ago"

    static let collisionClaude = "Write the failing test first."
    static let collisionCursor = "Add a regression test when the bug has a local target."

    static let detectedTools: [DetectedTool] = [
        DetectedTool(id: "claude", name: "Claude Code", path: "~/.claude/skills", detected: true, warning: nil),
        DetectedTool(id: "cursor", name: "Cursor", path: "~/.cursor/skills", detected: true, warning: nil),
        DetectedTool(
            id: "codex",
            name: "Codex",
            path: "~/.codex/skills",
            detected: true,
            warning: "Path moved from ~/.agents/skills"
        )
    ]

    static let syncTargets: [SyncTargetOption] = [
        SyncTargetOption(id: "claude-user", name: "Claude Code", scope: "This Mac", mode: .symlink),
        SyncTargetOption(id: "cursor-user", name: "Cursor", scope: "This Mac", mode: .symlink),
        SyncTargetOption(id: "codex-user", name: "Codex", scope: "This Mac", mode: .symlink),
        SyncTargetOption(id: "project", name: "This project", scope: "Project", mode: .copy)
    ]

    static let skills: [SkillFixture] = [
        make(
            name: "poteto-mode",
            summary: "Agent style for concise, verified work.",
            detail: "Route a request to one playbook, then check the result against the running app.",
            chips: [
                SkillChip(key: "name", value: "poteto-mode"),
                SkillChip(key: "description", value: "Concise agent style"),
                SkillChip(key: "tools", value: "cursor"),
                SkillChip(key: "version", value: "1")
            ],
            flags: [],
            targets: slots(claude: .inSync, cursor: .inSync, codex: .inSync),
            uses: 128,
            lastUsed: "2h ago",
            series: [2, 4, 3, 6, 8, 1, 0, 3, 5, 7, 9, 4, 1, 0, 4, 6, 8, 11, 5, 2, 1, 5, 7, 6, 10, 8, 3, 2, 9, 12],
            body: potetoBody
        ),
        make(
            name: "architect",
            summary: "Sketch the types before the implementation.",
            detail: "Two shapes get compared, then one is filled in.",
            chips: [
                SkillChip(key: "name", value: "architect"),
                SkillChip(key: "description", value: "Sketch types first"),
                SkillChip(key: "tools", value: "cursor")
            ],
            flags: [],
            targets: slots(claude: .inSync, cursor: .inSync, codex: .inSync),
            uses: 64,
            lastUsed: "5h ago",
            series: [1, 2, 2, 4, 3, 0, 0, 2, 3, 4, 5, 2, 1, 0, 2, 3, 4, 6, 3, 1, 1, 2, 4, 3, 5, 4, 2, 1, 4, 6],
            body: [
                .paragraph("Name the data shape, compare two structures, and only then fill in the code.")
            ]
        ),
        make(
            name: "arena",
            summary: "Compare candidates, then keep one.",
            detail: "Parallel attempts stay separate until a base is chosen.",
            chips: [
                SkillChip(key: "name", value: "arena"),
                SkillChip(key: "description", value: "Compare, then graft"),
                SkillChip(key: "tools", value: "cursor")
            ],
            flags: [],
            targets: slots(claude: .inSync, cursor: .inSync, codex: .drift),
            uses: 41,
            lastUsed: "Yesterday",
            series: [0, 1, 2, 2, 3, 1, 0, 1, 2, 3, 2, 1, 0, 0, 2, 2, 3, 4, 2, 1, 0, 1, 2, 3, 4, 2, 1, 1, 3, 4],
            body: [
                .paragraph("Each candidate writes to its own folder. The base is chosen after both exist.")
            ]
        ),
        make(
            name: "tdd",
            summary: "A failing test before the fix.",
            detail: "Use it when the bug has a local test target.",
            chips: [
                SkillChip(key: "name", value: "tdd"),
                SkillChip(key: "description", value: "Failing test first"),
                SkillChip(key: "tools", value: "cursor")
            ],
            flags: [.conflict],
            targets: slots(claude: .inSync, cursor: .conflict, codex: .drift),
            uses: 22,
            lastUsed: "3d ago",
            series: [1, 0, 1, 2, 1, 0, 0, 1, 1, 2, 1, 0, 0, 0, 1, 2, 1, 2, 1, 0, 0, 1, 1, 0, 2, 1, 0, 1, 1, 2],
            body: [
                .paragraph("Write the test that fails, then the change that makes it pass.")
            ]
        ),
        make(
            name: "importer",
            summary: "Adopt skills that already live in a tool.",
            detail: "Identical copies collapse. A name collision waits for a choice.",
            chips: [
                SkillChip(key: "name", value: "importer"),
                SkillChip(key: "description", value: "Adopt existing skills"),
                SkillChip(key: "tools", value: "cursor")
            ],
            flags: [.conflict],
            targets: slots(claude: .conflict, cursor: .conflict, codex: .inSync),
            uses: 11,
            lastUsed: "6d ago",
            series: [0, 1, 0, 1, 1, 0, 0, 0, 1, 1, 0, 0, 0, 0, 1, 0, 1, 1, 0, 0, 0, 1, 0, 1, 1, 0, 0, 0, 1, 1],
            body: [
                .paragraph("Scan the tools, group by digest, and write nothing until you confirm.")
            ]
        ),
        make(
            name: "unslop",
            summary: "Cut stock phrasing from the reply.",
            detail: "Short sentences, plain words, and no filler.",
            chips: [
                SkillChip(key: "name", value: "unslop"),
                SkillChip(key: "description", value: "Plain prose"),
                SkillChip(key: "tools", value: "cursor")
            ],
            flags: [.idle],
            targets: slots(claude: .inSync, cursor: .inSync, codex: .inSync),
            uses: 4,
            lastUsed: "36d ago",
            series: [1, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0],
            body: [
                .paragraph("Remove a stock phrase when a plain sentence says the same thing.")
            ]
        ),
        make(
            name: "how",
            summary: "Explain a subsystem before changing it.",
            detail: "Read the path a request takes, then name the owner.",
            chips: [
                SkillChip(key: "name", value: "how"),
                SkillChip(key: "description", value: "Explain the path"),
                SkillChip(key: "tools", value: "cursor")
            ],
            flags: [],
            targets: slots(claude: .inSync, cursor: .inSync, codex: .missing),
            uses: 18,
            lastUsed: "1d ago",
            series: [0, 1, 1, 2, 1, 0, 0, 1, 1, 2, 1, 0, 0, 0, 1, 1, 2, 2, 1, 0, 0, 1, 1, 1, 2, 1, 0, 1, 1, 2],
            body: [
                .paragraph("Follow one request from the entry point to the type that owns the decision.")
            ]
        )
    ]

    static func syncedSlots(in skills: [SkillFixture]) -> Int {
        skills.flatMap(\.targets).filter { $0.state == .inSync }.count
    }

    private static let potetoBody: [ReaderBlock] = [
        .heading("What it does"),
        .paragraph("Route a request to one playbook. Keep the reply specific, and check the result against the running app."),
        .heading("Use it for"),
        .bullets([
            "A new feature, from the data shape through the pull request.",
            "A visual choice, with two directions before one stays."
        ]),
        .heading("Motion"),
        .paragraph("Springs stay at or under 0.4 seconds. Reduce Motion falls back to a fade, or to no animation."),
        .code(
            language: "swift",
            source: """
            func animation(reduceMotion: Bool) -> Animation? {
                // Reduce Motion drops the spring.
                if reduceMotion { return nil }
                return .spring(response: 0.3, dampingFraction: 0.8)
            }
            """
        )
    ]

    private static func slots(claude: TargetSyncState, cursor: TargetSyncState, codex: TargetSyncState) -> [TargetSlot] {
        [
            TargetSlot(tool: "Claude", state: claude),
            TargetSlot(tool: "Cursor", state: cursor),
            TargetSlot(tool: "Codex", state: codex)
        ]
    }

    private static func make(
        name: String,
        summary: String,
        detail: String,
        chips: [SkillChip],
        flags: [SkillFlag],
        targets: [TargetSlot],
        uses: Int,
        lastUsed: String,
        series: [Int],
        body: [ReaderBlock]
    ) -> SkillFixture {
        SkillFixture(
            name: name,
            summary: summary,
            detail: detail,
            chips: chips,
            flags: flags,
            targets: targets,
            usesThisWeek: uses,
            lastUsed: lastUsed,
            series: series,
            body: body
        )
    }
}
