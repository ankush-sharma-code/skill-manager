import SwiftUI

struct AppShell<Content: View, Detail: View>: View {
    @Bindable var model: DemoModel
    @ViewBuilder var content: () -> Content
    @ViewBuilder var detail: () -> Detail

    var body: some View {
        HStack(spacing: 0) {
            SidebarColumn(model: model)
                .frame(width: Measure.sidebar)
            content()
                .frame(minWidth: 260, idealWidth: Measure.list, maxWidth: Measure.list)
            detail()
                .frame(maxWidth: .infinity, maxHeight: .infinity)
        }
    }
}

struct SidebarColumn: View {
    @Bindable var model: DemoModel
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            Text("Skill Manager")
                .font(Typography.headline)
                .padding(.horizontal, Spacing.sm)
                .padding(.top, Spacing.sm)
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
                show(section)
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

    private func show(_ section: SidebarSection) {
        model.section = section
        switch section {
        case .conflicts:
            if model.selectedSkill?.flags.contains(.conflict) != true {
                model.selection = model.skills.first { $0.flags.contains(.conflict) }?.id ?? model.selection
            }
        case .idle:
            if model.selectedSkill?.flags.contains(.idle) != true {
                model.selection = model.skills.first { $0.flags.contains(.idle) }?.id ?? model.selection
            }
        case .all, .tools, .usage:
            break
        }
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
                    Text(model.section.title)
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

            ScrollViewReader { proxy in
                ScrollView {
                    LazyVStack(spacing: Spacing.xxs) {
                        ForEach(model.visibleSkills) { skill in
                            skillButton(skill)
                                .id(skill.id)
                        }
                    }
                    .padding(.horizontal, Spacing.xs)
                    .padding(.bottom, Spacing.sm)
                }
                .onChange(of: model.selection) { _, id in
                    guard !id.isEmpty else { return }
                    if let animation = Motion.animation(.snappy, reduceMotion: reduceMotion) {
                        withAnimation(animation) {
                            proxy.scrollTo(id, anchor: .center)
                        }
                    } else {
                        proxy.scrollTo(id, anchor: .center)
                    }
                }
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
        let selected = model.selection == skill.id && model.section != .tools
        return ZStack(alignment: .trailing) {
            Button {
                withAnimation(Motion.animation(.snappy, reduceMotion: reduceMotion)) {
                    if skill.flags.contains(.conflict), model.section == .conflicts {
                        model.openConflict(skill.id)
                    } else {
                        model.select(skill.id)
                    }
                }
            } label: {
                SkillRowLabel(skill: skill, morphs: morphs, isSelected: selected, namespace: namespace)
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .padding(.trailing, skill.flags.contains(.conflict) ? 76 : 0)
            }
            .buttonStyle(.plain)
            .accessibilityIdentifier("skill-row-\(skill.id)")
            if skill.flags.contains(.conflict) {
                Button {
                    withAnimation(Motion.animation(.snappy, reduceMotion: reduceMotion)) {
                        model.openConflict(skill.id)
                    }
                } label: {
                    Text("Conflict")
                        .font(Typography.caption)
                        .foregroundStyle(StatusColor.paint(.conflict))
                        .padding(.horizontal, Spacing.xs)
                        .padding(.vertical, Spacing.xxs)
                        .background(StatusColor.paint(.conflict, opacity: 0.14), in: Capsule())
                }
                .buttonStyle(.plain)
                .motionPress()
                .padding(.trailing, Spacing.xs)
                .zIndex(1)
                .accessibilityIdentifier("conflict-badge-\(skill.id)")
            }
        }
        .padding(.horizontal, Spacing.xs)
        .padding(.vertical, Spacing.xs)
        .motionRow(isSelected: selected)
    }

    private func step(_ offset: Int) {
        withAnimation(Motion.animation(.snappy, reduceMotion: reduceMotion)) {
            model.selectAdjacent(offset)
        }
    }
}

struct ToolListColumn: View {
    @Bindable var model: DemoModel
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            Text("Tools")
                .font(Typography.title2)
                .padding(.horizontal, Spacing.sm)
                .padding(.top, Spacing.sm)
                .padding(.bottom, Spacing.xs)
            ScrollView {
                LazyVStack(spacing: Spacing.xxs) {
                    ForEach(FixtureLibrary.detectedTools) { tool in
                        Button {
                            withAnimation(Motion.animation(.snappy, reduceMotion: reduceMotion)) {
                                model.toolSelection = tool.id
                                model.section = .tools
                            }
                        } label: {
                            VStack(alignment: .leading, spacing: Spacing.xxs) {
                                Text(tool.name)
                                    .font(Typography.headline)
                                Text(tool.path)
                                    .font(Typography.caption)
                                    .foregroundStyle(.secondary)
                                    .lineLimit(1)
                            }
                            .padding(.horizontal, Spacing.xs)
                            .padding(.vertical, Spacing.xs)
                            .frame(maxWidth: .infinity, alignment: .leading)
                            .motionRow(isSelected: model.toolSelection == tool.id)
                        }
                        .buttonStyle(.plain)
                        .accessibilityIdentifier("tool-row-\(tool.id)")
                    }
                }
                .padding(.horizontal, Spacing.xs)
            }
            Spacer(minLength: 0)
        }
        .background(Palette.content)
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
                ForEach(skill.flags.filter { $0 != .conflict }, id: \.self) { flag in
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
                    ToolMark(tool: slot.tool, state: slot.state)
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
            if showsReader, model.selectedSkill != nil {
                ReaderToolbar()
            }
            Group {
                if model.section == .conflicts, let skill = model.selectedSkill, skill.flags.contains(.conflict) {
                    ConflictResolveView(skill: skill)
                        .id(skill.id)
                        .transition(.opacity)
                } else if model.section == .tools {
                    ToolDetail(toolID: model.toolSelection)
                } else if model.section == .usage, let skill = model.selectedSkill {
                    UsageDetail(skill: skill, playsIntro: playsIntro)
                        .id(skill.id)
                } else if showsReader, let skill = model.selectedSkill {
                    SkillReaderView(skill: skill, playsIntro: playsIntro, morphs: morphs, namespace: namespace)
                        .id(skill.id)
                        .transition(.opacity)
                } else if model.skills.isEmpty {
                    EmptyStateView(
                        title: SidebarSection.all.emptyTitle,
                        systemImage: SidebarSection.all.symbol,
                        message: SidebarSection.all.emptyMessage,
                        actionTitle: "Add Skill"
                    )
                } else {
                    EmptyStateView(
                        title: model.section.emptyTitle,
                        systemImage: model.section.symbol,
                        message: model.section.emptyMessage
                    )
                }
            }
            .animation(Motion.animation(.snappy, reduceMotion: reduceMotion), value: model.selection)
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .background(Palette.page)
    }

    private var showsReader: Bool {
        model.section == .all || model.section == .idle
    }
}

struct ToolDetail: View {
    var toolID: String

    private var tool: DetectedTool? {
        FixtureLibrary.detectedTools.first { $0.id == toolID }
    }

    var body: some View {
        VStack(alignment: .leading, spacing: Spacing.md) {
            if let tool {
                Text(tool.name)
                    .font(Typography.title2)
                Text(tool.path)
                    .font(Typography.body)
                    .foregroundStyle(.secondary)
                if let warning = tool.warning {
                    Text(warning)
                        .font(Typography.callout)
                        .foregroundStyle(StatusColor.paint(.attention))
                }
                SectionCard(title: "Sync") {
                    Text(tool.detected ? "Detected on this Mac" : "Not detected")
                        .font(Typography.body)
                }
            }
            Spacer()
        }
        .padding(Spacing.lg)
        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
        .background(Palette.page)
    }
}

struct UsageDetail: View {
    var skill: SkillFixture
    var playsIntro: Bool

    var body: some View {
        VStack(alignment: .leading, spacing: Spacing.md) {
            Text(skill.name)
                .font(Typography.title2)
            Text("\(skill.usesThisWeek) uses this week. Last used \(skill.lastUsed).")
                .font(Typography.body)
                .foregroundStyle(.secondary)
            Sparkline(values: skill.series, playsIntro: playsIntro)
                .frame(width: Measure.sparklineWidth)
            Spacer()
        }
        .padding(Spacing.lg)
        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
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
                if model.section == .tools {
                    ToolListColumn(model: model)
                } else {
                    SkillListColumn(model: model, morphs: animatesSelection, namespace: namespace)
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
                .frame(width: Measure.paletteWidth)
                .padding(.top, Measure.paletteTop)
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
        .frame(width: Measure.mainWidth, height: Measure.mainHeight)
}

#Preview("Main window dark") {
    MainWindowMock(model: DemoModel(), playsIntro: false, animatesSelection: false)
        .frame(width: Measure.mainWidth, height: Measure.mainHeight)
        .preferredColorScheme(.dark)
}

#Preview("Command palette") {
    CommandPaletteMock()
        .frame(width: Measure.mainWidth, height: Measure.mainHeight)
}
