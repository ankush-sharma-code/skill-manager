import SwiftUI

enum CodeKind: Equatable {
    case plain
    case keyword
    case string
    case comment
}

struct CodeRun: Equatable {
    var text: String
    var kind: CodeKind
}

enum CodeHighlighter {
    private static let keywords: Set<String> = [
        "func", "let", "var", "if", "else", "return", "struct", "import",
        "true", "false", "for", "in", "guard", "nil", "switch", "case"
    ]

    static func lines(in source: String) -> [[CodeRun]] {
        source.split(separator: "\n", omittingEmptySubsequences: false).map { line in
            highlight(String(line))
        }
    }

    static func highlight(_ line: String) -> [CodeRun] {
        if let range = line.range(of: "//") {
            let code = String(line[..<range.lowerBound])
            let comment = String(line[range.lowerBound...])
            return runs(in: code) + [CodeRun(text: comment, kind: .comment)]
        }
        return runs(in: line)
    }

    private static func runs(in line: String) -> [CodeRun] {
        var result: [CodeRun] = []
        var token = ""
        var literal = ""
        var inString = false

        func flushToken() {
            guard !token.isEmpty else { return }
            let kind: CodeKind = keywords.contains(token) ? .keyword : .plain
            result.append(CodeRun(text: token, kind: kind))
            token = ""
        }

        func flushLiteral() {
            guard !literal.isEmpty else { return }
            result.append(CodeRun(text: literal, kind: .string))
            literal = ""
        }

        for character in line {
            if character == "\"" {
                if inString {
                    literal.append(character)
                    flushLiteral()
                    inString = false
                } else {
                    flushToken()
                    inString = true
                    literal.append(character)
                }
                continue
            }
            if inString {
                literal.append(character)
                continue
            }
            if character.isLetter || character == "_" {
                token.append(character)
            } else {
                flushToken()
                result.append(CodeRun(text: String(character), kind: .plain))
            }
        }
        if inString {
            flushLiteral()
        } else {
            flushToken()
        }
        return result.isEmpty ? [CodeRun(text: " ", kind: .plain)] : result
    }
}

struct CodePaint: ShapeStyle {
    var kind: CodeKind

    func resolve(in environment: EnvironmentValues) -> Color {
        let dark = environment.colorScheme == .dark
        switch kind {
        case .plain:
            return Color.primary
        case .keyword:
            return dark ? Color(red: 0.96, green: 0.47, blue: 0.76) : Color(red: 0.68, green: 0.20, blue: 0.55)
        case .string:
            return dark ? Color(red: 0.98, green: 0.45, blue: 0.40) : Color(red: 0.77, green: 0.16, blue: 0.14)
        case .comment:
            return dark ? Color(red: 0.45, green: 0.78, blue: 0.48) : Color(red: 0.22, green: 0.48, blue: 0.27)
        }
    }
}

struct CodeBlockView: View {
    var language: String
    var source: String

    var body: some View {
        VStack(alignment: .leading, spacing: Spacing.xs) {
            Text(language)
                .font(Typography.caption)
                .foregroundStyle(.secondary)
            VStack(alignment: .leading, spacing: Spacing.xxs) {
                ForEach(Array(CodeHighlighter.lines(in: source).enumerated()), id: \.offset) { _, runs in
                    line(runs)
                        .frame(maxWidth: .infinity, alignment: .leading)
                }
            }
        }
        .padding(Spacing.sm)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(Palette.codeBackground, in: RoundedRectangle(cornerRadius: Radius.medium, style: .continuous))
        .overlay(
            RoundedRectangle(cornerRadius: Radius.medium, style: .continuous)
                .stroke(Palette.stroke)
        )
        .accessibilityLabel("\(language) code")
    }

    private func line(_ runs: [CodeRun]) -> Text {
        runs.reduce(Text("")) { partial, run in
            partial + Text(run.text).foregroundStyle(CodePaint(kind: run.kind))
        }
        .font(Typography.code)
    }
}

#Preview("Code block") {
    CodeBlockView(
        language: "swift",
        source: """
        func animation(reduceMotion: Bool) -> Animation? {
            if reduceMotion { return nil }
            return .spring(response: 0.3, dampingFraction: 0.8)
        }
        """
    )
    .padding(Spacing.lg)
    .frame(width: 560)
}

#Preview("Code block dark") {
    CodeBlockView(
        language: "swift",
        source: """
        func animation(reduceMotion: Bool) -> Animation? {
            // Reduce Motion drops the spring.
            if reduceMotion { return nil }
            return .spring(response: 0.3, dampingFraction: 0.8)
        }
        """
    )
    .padding(Spacing.lg)
    .frame(width: 560)
    .background(Palette.page)
    .preferredColorScheme(.dark)
}
