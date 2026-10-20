import AppKit
import SwiftUI

@main
struct SkillManagerApp: App {
    @State private var model = DemoModel()

    var body: some Scene {
        WindowGroup("Skill Manager") {
            DemoRoot(model: model)
                .preferredColorScheme(DemoAppearance.colorScheme)
        }
        .defaultSize(width: Measure.mainWidth, height: Measure.mainHeight)
        .commands {
            CommandGroup(replacing: .newItem) {
                Button("Add Skill") {
                    model.section = .skills
                }
                .keyboardShortcut("n", modifiers: .command)
            }
            CommandMenu("Go") {
                Button("Skills") { model.section = .skills }
                    .keyboardShortcut("1", modifiers: .command)
                Button("Conflicts") { model.section = .conflicts }
                    .keyboardShortcut("2", modifiers: .command)
                Button("Targets") { model.section = .targets }
                    .keyboardShortcut("3", modifiers: .command)
                Button("Usage") { model.section = .usage }
                    .keyboardShortcut("4", modifiers: .command)
                Button("Settings") { model.section = .settings }
                    .keyboardShortcut("5", modifiers: .command)
            }
            CommandGroup(after: .sidebar) {
                Button("Sync Now") {
                    model.syncNow()
                }
                .keyboardShortcut("r", modifiers: .command)
                Button("Quick Search") {
                    model.togglePalette()
                }
                .keyboardShortcut("k", modifiers: .command)
                Button("Settings") {
                    model.section = .settings
                }
                .keyboardShortcut(",", modifiers: .command)
                Button("Run Setup Again") {
                    model.wizardPresented = true
                }
                .keyboardShortcut("l", modifiers: [.command, .shift])
            }
        }

        MenuBarExtra("Skill Manager", systemImage: "square.stack.3d.up.fill") {
            MenuBarPopover(model: model)
        }
        .menuBarExtraStyle(.window)
    }
}

struct DemoRoot: View {
    @Bindable var model: DemoModel

    var body: some View {
        MainWindowMock(model: model, playsIntro: true, animatesSelection: true)
            .frame(minWidth: Measure.mainWidth, minHeight: Measure.mainHeight)
            .sheet(isPresented: $model.wizardPresented) {
                InteractiveWizard()
                    .preferredColorScheme(DemoAppearance.colorScheme)
            }
            .onAppear(perform: placeWindow)
    }

    private func placeWindow() {
        DispatchQueue.main.async {
            DemoAppearance.applyToApplication()
            NSApp.setActivationPolicy(.regular)
            NSApp.activate(ignoringOtherApps: true)
            guard let window = NSApp.windows.first(where: \.canBecomeMain) else { return }
            window.title = "Skill Manager"
            window.setContentSize(NSSize(width: Measure.mainWidth, height: Measure.mainHeight))
            window.center()
        }
    }
}

enum DemoAppearance {
    static var colorScheme: ColorScheme? {
        switch ProcessInfo.processInfo.environment["SKILL_MANAGER_APPEARANCE"] {
        case "light":
            .light
        case "dark":
            .dark
        default:
            nil
        }
    }

    static func applyToApplication() {
        switch colorScheme {
        case .light:
            NSApp.appearance = NSAppearance(named: .aqua)
        case .dark:
            NSApp.appearance = NSAppearance(named: .darkAqua)
        case nil:
            break
        }
    }
}
