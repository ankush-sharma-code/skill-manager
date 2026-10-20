import SwiftUI

enum MotionToken: Equatable {
    case snappy
    case fade
    case step
    case reveal
}

enum Motion {
    static let snappyResponse = 0.3
    static let snappyDamping = 0.8
    static let fadeDuration = 0.2
    static let stepResponse = 0.32
    static let stepDamping = 0.86
    static let revealResponse = 0.22
    static let revealDamping = 0.86
    static let revealBudget = 0.4
    static let revealStagger = 0.12

    static func animation(_ token: MotionToken, reduceMotion: Bool) -> Animation? {
        if reduceMotion {
            switch token {
            case .snappy, .reveal:
                return nil
            case .fade, .step:
                return .easeInOut(duration: fadeDuration)
            }
        }
        switch token {
        case .snappy:
            return .spring(response: snappyResponse, dampingFraction: snappyDamping)
        case .fade:
            return .easeInOut(duration: fadeDuration)
        case .step:
            return .spring(response: stepResponse, dampingFraction: stepDamping)
        case .reveal:
            return .spring(response: revealResponse, dampingFraction: revealDamping)
        }
    }

    static func revealDelay(index: Int, count: Int) -> Double {
        guard count > 1, index > 0 else { return 0 }
        let clamped = min(index, count - 1)
        return revealStagger * Double(clamped) / Double(count - 1)
    }

    static func wizardTransition(forward: Bool, reduceMotion: Bool) -> AnyTransition {
        if reduceMotion {
            return .opacity
        }
        let edgeIn: Edge = forward ? .trailing : .leading
        let edgeOut: Edge = forward ? .leading : .trailing
        return .asymmetric(
            insertion: .move(edge: edgeIn).combined(with: .opacity),
            removal: .move(edge: edgeOut).combined(with: .opacity)
        )
    }
}

struct SymbolBounceModifier<Value: Equatable>: ViewModifier {
    var value: Value
    var reduceMotion: Bool

    func body(content: Content) -> some View {
        if reduceMotion {
            content
        } else {
            content.symbolEffect(.bounce, options: .nonRepeating, value: value)
        }
    }
}

extension View {
    func motionSymbolBounce<Value: Equatable>(value: Value, reduceMotion: Bool) -> some View {
        modifier(SymbolBounceModifier(value: value, reduceMotion: reduceMotion))
    }

    @ViewBuilder
    func skillTitleMorph(id: String, namespace: Namespace.ID, enabled: Bool, isSource: Bool) -> some View {
        if enabled {
            matchedGeometryEffect(id: "title-\(id)", in: namespace, isSource: isSource)
        } else {
            self
        }
    }
}
