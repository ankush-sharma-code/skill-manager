import AppKit
import SwiftUI
import XCTest
@testable import SkillManager

@MainActor
final class MockRenderTests: XCTestCase {
    func testReduceMotionFallsBack() {
        XCTAssertNil(Motion.animation(.snappy, reduceMotion: true))
        XCTAssertNil(Motion.animation(.reveal, reduceMotion: true))
        XCTAssertNotNil(Motion.animation(.fade, reduceMotion: true))
        XCTAssertNotNil(Motion.animation(.step, reduceMotion: true))
        XCTAssertNotNil(Motion.animation(.snappy, reduceMotion: false))
        XCTAssertEqual(Motion.snappyResponse, 0.3)
        XCTAssertEqual(Motion.snappyDamping, 0.8)
        XCTAssertEqual(Motion.fadeDuration, 0.2)
        XCTAssertLessThanOrEqual(Motion.revealStagger + Motion.revealResponse, Motion.revealBudget)
        XCTAssertLessThanOrEqual(Motion.revealDelay(index: 29, count: 30), Motion.revealStagger)
    }

    func testPaletteGroupsAndFilter() {
        let all = PaletteCatalog.groups(skills: FixtureLibrary.skills, query: "")
        XCTAssertEqual(all.map(\.title), ["Skills", "Targets", "Actions"])
        let filtered = PaletteCatalog.groups(skills: FixtureLibrary.skills, query: "sync")
        let titles = filtered.flatMap(\.entries).map(\.title)
        XCTAssertTrue(titles.contains("Sync now"))
        XCTAssertFalse(titles.contains("architect"))
    }

    func testFixtureSlotCounts() {
        let model = DemoModel()
        XCTAssertEqual(model.conflictCount, 2)
        XCTAssertEqual(model.slotCount, 21)
        XCTAssertEqual(model.displayedSynced, 15)
        XCTAssertEqual(model.topSkills.map(\.name), ["poteto-mode", "architect", "arena", "tdd", "how"])
    }

    func testHighlighterMarksKeywordsAndComments() {
        let runs = CodeHighlighter.highlight("func animation() { return nil } // note")
        XCTAssertTrue(runs.contains { $0.text == "func" && $0.kind == .keyword })
        XCTAssertTrue(runs.contains { $0.text == "return" && $0.kind == .keyword })
        XCTAssertTrue(runs.contains { $0.kind == .comment && $0.text.contains("note") })
    }

    func testScreenNamesAreUnique() {
        let names = MockScreen.allCases.map(\.fileName)
        XCTAssertEqual(names.count, Set(names).count)
        XCTAssertEqual(names.count, 12)
    }

    func testRendersLightAndDark() throws {
        let root = ArtifactPaths.renders
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
        for screen in MockScreen.allCases {
            for scheme in [ColorScheme.light, ColorScheme.dark] {
                let label = scheme == .dark ? "dark" : "light"
                let name = "\(screen.fileName)-\(label).png"
                let data = try render(screen, scheme: scheme)
                let url = root.appendingPathComponent(name)
                try data.write(to: url)
                let rep = NSBitmapImageRep(data: data)
                XCTAssertGreaterThan(rep?.pixelsWide ?? 0, 200, name)
                XCTAssertGreaterThan(data.count, 5_000, name)
                if screen == .errorState {
                    XCTAssertTrue(hasAccentFill(data, scheme: scheme), "\(name) primary button is not accent filled")
                }
                print("MOCK_RENDER_WROTE \(url.path) \(data.count)")
            }
        }
        print("MOCK_RENDER_DONE")
    }

    private func render(_ screen: MockScreen, scheme: ColorScheme) throws -> Data {
        let size = screen.size
        let view = screen.view(playsIntro: false)
            .environment(\.colorScheme, scheme)
            .preferredColorScheme(scheme)
            .frame(width: size.width, height: size.height)
        let appearance = NSAppearance(named: scheme == .dark ? .darkAqua : .aqua)
        let hosted = try renderHosted(
            view,
            size: size,
            appearance: appearance,
            activates: screen == .errorState
        )
        if isUseful(hosted) {
            return hosted
        }
        var rendered: Data?
        appearance?.performAsCurrentDrawingAppearance {
            let renderer = ImageRenderer(content: view)
            renderer.scale = 2
            renderer.isOpaque = true
            renderer.proposedSize = ProposedViewSize(width: size.width, height: size.height)
            if let cgImage = renderer.cgImage {
                rendered = NSBitmapImageRep(cgImage: cgImage).representation(using: .png, properties: [:])
            }
        }
        if let rendered, isUseful(rendered) {
            return rendered
        }
        throw CocoaError(.fileWriteUnknown)
    }

    private func renderHosted(
        _ view: some View,
        size: CGSize,
        appearance: NSAppearance?,
        activates: Bool = false
    ) throws -> Data {
        let hosting = NSHostingView(rootView: view)
        hosting.appearance = appearance
        hosting.frame = NSRect(origin: .zero, size: size)
        let window = SnapshotWindow(
            contentRect: NSRect(origin: .zero, size: size),
            styleMask: [.borderless],
            backing: .buffered,
            defer: false
        )
        window.canTakeKey = activates
        window.appearance = appearance
        window.isReleasedWhenClosed = false
        window.contentView = hosting
        window.setContentSize(size)
        if activates {
            NSApp.setActivationPolicy(.regular)
            NSApp.activate(ignoringOtherApps: true)
            window.makeKeyAndOrderFront(nil)
        } else {
            window.orderFrontRegardless()
        }
        window.displayIfNeeded()
        hosting.layoutSubtreeIfNeeded()
        let settle = activates ? 0.25 : 0.12
        RunLoop.current.run(mode: .default, before: Date().addingTimeInterval(settle))
        let scale = max(window.backingScaleFactor, 2)
        let pixelsWide = Int(size.width * scale)
        let pixelsHigh = Int(size.height * scale)
        guard let context = CGContext(
            data: nil,
            width: pixelsWide,
            height: pixelsHigh,
            bitsPerComponent: 8,
            bytesPerRow: 0,
            space: CGColorSpaceCreateDeviceRGB(),
            bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue
        ) else {
            window.close()
            throw CocoaError(.fileWriteUnknown)
        }
        context.translateBy(x: 0, y: CGFloat(pixelsHigh))
        context.scaleBy(x: scale, y: -scale)
        if hosting.layer == nil {
            hosting.wantsLayer = true
        }
        hosting.layer?.render(in: context)
        window.contentView = NSView(frame: NSRect(origin: .zero, size: size))
        window.orderOut(nil)
        window.close()
        guard
            let cgImage = context.makeImage(),
            let data = NSBitmapImageRep(cgImage: cgImage).representation(using: .png, properties: [:])
        else {
            throw CocoaError(.fileWriteUnknown)
        }
        return data
    }

    private func isUseful(_ data: Data) -> Bool {
        guard let rep = NSBitmapImageRep(data: data), rep.pixelsWide > 0, rep.pixelsHigh > 0 else {
            return false
        }
        var colors = Set<Int>()
        var yellow = 0
        var total = 0
        let step = 16
        for y in stride(from: 0, to: rep.pixelsHigh, by: step) {
            for x in stride(from: 0, to: rep.pixelsWide, by: step) {
                guard let color = rep.colorAt(x: x, y: y) else { continue }
                total += 1
                if color.redComponent > 0.9, color.greenComponent > 0.7, color.blueComponent < 0.25 {
                    yellow += 1
                }
                let key = (Int(color.redComponent * 8) << 6)
                    | (Int(color.greenComponent * 8) << 3)
                    | Int(color.blueComponent * 8)
                colors.insert(key)
            }
        }
        guard total > 0 else { return false }
        if Double(yellow) / Double(total) > 0.45 { return false }
        return colors.count >= 8
    }

    private func hasAccentFill(_ data: Data, scheme: ColorScheme) -> Bool {
        guard let rep = NSBitmapImageRep(data: data) else { return false }
        var hits = 0
        let step = 4
        for y in stride(from: 0, to: rep.pixelsHigh, by: step) {
            for x in stride(from: 0, to: rep.pixelsWide, by: step) {
                guard let color = rep.colorAt(x: x, y: y) else { continue }
                let red = color.redComponent
                let green = color.greenComponent
                let blue = color.blueComponent
                let matches = scheme == .dark
                    ? red > 0.12 && red < 0.40 && green > 0.45 && green < 0.75 && blue > 0.85
                    : red < 0.18 && green > 0.32 && green < 0.68 && blue > 0.85
                if matches {
                    hits += 1
                }
            }
        }
        return hits > 40
    }
}

private final class SnapshotWindow: NSWindow {
    var canTakeKey = false

    override var canBecomeKey: Bool { canTakeKey }
    override var canBecomeMain: Bool { canTakeKey }
}

enum ArtifactPaths {
    static var renders: URL {
        if let value = ProcessInfo.processInfo.environment["MOCK_OUTPUT"],
           !value.isEmpty,
           !value.contains("$") {
            return URL(fileURLWithPath: value, isDirectory: true)
        }
        return URL(fileURLWithPath: "/tmp/skill-manager-renders", isDirectory: true)
    }
}
