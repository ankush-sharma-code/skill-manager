import SwiftUI

struct FrontmatterChip: View {
    var key: String
    var value: String

    var body: some View {
        HStack(spacing: Spacing.xxs) {
            Text(key)
                .foregroundStyle(.secondary)
            Text(value)
                .foregroundStyle(.primary)
                .lineLimit(1)
        }
        .font(Typography.caption)
        .padding(.horizontal, Spacing.xs)
        .padding(.vertical, Spacing.xxs)
        .frame(maxWidth: Measure.chipMax, alignment: .leading)
        .background(Palette.chip, in: Capsule())
        .overlay(Capsule().stroke(Palette.stroke))
        .accessibilityLabel("\(key) \(value)")
    }
}

#Preview("Frontmatter chip") {
    HStack(spacing: Spacing.xs) {
        FrontmatterChip(key: "name", value: "poteto-mode")
        FrontmatterChip(key: "tools", value: "cursor")
        FrontmatterChip(key: "version", value: "1")
    }
    .padding(Spacing.lg)
}
