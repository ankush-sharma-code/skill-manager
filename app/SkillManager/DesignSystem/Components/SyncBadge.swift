import SwiftUI

struct SyncBadge: View {
    var tool: String
    var state: TargetSyncState

    var body: some View {
        HStack(spacing: Spacing.xxs) {
            Circle()
                .fill(StatusColor.paint(state.kind))
                .frame(width: Spacing.xs, height: Spacing.xs)
            Text(tool)
                .font(Typography.caption)
            Text(state.label)
                .font(Typography.caption)
                .foregroundStyle(.secondary)
        }
        .padding(.horizontal, Spacing.xs)
        .padding(.vertical, Spacing.xxs)
        .background(StatusColor.paint(state.kind, opacity: 0.14), in: Capsule())
        .accessibilityElement(children: .combine)
        .accessibilityLabel("\(tool) \(state.label)")
    }
}

#Preview("Sync badge") {
    HStack(spacing: Spacing.xs) {
        SyncBadge(tool: "Claude", state: .inSync)
        SyncBadge(tool: "Cursor", state: .drift)
        SyncBadge(tool: "Codex", state: .missing)
        SyncBadge(tool: "Cursor", state: .conflict)
    }
    .padding(Spacing.lg)
}

#Preview("Sync badge dark") {
    HStack(spacing: Spacing.xs) {
        SyncBadge(tool: "Claude", state: .inSync)
        SyncBadge(tool: "Cursor", state: .conflict)
    }
    .padding(Spacing.lg)
    .background(Palette.page)
    .preferredColorScheme(.dark)
}
