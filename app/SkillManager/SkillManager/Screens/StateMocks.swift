import SwiftUI

struct EmptySkillsMock: View {
    @State private var model = DemoModel(skills: [], selection: "")

    var body: some View {
        AppShell(model: model) {
            VStack(alignment: .leading, spacing: Spacing.xs) {
                Text("Skills")
                    .font(Typography.title2)
                Text("None yet")
                    .font(Typography.callout)
                    .foregroundStyle(.secondary)
                Spacer()
            }
            .padding(Spacing.sm)
            .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
            .background(Palette.content)
        } detail: {
            EmptyStateView(
                title: "No skills yet",
                systemImage: "square.stack.3d.up",
                message: "Add a skill once. Skill Manager syncs it to every tool you turned on.",
                actionTitle: "Add Skill"
            )
            .background(Palette.page)
        }
    }
}

struct LoadingSkillsMock: View {
    @State private var model = DemoModel(skills: [], selection: "", reportsLoading: true)

    var body: some View {
        AppShell(model: model) {
            VStack(alignment: .leading, spacing: Spacing.sm) {
                Text("Skills")
                    .font(Typography.title2)
                SkeletonRows()
                Spacer()
            }
            .padding(.horizontal, Spacing.xs)
            .padding(.top, Spacing.sm)
            .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
            .background(Palette.content)
        } detail: {
            VStack(spacing: Spacing.md) {
                ProgressView("Loading skills")
                    .font(Typography.callout)
                Text("Reading ~/.skills")
                    .font(Typography.caption)
                    .foregroundStyle(.secondary)
                SkeletonBlock()
                    .frame(width: 280)
                SkeletonBlock(height: Spacing.xs)
                    .frame(width: 220)
            }
            .frame(maxWidth: .infinity, maxHeight: .infinity)
            .background(Palette.page)
        }
    }
}

struct ErrorSkillsMock: View {
    @State private var model = DemoModel(skills: [], selection: "")

    var body: some View {
        AppShell(model: model) {
            VStack(alignment: .leading, spacing: Spacing.xs) {
                Text("Skills")
                    .font(Typography.title2)
                Text("Unavailable")
                    .font(Typography.callout)
                    .foregroundStyle(.secondary)
                Spacer()
            }
            .padding(Spacing.sm)
            .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
            .background(Palette.content)
        } detail: {
            ErrorStateView(
                title: "Couldn't reach skillsd",
                message: "Skill Manager will keep trying in the background. Retry if you want to check again now."
            )
            .background(Palette.page)
        }
    }
}

#Preview("Empty") {
    EmptySkillsMock()
        .frame(width: Measure.mainWidth, height: Measure.mainHeight)
}

#Preview("Loading") {
    LoadingSkillsMock()
        .frame(width: Measure.mainWidth, height: Measure.mainHeight)
}

#Preview("Error") {
    ErrorSkillsMock()
        .frame(width: Measure.mainWidth, height: Measure.mainHeight)
}

#Preview("Error dark") {
    ErrorSkillsMock()
        .frame(width: Measure.mainWidth, height: Measure.mainHeight)
        .preferredColorScheme(.dark)
}
