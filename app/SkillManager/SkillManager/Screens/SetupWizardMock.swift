import SwiftUI

enum WizardStep: Int, CaseIterable, Identifiable, Hashable {
    case welcome
    case detect
    case importExisting
    case syncMode

    var id: Int { rawValue }

    var indexLabel: String {
        "Step \(rawValue + 1) of \(WizardStep.allCases.count)"
    }

    var title: String {
        switch self {
        case .welcome: "One folder for every skill"
        case .detect: "Choose your tools"
        case .importExisting: "Bring existing skills in"
        case .syncMode: "How should skills sync?"
        }
    }

    var subtitle: String {
        switch self {
        case .welcome:
            "Skill Manager keeps ~/.skills as the source of truth and syncs each skill to Claude Code, Cursor, and Codex."
        case .detect:
            "Detected tools are selected. You can change this later."
        case .importExisting:
            "Nothing is written yet. Duplicates collapse. A name collision waits for you."
        case .syncMode:
            "Symlink for a tool on this Mac. Copy for a project you commit."
        }
    }

    var continueTitle: String {
        switch self {
        case .welcome: "Get Started"
        case .syncMode: "Done"
        default: "Continue"
        }
    }
}

struct WizardChrome<Content: View>: View {
    var step: WizardStep
    var onBack: () -> Void
    var onContinue: () -> Void
    var onSkip: () -> Void
    @ViewBuilder var content: () -> Content
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            header
            ZStack(alignment: .topLeading) {
                content()
            }
            .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
            .clipped()
            .padding(.top, Spacing.sm)
            footer
        }
        .padding(Spacing.lg)
        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
        .background(Palette.page)
    }

    private var header: some View {
        VStack(alignment: .leading, spacing: Spacing.sm) {
            HStack(spacing: Spacing.xs) {
                ForEach(WizardStep.allCases) { item in
                    Capsule()
                        .fill(item.rawValue <= step.rawValue ? AnyShapeStyle(Color.accentColor) : AnyShapeStyle(Palette.track))
                        .frame(width: Spacing.lg, height: Spacing.xxs)
                }
            }
            .animation(Motion.animation(.snappy, reduceMotion: reduceMotion), value: step)
            .accessibilityLabel(step.indexLabel)

            Text(step.indexLabel)
                .font(Typography.caption)
                .foregroundStyle(.secondary)
            Text(step.title)
                .font(step == .welcome ? Typography.largeTitle : Typography.title2)
                .fixedSize(horizontal: false, vertical: true)
            Text(step.subtitle)
                .font(Typography.body)
                .foregroundStyle(.secondary)
                .fixedSize(horizontal: false, vertical: true)
        }
    }

    private var footer: some View {
        HStack(spacing: Spacing.xs) {
            if step == .welcome {
                Button("Skip for Now", action: onSkip)
                    .buttonStyle(.borderless)
                    .motionPress()
                    .accessibilityIdentifier("wizard-skip")
            } else {
                Button("Back", action: onBack)
                    .buttonStyle(.bordered)
                    .motionPress()
                    .accessibilityIdentifier("wizard-back")
            }
            Spacer()
            Button(step.continueTitle, action: onContinue)
                .buttonStyle(.borderedProminent)
                .motionPress()
                .accessibilityIdentifier("wizard-continue")
        }
        .padding(.top, Spacing.sm)
    }
}

struct WizardBody: View {
    var step: WizardStep
    @Binding var claude: Bool
    @Binding var cursor: Bool
    @Binding var codex: Bool
    @Binding var modes: [SyncTargetOption]

    var body: some View {
        switch step {
        case .welcome:
            welcome
        case .detect:
            detect
        case .importExisting:
            importExisting
        case .syncMode:
            syncMode
        }
    }

    private var welcome: some View {
        VStack(spacing: Spacing.md) {
            Spacer(minLength: Spacing.sm)
            Image(systemName: "square.stack.3d.up")
                .font(Typography.largeTitle)
                .foregroundStyle(Color.accentColor)
                .symbolRenderingMode(.hierarchical)
            Text("Add a skill once. Every tool stays on that copy.")
                .font(Typography.callout)
                .foregroundStyle(.secondary)
                .multilineTextAlignment(.center)
                .frame(maxWidth: 420)
            Spacer(minLength: Spacing.sm)
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
    }

    private var detect: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: Spacing.xs) {
                toolRow("Claude Code", path: FixtureLibrary.detectedTools[0].path, warning: nil, isOn: $claude)
                toolRow("Cursor", path: FixtureLibrary.detectedTools[1].path, warning: nil, isOn: $cursor)
                toolRow("Codex", path: FixtureLibrary.detectedTools[2].path, warning: FixtureLibrary.detectedTools[2].warning, isOn: $codex)
            }
        }
    }

    private func toolRow(_ name: String, path: String, warning: String?, isOn: Binding<Bool>) -> some View {
        VStack(alignment: .leading, spacing: Spacing.xxs) {
            Toggle(isOn: isOn) {
                HStack {
                    Text(name)
                        .font(Typography.headline)
                    Spacer()
                    Text(isOn.wrappedValue ? "Detected" : "Off")
                        .font(Typography.caption)
                        .foregroundStyle(.secondary)
                }
            }
            .toggleStyle(.checkbox)
            Text(path)
                .font(Typography.callout)
                .foregroundStyle(.secondary)
                .padding(.leading, Spacing.md)
            if let warning {
                Label(warning, systemImage: "exclamationmark.triangle.fill")
                    .font(Typography.caption)
                    .foregroundStyle(StatusColor.paint(.attention))
                    .padding(.leading, Spacing.md)
            }
        }
        .padding(Spacing.sm)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(Palette.elevated, in: RoundedRectangle(cornerRadius: Radius.medium, style: .continuous))
        .overlay(
            RoundedRectangle(cornerRadius: Radius.medium, style: .continuous)
                .stroke(Palette.stroke)
        )
    }

    private var importExisting: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: Spacing.sm) {
                SectionCard(title: "Same skill in two tools") {
                    VStack(alignment: .leading, spacing: Spacing.xxs) {
                        Text("review")
                            .font(Typography.headline)
                        Text("Claude Code and Cursor have the same files. They become one skill.")
                            .font(Typography.callout)
                            .foregroundStyle(.secondary)
                            .fixedSize(horizontal: false, vertical: true)
                    }
                }
                SectionCard(title: "Name collision") {
                    VStack(alignment: .leading, spacing: Spacing.sm) {
                        Text("tdd")
                            .font(Typography.headline)
                        HStack(alignment: .top, spacing: Spacing.sm) {
                            collisionColumn("Claude", text: FixtureLibrary.collisionClaude)
                            collisionColumn("Cursor", text: FixtureLibrary.collisionCursor)
                        }
                        HStack(spacing: Spacing.xs) {
                            Button("Keep Claude") {}
                                .buttonStyle(.bordered)
                                .motionPress()
                            Button("Keep Cursor") {}
                                .buttonStyle(.bordered)
                                .motionPress()
                            Button("Rename") {}
                                .buttonStyle(.bordered)
                                .motionPress()
                        }
                    }
                }
            }
        }
    }

    private func collisionColumn(_ title: String, text: String) -> some View {
        VStack(alignment: .leading, spacing: Spacing.xxs) {
            Text(title)
                .font(Typography.caption)
                .foregroundStyle(.secondary)
            Text(text)
                .font(Typography.code)
                .frame(maxWidth: .infinity, alignment: .leading)
                .padding(Spacing.xs)
                .background(Palette.codeBackground, in: RoundedRectangle(cornerRadius: Radius.small, style: .continuous))
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }

    private var syncMode: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: Spacing.xs) {
                ForEach(modes.indices, id: \.self) { index in
                    HStack(spacing: Spacing.sm) {
                        VStack(alignment: .leading, spacing: Spacing.xxs) {
                            Text(modes[index].name)
                                .font(Typography.headline)
                            Text(modes[index].scope)
                                .font(Typography.caption)
                                .foregroundStyle(.secondary)
                        }
                        Spacer(minLength: Spacing.sm)
                        Picker("Mode", selection: $modes[index].mode) {
                            ForEach(SyncModeChoice.allCases) { mode in
                                Text(mode.title).tag(mode)
                            }
                        }
                        .pickerStyle(.segmented)
                        .frame(width: Layout.modePicker)
                        .labelsHidden()
                    }
                    .padding(Spacing.sm)
                    .background(Palette.elevated, in: RoundedRectangle(cornerRadius: Radius.medium, style: .continuous))
                    .overlay(
                        RoundedRectangle(cornerRadius: Radius.medium, style: .continuous)
                            .stroke(Palette.stroke)
                    )
                }
            }
        }
    }
}

struct WizardFrame: View {
    var step: WizardStep
    @State private var claude = true
    @State private var cursor = true
    @State private var codex = true
    @State private var modes = FixtureLibrary.syncTargets

    var body: some View {
        WizardChrome(step: step, onBack: {}, onContinue: {}, onSkip: {}) {
            WizardBody(step: step, claude: $claude, cursor: $cursor, codex: $codex, modes: $modes)
        }
    }
}

struct InteractiveWizard: View {
    @Environment(\.dismiss) private var dismiss
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    @State private var step: WizardStep = .welcome
    @State private var forward = true
    @State private var claude = true
    @State private var cursor = true
    @State private var codex = true
    @State private var modes = FixtureLibrary.syncTargets

    var body: some View {
        WizardChrome(
            step: step,
            onBack: goBack,
            onContinue: goForward,
            onSkip: { dismiss() }
        ) {
            WizardBody(step: step, claude: $claude, cursor: $cursor, codex: $codex, modes: $modes)
                .id(step)
                .transition(Motion.wizardTransition(forward: forward, reduceMotion: reduceMotion))
        }
        .animation(Motion.animation(.step, reduceMotion: reduceMotion), value: step)
        .frame(width: Layout.wizardWidth, height: Layout.wizardHeight)
    }

    private func goForward() {
        guard let next = WizardStep(rawValue: step.rawValue + 1) else {
            dismiss()
            return
        }
        forward = true
        withAnimation(Motion.animation(.step, reduceMotion: reduceMotion)) {
            step = next
        }
    }

    private func goBack() {
        guard let previous = WizardStep(rawValue: step.rawValue - 1) else { return }
        forward = false
        withAnimation(Motion.animation(.step, reduceMotion: reduceMotion)) {
            step = previous
        }
    }
}

#Preview("Welcome") {
    WizardFrame(step: .welcome)
        .frame(width: Layout.wizardWidth, height: Layout.wizardHeight)
}

#Preview("Detect") {
    WizardFrame(step: .detect)
        .frame(width: Layout.wizardWidth, height: Layout.wizardHeight)
}

#Preview("Import") {
    WizardFrame(step: .importExisting)
        .frame(width: Layout.wizardWidth, height: Layout.wizardHeight)
}

#Preview("Sync mode") {
    WizardFrame(step: .syncMode)
        .frame(width: Layout.wizardWidth, height: Layout.wizardHeight)
}

#Preview("Welcome dark") {
    WizardFrame(step: .welcome)
        .frame(width: Layout.wizardWidth, height: Layout.wizardHeight)
        .preferredColorScheme(.dark)
}
