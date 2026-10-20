import SwiftUI

struct AppShell<Content: View, Detail: View>: View {
    @Bindable var model: DemoModel
    var content: () -> Content
    var detail: () -> Detail

    var body: some View {
        NavigationSplitView {
            SidebarColumn(model: model)
                .navigationSplitViewColumnWidth(
                    min: Layout.sidebarMin,
                    ideal: Layout.sidebar,
                    max: Layout.sidebarMax
                )
        } content: {
            content()
                .navigationSplitViewColumnWidth(
                    min: Layout.listMin,
                    ideal: Layout.list,
                    max: Layout.listMax
                )
        } detail: {
            detail()
        }
        .navigationSplitViewStyle(.balanced)
    }
}

struct SidebarColumn: View {
    @Bindable var model: DemoModel
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            VStack(alignment: .leading, spacing: Spacing.xxs) {
                ForEach(SidebarSection.allCases) { section in
                    sidebarButton(section)
                }
            }
            .padding(.horizontal, Spacing.xs)
            .padding(.top, Spacing.sm)
            Spacer(minLength: 0)
            Divider()
            statusFooter
        }
        .background {
            ZStack {
                Rectangle().fill(Palette.sidebar)
                Rectangle().fill(.regularMaterial)
            }
        }
    }

    private func sidebarButton(_ section: SidebarSection) -> some View {
        let selected = model.section == section
        return Button {
            withAnimation(Motion.animation(.snappy, reduceMotion: reduceMotion)) {
                model.section = section
            }
        } label: {
            HStack(spacing: Spacing.xs) {
                Image(systemName: section.symbol)
                    .font(Typography.body)
                    .foregroundStyle(selected ? Color.accentColor : Color.secondary)
                    .frame(width: Spacing.md)
                Text(section.title)
                    .font(Typography.body)
                    .foregroundStyle(.primary)
                Spacer(minLength: Spacing.xs)
                if section == .conflicts, model.conflictCount > 0 {
                    Text("\(model.conflictCount)")
                        .font(Typography.caption)
                        .foregroundStyle(.secondary)
                        .padding(.horizontal, Spacing.xs)
                        .padding(.vertical, Spacing.xxs)
                        .background(Palette.track, in: Capsule())
                }
            }
            .padding(.horizontal, Spacing.xs)
            .padding(.vertical, Spacing.xs)
            .frame(maxWidth: .infinity, alignment: .leading)
            .motionRow(isSelected: selected)
        }
        .buttonStyle(.plain)
        .accessibilityIdentifier("sidebar-\(section.rawValue)")
    }

    private var statusFooter: some View {
        VStack(alignment: .leading, spacing: Spacing.xs) {
            HStack(alignment: .firstTextBaseline, spacing: Spacing.xs) {
                StatusDot(kind: model.statusKind, showsCheck: model.showsCheck)
                    .accessibilityIdentifier("status-symbol")
                VStack(alignment: .leading, spacing: Spacing.xxs) {
                    HStack(alignment: .firstTextBaseline, spacing: Spacing.xxs) {
                        AnimatingCount(value: model.displayedSynced, font: Typography.headline)
                            .accessibilityIdentifier("synced-count")
                        Text("slots")
                            .font(Typography.caption)
                            .foregroundStyle(.secondary)
                    }
                    Text(model.statusTitle)
                        .font(Typography.caption)
                        .foregroundStyle(.secondary)
                        .contentTransition(.opacity)
                        .animation(Motion.animation(.fade, reduceMotion: reduceMotion), value: model.statusTitle)
                }
            }
            Button("Sync now") {
                model.syncNow()
            }
            .buttonStyle(.bordered)
            .motionPress()
            .frame(maxWidth: .infinity)
            .accessibilityIdentifier("sync-now")
        }
        .padding(Spacing.sm)
    }
}

struct SkillListColumn: View {
    @Bindable var model: DemoModel
    var morphs: Bool
    var namespace: Namespace.ID
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            VStack(alignment: .leading, spacing: Spacing.xs) {
                HStack {
                    Text("Skills")
                        .font(Typography.title2)
                    Spacer()
                    Text("\(model.visibleSkills.count)")
                        .font(Typography.caption)
                        .foregroundStyle(.secondary)
                }
                HStack(spacing: Spacing.xs) {
                    Image(systemName: "magnifyingglass")
                        .font(Typography.body)
                        .foregroundStyle(.secondary)
                    TextField("Search skills", text: $model.query)
                        .textFieldStyle(.plain)
                        .font(Typography.body)
                        .accessibilityIdentifier("skill-search")
                }
                .padding(.horizontal, Spacing.xs)
                .padding(.vertical, Spacing.xs)
                .background(Palette.search, in: RoundedRectangle(cornerRadius: Radius.medium, style: .continuous))
                .overlay(
                    RoundedRectangle(cornerRadius: Radius.medium, style: .continuous)
                        .stroke(Palette.stroke)
                )
            }
            .padding(.horizontal, Spacing.sm)
            .padding(.top, Spacing.sm)
            .padding(.bottom, Spacing.xs)

            ScrollView {
                LazyVStack(spacing: Spacing.xxs) {
                    ForEach(model.visibleSkills) { skill in
                        skillButton(skill)
                    }
                }
                .padding(.horizontal, Spacing.xs)
                .padding(.bottom, Spacing.sm)
            }
            .focusable()
            .onKeyPress(.downArrow) {
                step(1)
                return .handled
            }
            .onKeyPress(.upArrow) {
                step(-1)
                return .handled
            }
        }
        .background(Palette.content)
    }

    private func skillButton(_ skill: SkillFixture) -> some View {
        let selected = model.selection == skill.id && model.section == .skills
        return Button {
            withAnimation(Motion.animation(.snappy, reduceMotion: reduceMotion)) {
                model.select(skill.id)
            }
        } label: {
            SkillRowLabel(skill: skill, morphs: morphs, isSelected: selected, namespace: namespace)
                .padding(.horizontal, Spacing.xs)
                .padding(.vertical, Spacing.xs)
                .frame(maxWidth: .infinity, alignment: .leading)
                .motionRow(isSelected: selected)
        }
        .buttonStyle(.plain)
        .accessibilityIdentifier("skill-row-\(skill.id)")
    }

    private func step(_ offset: Int) {
        withAnimation(Motion.animation(.snappy, reduceMotion: reduceMotion)) {
            model.selectAdjacent(offset)
        }
    }
}

struct SkillRowLabel: View {
    var skill: SkillFixture
    var morphs: Bool
    var isSelected: Bool
    var namespace: Namespace.ID

    var body: some View {
        VStack(alignment: .leading, spacing: Spacing.xxs) {
            HStack(alignment: .firstTextBaseline, spacing: Spacing.xs) {
                Text(skill.name)
                    .font(Typography.headline)
                    .lineLimit(1)
                    .skillTitleMorph(id: skill.id, namespace: namespace, enabled: morphs, isSource: !isSelected)
                Spacer(minLength: Spacing.xs)
                ForEach(skill.flags, id: \.self) { flag in
                    Text(flag.title)
                        .font(Typography.caption)
                        .foregroundStyle(StatusColor.paint(flag.kind))
                        .padding(.horizontal, Spacing.xs)
                        .padding(.vertical, Spacing.xxs)
                        .background(StatusColor.paint(flag.kind, opacity: 0.14), in: Capsule())
                }
            }
            Text(skill.summary)
                .font(Typography.callout)
                .foregroundStyle(.secondary)
                .lineLimit(2)
            HStack(spacing: Spacing.xs) {
                ForEach(skill.targets) { slot in
                    Circle()
                        .fill(StatusColor.paint(slot.state.kind))
                        .frame(width: Spacing.xs, height: Spacing.xs)
                        .help("\(slot.tool) \(slot.state.label)")
                        .accessibilityLabel("\(slot.tool) \(slot.state.label)")
                }
                Spacer(minLength: Spacing.xs)
                Text("\(skill.usesThisWeek)")
                    .font(Typography.caption)
                    .foregroundStyle(.secondary)
                    .monospacedDigit()
                Text(skill.lastUsed)
                    .font(Typography.caption)
                    .foregroundStyle(.secondary)
            }
        }
    }
}

struct DetailColumn: View {
    @Bindable var model: DemoModel
    var playsIntro: Bool
    var morphs: Bool
    var namespace: Namespace.ID
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    var body: some View {
        VStack(spacing: 0) {
            if model.section == .skills, model.selectedSkill != nil {
                ReaderToolbar()
            }
            Group {
                if model.section == .skills, let skill = model.selectedSkill {
                    SkillReaderView(skill: skill, playsIntro: playsIntro, morphs: morphs, namespace: namespace)
                        .id(skill.id)
                        .transition(.opacity)
                } else if model.section == .skills {
                    EmptyStateView(
                        title: SidebarSection.skills.emptyTitle,
                        systemImage: SidebarSection.skills.symbol,
                        message: SidebarSection.skills.emptyMessage,
                        actionTitle: "Add Skill"
                    )
                } else {
                    EmptyStateView(
                        title: model.section.emptyTitle,
                        systemImage: model.section.symbol,
                        message: model.section.emptyMessage,
                        actionTitle: model.section == .settings ? "Run Setup Again" : nil,
                        action: { model.wizardPresented = true }
                    )
                }
            }
            .animation(Motion.animation(.snappy, reduceMotion: reduceMotion), value: model.selection)
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .background(Palette.page)
    }
}

struct MainWindowMock: View {
    @Bindable var model: DemoModel
    var playsIntro: Bool
    var animatesSelection: Bool
    var showsPalette: Bool = false
    @Namespace private var namespace
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    var body: some View {
        ZStack(alignment: .top) {
            AppShell(model: model) {
                if model.section == .skills {
                    SkillListColumn(model: model, morphs: animatesSelection, namespace: namespace)
                } else {
                    VStack(alignment: .leading, spacing: Spacing.xs) {
                        Text(model.section.title)
                            .font(Typography.title2)
                            .padding(.horizontal, Spacing.sm)
                            .padding(.top, Spacing.sm)
                        Spacer()
                    }
                    .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
                    .background(Palette.content)
                }
            } detail: {
                DetailColumn(
                    model: model,
                    playsIntro: playsIntro,
                    morphs: animatesSelection,
                    namespace: namespace
                )
            }

            if showsPalette || model.palettePresented {
                Rectangle()
                    .fill(Palette.scrim)
                    .ignoresSafeArea()
                    .onTapGesture {
                        model.palettePresented = false
                    }
                    .transition(.opacity)
                QuickSearchPalette(
                    groups: PaletteCatalog.groups(skills: model.skills, query: model.paletteQuery),
                    query: $model.paletteQuery,
                    onSubmit: { model.perform($0) },
                    onDismiss: { model.palettePresented = false }
                )
                .frame(width: Layout.paletteWidth)
                .padding(.top, Layout.paletteTop)
                .transition(.opacity)
            }
        }
        .animation(Motion.animation(.fade, reduceMotion: reduceMotion), value: model.palettePresented)
        .animation(Motion.animation(.fade, reduceMotion: reduceMotion), value: showsPalette)
    }
}

struct CommandPaletteMock: View {
    @State private var model = DemoModel()

    var body: some View {
        MainWindowMock(model: model, playsIntro: false, animatesSelection: false, showsPalette: true)
    }
}

#Preview("Main window") {
    MainWindowMock(model: DemoModel(), playsIntro: false, animatesSelection: false)
        .frame(width: Layout.mainWidth, height: Layout.mainHeight)
}

#Preview("Main window dark") {
    MainWindowMock(model: DemoModel(), playsIntro: false, animatesSelection: false)
        .frame(width: Layout.mainWidth, height: Layout.mainHeight)
        .preferredColorScheme(.dark)
}

#Preview("Command palette") {
    CommandPaletteMock()
        .frame(width: Layout.mainWidth, height: Layout.mainHeight)
}
