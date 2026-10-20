import SwiftUI

struct ErrorStateView: View {
    var title: String
    var message: String
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
                .frame(maxWidth: 360)
            Button("Retry", action: retry)
                .buttonStyle(.borderedProminent)
                .motionPress()
                .padding(.top, Spacing.xs)
                .accessibilityIdentifier("retry")
        }
        .padding(Spacing.lg)
        .frame(maxWidth: .infinity, maxHeight: .infinity)
    }
}

#Preview("Error state") {
    ErrorStateView(
        title: "Couldn't reach skillsd",
        message: "Skill Manager will keep trying in the background. Retry if you want to check again now."
    )
    .frame(width: 480, height: 360)
}
