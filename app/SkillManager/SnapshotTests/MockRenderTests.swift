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
        XCTAssertEqual(names.count, 11)
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
                let attachment = XCTAttachment(data: data, uniformTypeIdentifier: "public.png")
                attachment.name = name
                attachment.lifetime = .keepAlways
                add(attachment)
                let rep = NSBitmapImageRep(data: data)
                XCTAssertGreaterThan(rep?.pixelsWide ?? 0, 200, name)
                XCTAssertGreaterThan(data.count, 5_000, name)
                print("MOCK_RENDER_WROTE \(url.path) \(data.count)")
            }
        }
    }

    private func render(_ screen: MockScreen, scheme: ColorScheme) throws -> Data {
        let size = screen.size
        let view = screen.view(playsIntro: false)
            .environment(\.colorScheme, scheme)
            .preferredColorScheme(scheme)
            .frame(width: size.width, height: size.height)
        let renderer = ImageRenderer(content: view)
        renderer.scale = 2
        renderer.isOpaque = true
        renderer.proposedSize = ProposedViewSize(width: size.width, height: size.height)
        let appearance = NSAppearance(named: scheme == .dark ? .darkAqua : .aqua)
        var image: NSImage?
        appearance?.performAsCurrentDrawingAppearance {
            image = renderer.nsImage
        }
        guard
            let image,
            let tiff = image.tiffRepresentation,
            let rep = NSBitmapImageRep(data: tiff),
            let data = rep.representation(using: .png, properties: [:])
        else {
            XCTFail("No image for \(screen.fileName)")
            return Data()
        }
        return data
    }
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
