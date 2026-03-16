import Testing
@testable import TownCrierPresentation

@Suite("HomeViewModel")
@MainActor
struct HomeViewModelTests {
    @Test func init_setsTitle() {
        let sut = HomeViewModel()
        #expect(sut.title == "Town Crier")
    }

    @Test func init_setsSubtitle() {
        let sut = HomeViewModel()
        #expect(sut.subtitle == "Planning applications near you")
    }
}
