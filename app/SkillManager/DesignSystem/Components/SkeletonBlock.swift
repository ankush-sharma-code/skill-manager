import SwiftUI

struct SkeletonBlock: View {
    var height: CGFloat = Spacing.sm

    var body: some View {
        RoundedRectangle(cornerRadius: Radius.small, style: .continuous)
            .fill(Palette.track)
            .frame(height: height)
    }
}

struct SkeletonRows: View {
    var count: Int = 6

    var body: some View {
        VStack(alignment: .leading, spacing: Spacing.sm) {
            ForEach(0..<count, id: \.self) { index in
                VStack(alignment: .leading, spacing: Spacing.xs) {
                    SkeletonBlock()
                        .frame(width: index.isMultiple(of: 2) ? 180 : 140)
                    SkeletonBlock(height: Spacing.xs)
                        .frame(maxWidth: index.isMultiple(of: 3) ? 220 : 260)
                }
                .padding(.vertical, Spacing.xxs)
            }
        }
        .padding(Spacing.sm)
        .accessibilityLabel("Loading skills")
    }
}

#Preview("Skeleton") {
    SkeletonRows()
        .frame(width: 320)
}
