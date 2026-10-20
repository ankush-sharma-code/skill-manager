import AVFoundation
import AppKit
import CoreVideo
import XCTest

final class InteractionRecordingTests: XCTestCase {
    func testRecordsKeyInteractions() throws {
        continueAfterFailure = true
        let videoURL = URL(fileURLWithPath: "/tmp/skill-manager-interaction-light.mov")
        try? FileManager.default.removeItem(at: videoURL)

        let log = Pipe()
        let capture = Process()
        capture.executableURL = URL(fileURLWithPath: "/usr/sbin/screencapture")
        capture.arguments = ["-v", "-V", "22", videoURL.path]
        capture.standardOutput = log
        capture.standardError = log
        try? capture.run()

        let app = XCUIApplication()
        app.launchEnvironment["SKILL_MANAGER_APPEARANCE"] = "light"
        var frames: [NSImage] = []
        var notes: [String] = []

        app.launch()
        let window = app.windows.firstMatch
        if window.waitForExistence(timeout: 20) {
            app.activate()
            pause(1.1, frames: &frames)
        } else {
            notes.append("window missing")
        }

        let architect = app.buttons["skill-row-architect"]
        if architect.waitForExistence(timeout: 8) {
            tap(architect, name: "architect row", notes: &notes)
            pause(0.7, frames: &frames)
        } else {
            notes.append("architect row missing")
        }

        let tdd = app.buttons["skill-row-tdd"]
        if tdd.waitForExistence(timeout: 4) {
            tap(tdd, name: "tdd row", notes: &notes)
            pause(0.45, frames: &frames)
        } else {
            notes.append("tdd row missing")
        }

        let badge = app.buttons["conflict-badge-tdd"]
        if badge.waitForExistence(timeout: 4) {
            let ready = Date().addingTimeInterval(3)
            while Date() < ready, !badge.isHittable {
                Thread.sleep(forTimeInterval: 0.15)
            }
            print("BADGE frame \(badge.frame) hittable \(badge.isHittable) window \(window.frame)")
            tap(badge, name: "conflict badge", notes: &notes)
            let resolve = app.descendants(matching: .any)["conflict-resolve"]
            if resolve.waitForExistence(timeout: 3) {
                pause(0.8, frames: &frames)
            } else {
                notes.append("conflict screen missing")
                pause(0.4, frames: &frames)
            }
        } else {
            notes.append("conflict badge missing")
        }

        let sync = app.buttons["sync-now"]
        if sync.waitForExistence(timeout: 4) {
            if sync.isHittable {
                tap(sync, name: "sync button", notes: &notes)
            } else {
                notes.append("sync button offscreen")
                app.typeKey("r", modifierFlags: .command)
            }
            pause(0.9, frames: &frames)
        } else {
            notes.append("sync button missing")
        }

        if app.buttons["sidebar-all"].isHittable {
            app.buttons["sidebar-all"].click()
        }
        app.typeKey("k", modifierFlags: .command)
        let palette = app.descendants(matching: .any)["command-palette"]
        let search = app.descendants(matching: .any)["palette-search"]
        if palette.waitForExistence(timeout: 4) {
            if search.waitForExistence(timeout: 2), search.isHittable {
                search.click()
            } else if palette.isHittable {
                palette.click()
            }
            app.typeText("sync")
            pause(0.7, frames: &frames)
            app.typeKey(.escape, modifierFlags: [])
            pause(0.4, frames: &frames)
        } else {
            notes.append("palette missing")
        }

        app.typeKey("l", modifierFlags: [.command, .shift])
        let forward = app.buttons["wizard-continue"]
        if forward.waitForExistence(timeout: 5) {
            pause(0.45, frames: &frames)
            tap(forward, name: "wizard continue", notes: &notes)
            pause(0.65, frames: &frames)
            tap(app.buttons["wizard-continue"], name: "wizard continue", notes: &notes)
            pause(0.65, frames: &frames)
            tap(app.buttons["wizard-back"], name: "wizard back", notes: &notes)
            pause(0.65, frames: &frames)
        } else {
            notes.append("wizard missing")
        }

        stop(capture)
        let captureLog = String(data: log.fileHandleForReading.readDataToEndOfFile(), encoding: .utf8) ?? ""
        var best = videoURL
        print("SCREENCAPTURE bytes \(fileSize(videoURL))")
        print(captureLog)

        if fileSize(best) < 80_000 {
            let fallback = URL(fileURLWithPath: NSTemporaryDirectory()).appendingPathComponent("skill-manager-interaction-light.mov")
            do {
                try MovieWriter.write(images: frames, to: fallback, fps: 8)
                if fileSize(fallback) > fileSize(best) {
                    best = fallback
                    try? FileManager.default.removeItem(at: videoURL)
                    try? FileManager.default.copyItem(at: fallback, to: videoURL)
                    if fileSize(videoURL) >= fileSize(fallback) {
                        best = videoURL
                    }
                }
            } catch {
                notes.append("frame writer \(error.localizedDescription)")
                print("FRAME_WRITER \(error)")
            }
        }

        publish(best)
        let size = max(fileSize(best), fileSize(videoURL))
        print("DEMO_VIDEO \(best.path) bytes \(size) frames \(frames.count)")
        if FileManager.default.fileExists(atPath: best.path) {
            let attachment = XCTAttachment(contentsOfFile: best)
            attachment.name = "interaction-light.mov"
            attachment.lifetime = .keepAlways
            add(attachment)
        }
        XCTAssertGreaterThan(size, 20_000, "recording was \(size) bytes. \(notes.joined(separator: ", ")). \(captureLog)")
        XCTAssertTrue(notes.isEmpty, notes.joined(separator: ", "))
    }

    private func tap(_ element: XCUIElement, name: String, notes: inout [String]) {
        guard element.exists else {
            notes.append("\(name) missing")
            return
        }
        guard element.isHittable else {
            notes.append("\(name) offscreen")
            return
        }
        element.hover()
        element.click()
    }

    private func pause(_ seconds: TimeInterval, frames: inout [NSImage]) {
        let end = Date().addingTimeInterval(seconds)
        while Date() < end {
            if frames.count < 72 {
                frames.append(XCUIScreen.main.screenshot().image)
            }
            let slice = min(0.12, end.timeIntervalSinceNow)
            if slice > 0 {
                Thread.sleep(forTimeInterval: slice)
            }
        }
    }

    private func stop(_ capture: Process) {
        guard capture.isRunning else { return }
        capture.interrupt()
        let deadline = Date().addingTimeInterval(6)
        while capture.isRunning, Date() < deadline {
            Thread.sleep(forTimeInterval: 0.1)
        }
        if capture.isRunning {
            capture.terminate()
            capture.waitUntilExit()
        }
    }

    private func fileSize(_ url: URL) -> Int {
        (try? FileManager.default.attributesOfItem(atPath: url.path)[.size] as? Int) ?? 0
    }

    private func publish(_ url: URL) {
        guard FileManager.default.fileExists(atPath: url.path) else { return }
        let name = "skill-manager-interaction-light.mov"
        let destinations = [
            URL(fileURLWithPath: "/tmp").appendingPathComponent(name),
            URL(fileURLWithPath: NSTemporaryDirectory()).appendingPathComponent(name),
            FileManager.default.homeDirectoryForCurrentUser.appendingPathComponent(name)
        ]
        for destination in destinations where destination.path != url.path {
            try? FileManager.default.removeItem(at: destination)
            try? FileManager.default.copyItem(at: url, to: destination)
            print("PUBLISHED \(destination.path) bytes \(fileSize(destination))")
        }
    }
}

enum MovieWriter {
    static func write(images: [NSImage], to url: URL, fps: Int32) throws {
        let bitmaps = images.compactMap { cgImage(from: $0) }
        guard let first = bitmaps.first else {
            throw CocoaError(.fileWriteUnknown)
        }
        let width = max(first.width - (first.width % 2), 2)
        let height = max(first.height - (first.height % 2), 2)
        try? FileManager.default.removeItem(at: url)
        let writer = try AVAssetWriter(outputURL: url, fileType: .mov)
        let settings: [String: Any] = [
            AVVideoCodecKey: AVVideoCodecType.h264,
            AVVideoWidthKey: width,
            AVVideoHeightKey: height
        ]
        let input = AVAssetWriterInput(mediaType: .video, outputSettings: settings)
        input.expectsMediaDataInRealTime = false
        let adaptor = AVAssetWriterInputPixelBufferAdaptor(
            assetWriterInput: input,
            sourcePixelBufferAttributes: [
                kCVPixelBufferPixelFormatTypeKey as String: Int(kCVPixelFormatType_32BGRA),
                kCVPixelBufferWidthKey as String: width,
                kCVPixelBufferHeightKey as String: height,
                kCVPixelBufferCGImageCompatibilityKey as String: true,
                kCVPixelBufferCGBitmapContextCompatibilityKey as String: true
            ]
        )
        writer.add(input)
        guard writer.startWriting() else {
            throw writer.error ?? CocoaError(.fileWriteUnknown)
        }
        writer.startSession(atSourceTime: .zero)
        for (index, image) in bitmaps.enumerated() {
            let readyDeadline = Date().addingTimeInterval(2)
            while !input.isReadyForMoreMediaData, Date() < readyDeadline {
                Thread.sleep(forTimeInterval: 0.01)
            }
            guard input.isReadyForMoreMediaData,
                  let buffer = pixelBuffer(from: image, width: width, height: height) else { continue }
            let time = CMTime(value: CMTimeValue(index), timescale: fps)
            if !adaptor.append(buffer, withPresentationTime: time) {
                break
            }
        }
        let end = CMTime(value: CMTimeValue(bitmaps.count), timescale: fps)
        writer.endSession(atSourceTime: end)
        input.markAsFinished()
        let group = DispatchGroup()
        group.enter()
        writer.finishWriting {
            group.leave()
        }
        _ = group.wait(timeout: .now() + 15)
        if writer.status != .completed {
            throw writer.error ?? CocoaError(.fileWriteUnknown)
        }
    }

    private static func cgImage(from image: NSImage) -> CGImage? {
        var rect = CGRect(origin: .zero, size: image.size)
        if let rendered = image.cgImage(forProposedRect: &rect, context: nil, hints: nil) {
            return rendered
        }
        guard let tiff = image.tiffRepresentation, let rep = NSBitmapImageRep(data: tiff) else { return nil }
        return rep.cgImage
    }

    private static func pixelBuffer(from image: CGImage, width: Int, height: Int) -> CVPixelBuffer? {
        var buffer: CVPixelBuffer?
        let status = CVPixelBufferCreate(
            kCFAllocatorDefault,
            width,
            height,
            kCVPixelFormatType_32BGRA,
            nil,
            &buffer
        )
        guard status == kCVReturnSuccess, let buffer else { return nil }
        CVPixelBufferLockBaseAddress(buffer, [])
        defer { CVPixelBufferUnlockBaseAddress(buffer, []) }
        guard let context = CGContext(
            data: CVPixelBufferGetBaseAddress(buffer),
            width: width,
            height: height,
            bitsPerComponent: 8,
            bytesPerRow: CVPixelBufferGetBytesPerRow(buffer),
            space: CGColorSpaceCreateDeviceRGB(),
            bitmapInfo: CGBitmapInfo.byteOrder32Little.rawValue | CGImageAlphaInfo.premultipliedFirst.rawValue
        ) else { return nil }
        context.draw(image, in: CGRect(x: 0, y: 0, width: width, height: height))
        return buffer
    }
}
