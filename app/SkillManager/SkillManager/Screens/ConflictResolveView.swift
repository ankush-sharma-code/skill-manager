import SwiftUI

struct ConflictResolveView: View {
    var skill: SkillFixture
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    @State private var choice: String?

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: Spacing.md) {
                VStack(alignment: .leading, spacing: Spacing.xs) {
                    Text(skill.name)
                        .font(Typography.title2)
                    Text("Choose which copy to keep. The folder stays the source of truth unless you pick the tool.")
                        .font(Typography.body)
                        .foregroundStyle(.secondary)
                        .fixedSize(horizontal: false, vertical: true)
                }
                ForEach(skill.targets.filter { $0.state == .conflict }) { slot in
                    conflictCard(slot)
                }
            }
            .padding(Spacing.lg)
            .frame(maxWidth: Measure.reading, alignment: .leading)
            .frame(maxWidth: .infinity, alignment: .leading)
        }
        .background(Palette.page)
        .accessibilityIdentifier("conflict-resolve")
    }

    private func conflictCard(_ slot: TargetSlot) -> some View {
        let copies = ConflictCopy.pair(skill: skill.id, tool: slot.tool)
        return SectionCard(title: slot.tool) {
            VStack(alignment: .leading, spacing: Spacing.sm) {
                HStack(spacing: Spacing.xs) {
                    ToolMark(tool: slot.tool, state: slot.state)
                    Text(slot.state.label)
                        .font(Typography.caption)
                        .foregroundStyle(.secondary)
                }
                HStack(alignment: .top, spacing: Spacing.sm) {
                    copyColumn(title: "Folder", text: copies.folder)
                    copyColumn(title: "Tool copy", text: copies.toolCopy)
                }
                HStack(spacing: Spacing.xs) {
                    resolveButton("Keep folder", id: "\(slot.tool)-folder", prominent: true)
                    resolveButton("Keep tool copy", id: "\(slot.tool)-tool", prominent: false)
                    resolveButton("Keep both", id: "\(slot.tool)-both", prominent: false)
                }
            }
        }
    }

    private func copyColumn(title: String, text: String) -> some View {
        VStack(alignment: .leading, spacing: Spacing.xxs) {
            Text(title)
                .font(Typography.caption)
                .foregroundStyle(.secondary)
            Text(text)
                .font(Typography.callout)
                .fixedSize(horizontal: false, vertical: true)
                .frame(maxWidth: .infinity, alignment: .leading)
                .padding(Spacing.xs)
                .background(Palette.codeBackground, in: RoundedRectangle(cornerRadius: Radius.small, style: .continuous))
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }

    @ViewBuilder
    private func resolveButton(_ title: String, id: String, prominent: Bool) -> some View {
        let selected = prominent || choice == id
        if selected {
            Button(title) {
                pick(id)
            }
            .buttonStyle(.borderedProminent)
            .motionPress()
            .accessibilityIdentifier("resolve-\(id)")
        } else {
            Button(title) {
                pick(id)
            }
            .buttonStyle(.bordered)
            .motionPress()
            .accessibilityIdentifier("resolve-\(id)")
        }
    }

    private func pick(_ id: String) {
        withAnimation(Motion.animation(.snappy, reduceMotion: reduceMotion)) {
            choice = id
        }
    }
}

enum ConflictCopy {
    static func pair(skill: String, tool: String) -> (folder: String, toolCopy: String) {
        if skill == "tdd" {
            return (FixtureLibrary.collisionClaude, FixtureLibrary.collisionCursor)
        }
        if tool == "Claude" {
            return ("Adopt skills that already live in a tool.", "Claude already has an importer skill with a different body.")
        }
        return ("Group copies by digest before writing.", "Cursor's importer asks for a new name on every collision.")
    }
}

#Preview("Conflict resolve") {
    ConflictResolveView(skill: FixtureLibrary.skills.first { $0.id == "tdd" } ?? FixtureLibrary.skills[0])
        .frame(width: Measure.readerWidth, height: Measure.readerHeight)
}
