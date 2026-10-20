import SwiftUI

enum PaletteKind: String, Equatable, Hashable {
    case skill
    case target
    case action
}

struct PaletteEntry: Identifiable, Equatable, Hashable {
    var id: String
    var kind: PaletteKind
    var title: String
    var subtitle: String
    var symbol: String
    var shortcut: String?
}

struct PaletteGroup: Identifiable, Equatable {
    var title: String
    var entries: [PaletteEntry]
    var id: String { title }
}

struct QuickSearchPalette: View {
    var groups: [PaletteGroup]
    @Binding var query: String
    var onSubmit: (PaletteEntry) -> Void
    var onDismiss: () -> Void
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    @Environment(\.colorScheme) private var colorScheme
    @FocusState private var focused: Bool
    @State private var selection = 0

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            searchField
            Divider()
            results
        }
        .frame(maxWidth: .infinity, maxHeight: Measure.paletteHeight, alignment: .top)
        .background {
            RoundedRectangle(cornerRadius: Radius.large, style: .continuous)
                .fill(Palette.elevated)
        }
        .overlay(
            RoundedRectangle(cornerRadius: Radius.large, style: .continuous)
                .stroke(Palette.stroke)
        )
        .clipShape(RoundedRectangle(cornerRadius: Radius.large, style: .continuous))
        .shadow(color: Palette.shadow(colorScheme), radius: Spacing.md, y: Spacing.xs)
        .accessibilityIdentifier("command-palette")
        .onAppear { focused = true }
        .onExitCommand(perform: onDismiss)
        .onChange(of: query) { _, _ in
            selection = 0
        }
    }

    private var flat: [PaletteEntry] {
        groups.flatMap(\.entries)
    }

    private var searchField: some View {
        HStack(spacing: Spacing.xs) {
            Image(systemName: "magnifyingglass")
                .font(Typography.body)
                .foregroundStyle(.secondary)
            TextField("Skills, targets, and actions", text: $query)
                .textFieldStyle(.plain)
                .font(Typography.body)
                .focused($focused)
                .accessibilityIdentifier("palette-search")
                .onSubmit { submitSelected() }
        }
        .padding(Spacing.sm)
        .onKeyPress(.downArrow) {
            move(1)
            return .handled
        }
        .onKeyPress(.upArrow) {
            move(-1)
            return .handled
        }
    }

    private var results: some View {
        ScrollView {
            if flat.isEmpty {
                Text("No matches")
                    .font(Typography.callout)
                    .foregroundStyle(.secondary)
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .padding(Spacing.sm)
            } else {
                LazyVStack(alignment: .leading, spacing: Spacing.xxs) {
                    ForEach(groups) { group in
                        if !group.entries.isEmpty {
                            Text(group.title)
                                .font(Typography.caption)
                                .foregroundStyle(.secondary)
                                .padding(.horizontal, Spacing.sm)
                                .padding(.top, Spacing.xs)
                            ForEach(group.entries) { entry in
                                row(entry)
                            }
                        }
                    }
                }
                .padding(.bottom, Spacing.xs)
            }
        }
    }

    private func row(_ entry: PaletteEntry) -> some View {
        let index = flat.firstIndex(of: entry) ?? 0
        let isSelected = index == selection
        return Button {
            onSubmit(entry)
        } label: {
            HStack(spacing: Spacing.xs) {
                Image(systemName: entry.symbol)
                    .font(Typography.body)
                    .foregroundStyle(isSelected ? Color.accentColor : Color.secondary)
                    .frame(width: Spacing.md)
                VStack(alignment: .leading, spacing: Spacing.xxs) {
                    Text(entry.title)
                        .font(Typography.body)
                        .foregroundStyle(.primary)
                    if !entry.subtitle.isEmpty {
                        Text(entry.subtitle)
                            .font(Typography.caption)
                            .foregroundStyle(.secondary)
                            .lineLimit(1)
                    }
                }
                Spacer(minLength: Spacing.xs)
                if let shortcut = entry.shortcut {
                    Text(shortcut)
                        .font(Typography.caption)
                        .foregroundStyle(.secondary)
                }
            }
            .padding(.horizontal, Spacing.sm)
            .padding(.vertical, Spacing.xs)
            .frame(maxWidth: .infinity, alignment: .leading)
            .background {
                RoundedRectangle(cornerRadius: Radius.medium, style: .continuous)
                    .fill(isSelected ? AnyShapeStyle(Palette.selection) : AnyShapeStyle(Color.clear))
            }
            .padding(.horizontal, Spacing.xs)
        }
        .buttonStyle(.plain)
        .motionRow(isSelected: false)
        .accessibilityIdentifier("palette-\(entry.id)")
        .animation(Motion.animation(.fade, reduceMotion: reduceMotion), value: isSelected)
    }

    private func move(_ delta: Int) {
        guard !flat.isEmpty else { return }
        let next = min(max(selection + delta, 0), flat.count - 1)
        withAnimation(Motion.animation(.snappy, reduceMotion: reduceMotion)) {
            selection = next
        }
    }

    private func submitSelected() {
        guard flat.indices.contains(selection) else { return }
        onSubmit(flat[selection])
    }
}

#Preview("Quick search") {
    QuickSearchPreview()
        .padding(Spacing.lg)
        .frame(width: 640, height: 480)
}

private struct QuickSearchPreview: View {
    @State private var query = ""

    var body: some View {
        QuickSearchPalette(
            groups: [
                PaletteGroup(title: "Skills", entries: [
                    PaletteEntry(id: "poteto-mode", kind: .skill, title: "poteto-mode", subtitle: "Agent style", symbol: "doc.text", shortcut: nil)
                ]),
                PaletteGroup(title: "Actions", entries: [
                    PaletteEntry(id: "action-sync", kind: .action, title: "Sync now", subtitle: "Write the current skills", symbol: "arrow.clockwise", shortcut: "⌘R")
                ])
            ],
            query: $query,
            onSubmit: { _ in },
            onDismiss: {}
        )
    }
}
