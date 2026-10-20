import SwiftUI

struct StatusDot: View {
    var kind: StatusKind
    var showsCheck: Bool = false
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    var body: some View {
        Image(systemName: showsCheck ? "checkmark.circle.fill" : "circle.fill")
            .font(Typography.headline)
            .foregroundStyle(StatusColor.paint(showsCheck ? .ok : kind))
            .contentTransition(.symbolEffect(.replace))
            .motionSymbolBounce(value: showsCheck, reduceMotion: reduceMotion)
            .animation(Motion.animation(.fade, reduceMotion: reduceMotion), value: showsCheck)
            .accessibilityLabel(showsCheck ? "Sync complete" : kind.title)
    }
}

#Preview("Status dot") {
    HStack(spacing: Spacing.sm) {
        StatusDot(kind: .ok)
        StatusDot(kind: .attention)
        StatusDot(kind: .conflict)
        StatusDot(kind: .ok, showsCheck: true)
    }
    .padding(Spacing.lg)
}

#Preview("Status dot dark") {
    HStack(spacing: Spacing.sm) {
        StatusDot(kind: .ok)
        StatusDot(kind: .attention)
        StatusDot(kind: .conflict)
        StatusDot(kind: .ok, showsCheck: true)
    }
    .padding(Spacing.lg)
    .background(Palette.page)
    .preferredColorScheme(.dark)
}
