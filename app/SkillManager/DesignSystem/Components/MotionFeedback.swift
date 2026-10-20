import SwiftUI

struct MotionPressModifier: ViewModifier {
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    @State private var hovered = false
    @State private var pressed = false

    func body(content: Content) -> some View {
        content
            .scaleEffect(pressed && !reduceMotion ? 0.97 : 1)
            .brightness(hovered && !pressed ? 0.03 : 0)
            .onHover { hovering in
                withAnimation(Motion.animation(.fade, reduceMotion: reduceMotion)) {
                    hovered = hovering
                }
            }
            .simultaneousGesture(pressGesture)
            .animation(Motion.animation(.snappy, reduceMotion: reduceMotion), value: pressed)
    }

    private var pressGesture: some Gesture {
        DragGesture(minimumDistance: 0)
            .onChanged { _ in
                if !pressed {
                    withAnimation(Motion.animation(.snappy, reduceMotion: reduceMotion)) {
                        pressed = true
                    }
                }
            }
            .onEnded { _ in
                withAnimation(Motion.animation(.snappy, reduceMotion: reduceMotion)) {
                    pressed = false
                }
            }
    }
}

struct MotionRowModifier: ViewModifier {
    var isSelected: Bool
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    @State private var hovered = false
    @State private var pressed = false

    func body(content: Content) -> some View {
        content
            .background {
                RoundedRectangle(cornerRadius: Radius.medium, style: .continuous)
                    .fill(fill)
            }
            .scaleEffect(pressed && !reduceMotion ? 0.985 : 1)
            .onHover { hovering in
                withAnimation(Motion.animation(.fade, reduceMotion: reduceMotion)) {
                    hovered = hovering
                }
            }
            .simultaneousGesture(pressGesture)
            .animation(Motion.animation(.snappy, reduceMotion: reduceMotion), value: pressed)
    }

    private var fill: AnyShapeStyle {
        if isSelected {
            return AnyShapeStyle(Palette.selection)
        }
        if hovered {
            return AnyShapeStyle(Palette.hover)
        }
        return AnyShapeStyle(Color.clear)
    }

    private var pressGesture: some Gesture {
        DragGesture(minimumDistance: 0)
            .onChanged { _ in
                if !pressed {
                    withAnimation(Motion.animation(.snappy, reduceMotion: reduceMotion)) {
                        pressed = true
                    }
                }
            }
            .onEnded { _ in
                withAnimation(Motion.animation(.snappy, reduceMotion: reduceMotion)) {
                    pressed = false
                }
            }
    }
}

extension View {
    func motionPress() -> some View {
        modifier(MotionPressModifier())
    }

    func motionRow(isSelected: Bool) -> some View {
        modifier(MotionRowModifier(isSelected: isSelected))
    }
}
