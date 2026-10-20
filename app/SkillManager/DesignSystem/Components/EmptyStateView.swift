import SwiftUI

struct EmptyStateView: View {
    var title: String
    var systemImage: String
    var message: String
    var actionTitle: String?
    var action: () -> Void = {}

    var body: some View {
        VStack(spacing: Spacing.sm) {
            Image(systemName: systemImage)
                .font(Typography.largeTitle)
                .foregroundStyle(Color.accentColor)
                .symbolRenderingMode(.hierarchical)
            Text(title)
                .font(Typography.title2)
                .multilineTextAlignment(.center)
            Text(message)
                .font(Typography.body)
                .foregroundStyle(.secondary)
                .multilineTextAlignment(.center)
                .frame(maxWidth: Measure.chipMax)
            if let actionTitle {
                Button(actionTitle, action: action)
                    .buttonStyle(.borderedProminent)
                    .motionPress()
                    .padding(.top, Spacing.xs)
            }
        }
        .padding(Spacing.lg)
        .frame(maxWidth: .infinity, maxHeight: .infinity)
    }
}

#Preview("Empty state") {
    EmptyStateView(
        title: "No skills yet",
        systemImage: "square.stack.3d.up",
        message: "Add a skill once. Skill Manager syncs it to every tool you turned on.",
        actionTitle: "Add Skill"
    )
    .frame(width: 480, height: 360)
}
