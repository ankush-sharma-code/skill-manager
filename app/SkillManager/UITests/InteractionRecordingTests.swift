import XCTest

final class InteractionRecordingTests: XCTestCase {
    func testRecordsKeyInteractions() throws {
        let video = URL(fileURLWithPath: "/tmp/skill-manager-interaction-light.mov")
        let logURL = URL(fileURLWithPath: "/tmp/skill-manager-screencapture.log")
        try? FileManager.default.removeItem(at: video)
        try? FileManager.default.removeItem(at: logURL)
        FileManager.default.createFile(atPath: logURL.path, contents: nil)
        let logHandle = try FileHandle(forWritingTo: logURL)

        let capture = Process()
        capture.executableURL = URL(fileURLWithPath: "/usr/sbin/screencapture")
        capture.arguments = ["-v", "-V", "28", video.path]
        capture.standardOutput = logHandle
        capture.standardError = logHandle
        try capture.run()

        let app = XCUIApplication()
        app.launch()
        let window = app.windows.firstMatch
        XCTAssertTrue(window.waitForExistence(timeout: 20), app.debugDescription)
        app.activate()
        pause(1.0)

        let architect = app.buttons["skill-row-architect"]
        XCTAssertTrue(architect.waitForExistence(timeout: 8), app.debugDescription)
        architect.hover()
        pause(0.6)
        architect.click()
        pause(0.9)

        let tdd = app.buttons["skill-row-tdd"]
        XCTAssertTrue(tdd.waitForExistence(timeout: 4))
        tdd.hover()
        pause(0.5)
        tdd.click()
        pause(0.9)

        let sync = app.buttons["sync-now"]
        XCTAssertTrue(sync.waitForExistence(timeout: 4))
        sync.hover()
        pause(0.3)
        sync.click()
        pause(1.1)

        app.typeKey("k", modifierFlags: .command)
        let search = app.textFields["palette-search"]
        XCTAssertTrue(search.waitForExistence(timeout: 4), app.debugDescription)
        search.click()
        search.typeText("sync")
        pause(0.9)
        app.typeKey(.escape, modifierFlags: [])
        pause(0.4)

        app.typeKey("l", modifierFlags: [.command, .shift])
        let forward = app.buttons["wizard-continue"]
        XCTAssertTrue(forward.waitForExistence(timeout: 4), app.debugDescription)
        pause(0.6)
        forward.click()
        pause(0.8)
        forward.click()
        pause(0.8)
        app.buttons["wizard-back"].click()
        pause(0.8)

        if capture.isRunning {
            capture.waitUntilExit()
        }
        try logHandle.close()
        let log = (try? String(contentsOf: logURL, encoding: .utf8)) ?? ""
        let size = (try? FileManager.default.attributesOfItem(atPath: video.path)[.size] as? Int) ?? 0
        print("DEMO_VIDEO \(video.path) bytes \(size)")
        print(log)
        XCTAssertGreaterThan(size, 80_000, "recording was \(size) bytes. \(log)")
    }

    private func pause(_ seconds: TimeInterval) {
        Thread.sleep(forTimeInterval: seconds)
    }
}
