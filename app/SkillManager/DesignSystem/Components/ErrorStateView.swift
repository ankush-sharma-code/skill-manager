import SwiftUI

struct ErrorStateView: View {
    var title: String
    var message: String
    var start: () -> Void = {}
    var retry: () -> Void = {}

    var body: some View {
        VStack(spacing: Spacing.sm) {
            Image(systemName: "exclamationmark.triangle")
                .font(Typography.largeTitle)
                .foregroundStyle(StatusColor.paint(.attention))
            Text(title)
                .font(Typography.title2)
                .multilineTextAlignment(.center)
            Text(message)
                .font(Typography.body)
                .foregroundStyle(.secondary)
                .multilineTextAlignment(.center)
                .frame(maxWidth: 420)
            HStack(spacing: Spacing.xs) {
                Button("Start it", action: start)
                    .keyboardShortcut(.defaultAction)
                    .buttonStyle(.borderedProminent)
                    .tint(Color.accentColor)
                    .environment(\.controlActiveState, .active)
                    .motionPress()
                    .accessibilityIdentifier("start-service")
                Button("Retry", action: retry)
                    .buttonStyle(.bordered)
                    .motionPress()
                    .accessibilityIdentifier("retry")
            }
            .padding(.top, Spacing.xs)
        }
        .padding(Spacing.lg)
        .frame(maxWidth: .infinity, maxHeight: .infinity)
    }
}

#Preview("Error state") {
    ErrorStateView(
        title: "Skill Manager's background service isn't running",
        message: "Start the service, or retry the check."
    )
    .frame(width: 480, height: 360)
}
