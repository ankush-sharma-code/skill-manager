import SwiftUI

enum MockScreen: String, CaseIterable, Identifiable {
    case mainWindow
    case wizardWelcome
    case wizardDetect
    case wizardImport
    case wizardSyncMode
    case skillReader
    case menuBarPopover
    case emptyState
    case loadingState
    case errorState
    case commandPalette

    var id: String { rawValue }

    var fileName: String {
        switch self {
        case .mainWindow: "main-window"
        case .wizardWelcome: "wizard-welcome"
        case .wizardDetect: "wizard-detect"
        case .wizardImport: "wizard-import"
        case .wizardSyncMode: "wizard-sync-mode"
        case .skillReader: "skill-reader"
        case .menuBarPopover: "menu-bar-popover"
        case .emptyState: "empty-state"
        case .loadingState: "loading-state"
        case .errorState: "error-state"
        case .commandPalette: "command-palette"
        }
    }

    var size: CGSize {
        switch self {
        case .wizardWelcome, .wizardDetect, .wizardImport, .wizardSyncMode:
            CGSize(width: Layout.wizardWidth, height: Layout.wizardHeight)
        case .skillReader:
            CGSize(width: Layout.readerWidth, height: Layout.readerHeight)
        case .menuBarPopover:
            CGSize(width: Layout.popoverWidth, height: Layout.popoverHeight)
        default:
            CGSize(width: Layout.mainWidth, height: Layout.mainHeight)
        }
    }

    @MainActor
    @ViewBuilder
    func view(playsIntro: Bool) -> some View {
        switch self {
        case .mainWindow:
            MainWindowMock(model: DemoModel(), playsIntro: playsIntro, animatesSelection: false)
        case .wizardWelcome:
            WizardFrame(step: .welcome)
        case .wizardDetect:
            WizardFrame(step: .detect)
        case .wizardImport:
            WizardFrame(step: .importExisting)
        case .wizardSyncMode:
            WizardFrame(step: .syncMode)
        case .skillReader:
            SkillReaderScreen(playsIntro: playsIntro)
        case .menuBarPopover:
            MenuBarPopover(model: DemoModel())
        case .emptyState:
            EmptySkillsMock()
        case .loadingState:
            LoadingSkillsMock()
        case .errorState:
            ErrorSkillsMock()
        case .commandPalette:
            CommandPaletteMock()
        }
    }
}
