import SwiftUI

struct ReaderToolbar: View {
    var body: some View {
        HStack(spacing: Spacing.xs) {
            Spacer()
            Button {} label: {
                Label("Reveal in Finder", systemImage: "folder")
            }
            .buttonStyle(.bordered)
            .motionPress()
            .accessibilityIdentifier("reveal-in-finder")

            Button {} label: {
                Label("Open in Editor", systemImage: "square.and.pencil")
            }
            .buttonStyle(.borderedProminent)
            .motionPress()
            .accessibilityIdentifier("open-in-editor")
        }
        .padding(.horizontal, Spacing.sm)
        .frame(minHeight: Measure.columnHeader)
        .background {
            ZStack {
                Rectangle().fill(Palette.content)
                Rectangle().fill(.bar)
            }
        }
    }
}

struct SkillReaderView: View {
    var skill: SkillFixture
    var playsIntro: Bool
    var morphs: Bool
    var namespace: Namespace.ID

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: Spacing.md) {
                header
                ChipWrap(spacing: Spacing.xs) {
                    ForEach(skill.chips) { chip in
                        FrontmatterChip(key: chip.key, value: chip.value)
                    }
                }
                SectionCard(title: "Targets") {
                    ChipWrap(spacing: Spacing.xs) {
                        ForEach(skill.targets) { slot in
                            SyncBadge(tool: slot.tool, state: slot.state)
                        }
                    }
                }
                ReaderBlocks(blocks: skill.body)
            }
            .padding(Spacing.lg)
            .frame(maxWidth: Measure.reading, alignment: .leading)
            .frame(maxWidth: .infinity)
            .textSelection(.enabled)
        }
        .background(Palette.page)
    }

    private var header: some View {
        HStack(alignment: .top, spacing: Spacing.sm) {
            VStack(alignment: .leading, spacing: Spacing.xs) {
                Text(skill.name)
                    .font(Typography.title2)
                    .skillTitleMorph(id: skill.id, namespace: namespace, enabled: morphs, isSource: true)
                Text(skill.detail)
                    .font(Typography.body)
                    .foregroundStyle(.secondary)
                    .fixedSize(horizontal: false, vertical: true)
            }
            Spacer(minLength: Spacing.sm)
            VStack(alignment: .trailing, spacing: Spacing.xxs) {
                Sparkline(values: skill.series, playsIntro: playsIntro)
                    .frame(width: Measure.sparklineWidth)
                HStack(spacing: Spacing.xxs) {
                    AnimatingCount(value: skill.usesThisWeek, font: Typography.caption)
                    Text("this week")
                        .font(Typography.caption)
                        .foregroundStyle(.secondary)
                }
                Text("30 days")
                    .font(Typography.caption)
                    .foregroundStyle(.secondary)
            }
        }
    }
}

struct ReaderBlocks: View {
    var blocks: [ReaderBlock]

    var body: some View {
        VStack(alignment: .leading, spacing: Spacing.md) {
            ForEach(blocks) { block in
                switch block {
                case let .heading(text):
                    Text(text)
                        .font(Typography.headline)
                case let .paragraph(text):
                    Text(text)
                        .font(Typography.body)
                        .lineSpacing(Spacing.xxs)
                        .fixedSize(horizontal: false, vertical: true)
                case let .bullets(items):
                    VStack(alignment: .leading, spacing: Spacing.xs) {
                        ForEach(items, id: \.self) { item in
                            HStack(alignment: .firstTextBaseline, spacing: Spacing.xs) {
                                Text("•")
                                    .font(Typography.body)
                                Text(item)
                                    .font(Typography.body)
                                    .fixedSize(horizontal: false, vertical: true)
                            }
                        }
                    }
                case let .code(language, source):
                    CodeBlockView(language: language, source: source)
                }
            }
        }
    }
}

struct SkillReaderScreen: View {
    var playsIntro: Bool
    @Namespace private var namespace
    private var skill: SkillFixture {
        FixtureLibrary.skills.first { $0.id == "poteto-mode" } ?? FixtureLibrary.skills[0]
    }

    var body: some View {
        VStack(spacing: 0) {
            ReaderToolbar()
            SkillReaderView(skill: skill, playsIntro: playsIntro, morphs: false, namespace: namespace)
        }
        .background(Palette.page)
    }
}

#Preview("Skill reader") {
    SkillReaderScreen(playsIntro: false)
        .frame(width: Measure.readerWidth, height: Measure.readerHeight)
}

#Preview("Skill reader dark") {
    SkillReaderScreen(playsIntro: false)
        .frame(width: Measure.readerWidth, height: Measure.readerHeight)
        .preferredColorScheme(.dark)
}
