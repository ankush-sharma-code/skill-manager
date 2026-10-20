import Foundation
import Observation

@Observable
@MainActor
final class DemoModel {
    var skills: [SkillFixture]
    var selection: String
    var query = ""
    var section: SidebarSection = .all
    var toolSelection = "claude"
    var palettePresented = false
    var paletteQuery = ""
    var syncPhase: SyncPhase = .idle
    var displayedSynced: Int
    var wizardPresented = false
    var reportsLoading: Bool

    enum SyncPhase: Equatable {
        case idle
        case complete
    }

    init(
        skills: [SkillFixture] = FixtureLibrary.skills,
        selection: String? = nil,
        reportsLoading: Bool = false,
        section: SidebarSection = .all
    ) {
        self.skills = skills
        if let selection {
            self.selection = selection
        } else {
            self.selection = skills.first { $0.id == "poteto-mode" }?.id ?? skills.first?.id ?? ""
        }
        displayedSynced = FixtureLibrary.syncedSlots(in: skills)
        self.reportsLoading = reportsLoading
        self.section = section
    }

    var visibleSkills: [SkillFixture] {
        let trimmed = query.trimmingCharacters(in: .whitespacesAndNewlines)
        let pool: [SkillFixture]
        switch section {
        case .conflicts:
            pool = skills.filter { $0.flags.contains(.conflict) }
        case .idle:
            pool = skills.filter { $0.flags.contains(.idle) }
        case .all, .tools, .usage:
            pool = skills
        }
        return pool
            .filter { skill in
                trimmed.isEmpty
                    || skill.name.localizedCaseInsensitiveContains(trimmed)
                    || skill.summary.localizedCaseInsensitiveContains(trimmed)
            }
            .sorted { lhs, rhs in
                if section == .usage {
                    return lhs.usesThisWeek > rhs.usesThisWeek
                }
                return lhs.name < rhs.name
            }
    }

    var selectedSkill: SkillFixture? {
        skills.first { $0.id == selection }
    }

    var conflictCount: Int {
        skills.filter { $0.flags.contains(.conflict) }.count
    }

    var driftCount: Int {
        skills.flatMap(\.targets).filter { $0.state == .drift || $0.state == .missing }.count
    }

    var slotCount: Int {
        skills.flatMap(\.targets).count
    }

    var topSkills: [SkillFixture] {
        Array(skills.sorted { $0.usesThisWeek > $1.usesThisWeek }.prefix(5))
    }

    var statusKind: StatusKind {
        if syncPhase == .complete { return .ok }
        if reportsLoading || slotCount == 0 { return .attention }
        if conflictCount > 0 { return .conflict }
        if driftCount > 0 { return .attention }
        return .ok
    }

    var statusTitle: String {
        if syncPhase == .complete { return "Sync complete" }
        if reportsLoading { return "Loading" }
        if slotCount == 0 { return "Waiting for skills" }
        if conflictCount > 0 { return "\(conflictCount) conflicts" }
        if driftCount > 0 { return "Drift" }
        return "In sync"
    }

    var showsCheck: Bool {
        syncPhase == .complete
    }

    func select(_ id: String) {
        selection = id
        if section == .conflicts || section == .idle || section == .usage {
            return
        }
        section = .all
    }

    func openConflict(_ id: String) {
        selection = id
        section = .conflicts
    }

    func selectAdjacent(_ offset: Int) {
        let ids = visibleSkills.map(\.id)
        guard let index = ids.firstIndex(of: selection) else {
            selection = ids.first ?? ""
            return
        }
        let next = min(max(index + offset, 0), ids.count - 1)
        selection = ids[next]
    }

    func syncNow() {
        if syncPhase == .complete {
            syncPhase = .idle
            displayedSynced = FixtureLibrary.syncedSlots(in: skills)
            Task { @MainActor in
                try? await Task.sleep(nanoseconds: 40_000_000)
                displayedSynced = slotCount
                syncPhase = .complete
            }
            return
        }
        displayedSynced = slotCount
        syncPhase = .complete
    }

    func togglePalette() {
        palettePresented.toggle()
        if palettePresented {
            paletteQuery = ""
        }
    }

    func perform(_ entry: PaletteEntry) {
        palettePresented = false
        switch entry.id {
        case "action-sync":
            syncNow()
        case "action-add":
            section = .all
        case "action-setup":
            wizardPresented = true
        case "action-settings":
            break
        default:
            if entry.kind == .skill {
                select(entry.id)
            } else if entry.kind == .target {
                section = .tools
            }
        }
    }
}
