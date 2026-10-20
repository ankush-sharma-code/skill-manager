import SwiftUI

struct SectionCard<Content: View>: View {
    var title: String
    @ViewBuilder var content: () -> Content

    var body: some View {
        VStack(alignment: .leading, spacing: Spacing.xs) {
            Text(title)
                .font(Typography.headline)
            content()
        }
        .padding(Spacing.sm)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(Palette.elevated, in: RoundedRectangle(cornerRadius: Radius.large, style: .continuous))
        .overlay(
            RoundedRectangle(cornerRadius: Radius.large, style: .continuous)
                .stroke(Palette.stroke)
        )
    }
}

#Preview("Section card") {
    SectionCard(title: "Targets") {
        Text("Claude, Cursor, and Codex")
            .font(Typography.body)
            .foregroundStyle(.secondary)
    }
    .padding(Spacing.lg)
    .frame(width: 420)
}
