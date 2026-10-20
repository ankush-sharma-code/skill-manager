import AppKit
import SwiftUI

struct MenuBarPopover: View {
    @Bindable var model: DemoModel
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            status
            Divider()
            conflictRow
            Divider()
            week
            Divider()
            actions
        }
        .frame(width: Measure.popoverWidth, alignment: .topLeading)
        .frame(maxHeight: .infinity, alignment: .top)
        .background {
            ZStack {
                Rectangle().fill(Palette.elevated)
                Rectangle().fill(.regularMaterial)
            }
        }
    }

    private var status: some View {
        VStack(alignment: .leading, spacing: Spacing.xs) {
            HStack(spacing: Spacing.xs) {
                StatusDot(kind: model.statusKind, showsCheck: model.showsCheck)
                Text(model.statusTitle)
                    .font(Typography.headline)
                    .contentTransition(.opacity)
                Spacer(minLength: Spacing.xs)
                Button("Sync now") {
                    model.syncNow()
                }
                .buttonStyle(.borderedProminent)
                .motionPress()
                .accessibilityIdentifier("popover-sync")
            }
            Text("Last sync \(FixtureLibrary.lastSync)")
                .font(Typography.caption)
                .foregroundStyle(.secondary)
        }
        .padding(Spacing.sm)
        .animation(Motion.animation(.fade, reduceMotion: reduceMotion), value: model.statusTitle)
    }

    private var conflictRow: some View {
        HStack(spacing: Spacing.xs) {
            Text("\(model.conflictCount) conflicts")
                .font(Typography.callout)
            Spacer()
            Button("Review") {
                model.section = .conflicts
            }
            .buttonStyle(.bordered)
            .motionPress()
        }
        .padding(Spacing.sm)
    }

    private var week: some View {
        VStack(alignment: .leading, spacing: Spacing.xs) {
            Text("This week")
                .font(Typography.caption)
                .foregroundStyle(.secondary)
            ForEach(model.topSkills) { skill in
                HStack(spacing: Spacing.xs) {
                    Text(skill.name)
                        .font(Typography.callout)
                        .lineLimit(1)
                    Spacer(minLength: Spacing.xs)
                    AnimatingCount(value: skill.usesThisWeek, font: Typography.caption)
                        .foregroundStyle(.secondary)
                }
            }
        }
        .padding(Spacing.sm)
    }

    private var actions: some View {
        VStack(alignment: .leading, spacing: Spacing.xxs) {
            Button {
                model.section = .skills
            } label: {
                Label("Add Skill", systemImage: "plus")
                    .frame(maxWidth: .infinity, alignment: .leading)
            }
            .buttonStyle(.borderless)
            .motionPress()

            Button {
                NSApp.activate(ignoringOtherApps: true)
            } label: {
                Label("Open Skill Manager", systemImage: "macwindow")
                    .frame(maxWidth: .infinity, alignment: .leading)
            }
            .buttonStyle(.borderless)
            .motionPress()
        }
        .padding(Spacing.sm)
    }
}

#Preview("Menu bar") {
    MenuBarPopover(model: DemoModel())
        .frame(width: Measure.popoverWidth, height: Measure.popoverHeight)
}

#Preview("Menu bar dark") {
    MenuBarPopover(model: DemoModel())
        .frame(width: Measure.popoverWidth, height: Measure.popoverHeight)
        .preferredColorScheme(.dark)
}
