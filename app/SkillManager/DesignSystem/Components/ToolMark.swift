import SwiftUI

enum ToolSymbol {
    static func systemName(for tool: String) -> String {
        switch tool {
        case "Claude", "Claude Code":
            "sparkle"
        case "Cursor":
            "cursorarrow"
        case "Codex":
            "terminal"
        default:
            "circle.fill"
        }
    }
}

struct ToolMark: View {
    var tool: String
    var state: TargetSyncState

    var body: some View {
        Image(systemName: ToolSymbol.systemName(for: tool))
            .font(Typography.caption)
            .foregroundStyle(StatusColor.paint(state.kind))
            .frame(width: Spacing.sm, height: Spacing.sm)
            .help("\(tool), \(state.label)")
            .accessibilityLabel("\(tool), \(state.label)")
    }
}

#Preview("Tool marks") {
    HStack(spacing: Spacing.sm) {
        ToolMark(tool: "Claude", state: .inSync)
        ToolMark(tool: "Cursor", state: .conflict)
        ToolMark(tool: "Codex", state: .drift)
    }
    .padding(Spacing.lg)
}
