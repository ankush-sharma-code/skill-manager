import SwiftUI

struct Sparkline: View {
    var values: [Int]
    var playsIntro: Bool
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    @State private var revealed = false

    var body: some View {
        GeometryReader { geo in
            let count = CGFloat(max(values.count, 1))
            let slot = geo.size.width / count
            let gap = slot * 0.28
            let barWidth = max(1, slot - gap)
            HStack(alignment: .bottom, spacing: gap) {
                ForEach(Array(values.enumerated()), id: \.offset) { index, value in
                    RoundedRectangle(cornerRadius: Radius.bar, style: .continuous)
                        .fill(Color.accentColor.opacity(barOpacity(index: index)))
                        .frame(width: barWidth, height: barHeight(value, plot: geo.size.height))
                        .scaleEffect(y: shown ? 1 : 0.02, anchor: .bottom)
                        .animation(revealAnimation(index: index), value: revealed)
                }
            }
            .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .bottomLeading)
        }
        .frame(height: Layout.sparklineHeight)
        .accessibilityLabel("Usage over 30 days")
        .onAppear {
            guard playsIntro, !reduceMotion else {
                revealed = true
                return
            }
            revealed = true
        }
        .onChange(of: values) { _, _ in
            revealed = true
        }
    }

    private var shown: Bool {
        !playsIntro || reduceMotion || revealed
    }

    private func barHeight(_ value: Int, plot: CGFloat) -> CGFloat {
        let peak = max(values.max() ?? 1, 1)
        let ratio = CGFloat(value) / CGFloat(peak)
        return max(plot * 0.08, plot * ratio)
    }

    private func barOpacity(index: Int) -> Double {
        guard values.count > 1 else { return 1 }
        let t = Double(index) / Double(values.count - 1)
        return 0.35 + (0.65 * t)
    }

    private func revealAnimation(index: Int) -> Animation? {
        guard playsIntro, !reduceMotion else { return nil }
        return Motion.animation(.reveal, reduceMotion: false)?
            .delay(Motion.revealDelay(index: index, count: values.count))
    }
}

#Preview("Sparkline") {
    Sparkline(values: [2, 4, 3, 8, 6, 1, 0, 5, 9, 7, 4, 8, 11, 6, 3], playsIntro: false)
        .frame(width: Layout.sparklineWidth)
        .padding(Spacing.lg)
}

#Preview("Sparkline dark") {
    Sparkline(values: [2, 4, 3, 8, 6, 1, 0, 5, 9, 7, 4, 8, 11, 6, 3], playsIntro: false)
        .frame(width: Layout.sparklineWidth)
        .padding(Spacing.lg)
        .background(Palette.page)
        .preferredColorScheme(.dark)
}
