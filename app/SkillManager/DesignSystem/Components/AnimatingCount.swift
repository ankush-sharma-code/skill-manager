import SwiftUI

struct AnimatingCount: View {
    var value: Int
    var font: Font
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    var body: some View {
        Text(value, format: .number)
            .font(font)
            .monospacedDigit()
            .contentTransition(.numericText())
            .animation(Motion.animation(.snappy, reduceMotion: reduceMotion), value: value)
    }
}

#Preview("Animating count") {
    AnimatingCount(value: 128, font: Typography.headline)
        .padding(Spacing.lg)
}
