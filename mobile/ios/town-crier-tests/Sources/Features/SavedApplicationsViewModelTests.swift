import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("SavedApplicationsViewModel")
@MainActor
struct SavedApplicationsViewModelTests {

  // MARK: - Helpers

  private func makeSUT(
    savedApplications: [SavedApplication] = []
  ) -> (SavedApplicationsViewModel, SpySavedApplicationRepository) {
    let spy = SpySavedApplicationRepository()
    spy.loadAllResult = .success(savedApplications)
    let vm = SavedApplicationsViewModel(repository: spy)
    return (vm, spy)
  }

  // MARK: - Initial Load

  @Test("loadSavedApplications populates applications on success")
  func loadSavedApplications_populatesOnSuccess() async {
    let expected = [SavedApplication.rearExtension, .changeOfUse]
    let (sut, _) = makeSUT(savedApplications: expected)

    await sut.loadSavedApplications()

    #expect(sut.savedApplications == expected)
    #expect(!sut.isLoading)
    #expect(sut.error == nil)
  }

  @Test("loadSavedApplications calls repository loadAll")
  func loadSavedApplications_callsRepository() async {
    let (sut, spy) = makeSUT()

    await sut.loadSavedApplications()

    #expect(spy.loadAllCallCount == 1)
  }

  @Test("loadSavedApplications sets isLoading false after completion")
  func loadSavedApplications_setsIsLoadingFalse() async {
    let (sut, _) = makeSUT()

    await sut.loadSavedApplications()

    #expect(!sut.isLoading)
  }

  @Test("loadSavedApplications sets error on failure")
  func loadSavedApplications_setsErrorOnFailure() async {
    let (sut, spy) = makeSUT()
    spy.loadAllResult = .failure(DomainError.networkUnavailable)

    await sut.loadSavedApplications()

    #expect(sut.error == .networkUnavailable)
    #expect(sut.savedApplications.isEmpty)
  }

  @Test("loadSavedApplications clears error on retry")
  func loadSavedApplications_clearsErrorOnRetry() async {
    let (sut, spy) = makeSUT()
    spy.loadAllResult = .failure(DomainError.networkUnavailable)
    await sut.loadSavedApplications()

    spy.loadAllResult = .success([.rearExtension])
    await sut.loadSavedApplications()

    #expect(sut.error == nil)
    #expect(sut.savedApplications.count == 1)
  }

  @Test("loadSavedApplications resets on subsequent call")
  func loadSavedApplications_resetsOnReload() async {
    let (sut, spy) = makeSUT(savedApplications: [.rearExtension, .changeOfUse])
    await sut.loadSavedApplications()

    spy.loadAllResult = .success([.rearExtension])
    await sut.loadSavedApplications()

    #expect(sut.savedApplications.count == 1)
  }

  // MARK: - Empty State

  @Test("isEmpty is true when load returns no results")
  func isEmpty_noResults_returnsTrue() async {
    let (sut, _) = makeSUT()

    await sut.loadSavedApplications()

    #expect(sut.isEmpty)
  }

  @Test("isEmpty is false before loading")
  func isEmpty_beforeLoad_returnsFalse() {
    let (sut, _) = makeSUT()

    #expect(!sut.isEmpty)
  }

  @Test("isEmpty is false when applications exist")
  func isEmpty_withApplications_returnsFalse() async {
    let (sut, _) = makeSUT(savedApplications: [.rearExtension])

    await sut.loadSavedApplications()

    #expect(!sut.isEmpty)
  }

  @Test("isEmpty is false when error exists")
  func isEmpty_withError_returnsFalse() async {
    let (sut, spy) = makeSUT()
    spy.loadAllResult = .failure(DomainError.networkUnavailable)

    await sut.loadSavedApplications()

    #expect(!sut.isEmpty)
  }

  // MARK: - Unsave

  @Test("unsave calls repository remove with correct UID")
  func unsave_callsRepositoryRemove() async {
    let (sut, spy) = makeSUT(savedApplications: [.rearExtension])
    await sut.loadSavedApplications()

    await sut.unsave(applicationUid: "BK/2026/0042")

    #expect(spy.removeCalls == ["BK/2026/0042"])
  }

  @Test("unsave removes item from local list on success")
  func unsave_removesFromList() async {
    let (sut, _) = makeSUT(savedApplications: [.rearExtension, .changeOfUse])
    await sut.loadSavedApplications()

    await sut.unsave(applicationUid: "BK/2026/0042")

    #expect(sut.savedApplications.count == 1)
    #expect(sut.savedApplications[0].applicationUid == "BK/2026/0099")
  }

  @Test("unsave sets error on failure but preserves list")
  func unsave_setsErrorOnFailure() async {
    let (sut, spy) = makeSUT(savedApplications: [.rearExtension])
    await sut.loadSavedApplications()
    spy.removeResult = .failure(DomainError.networkUnavailable)

    await sut.unsave(applicationUid: "BK/2026/0042")

    #expect(sut.error == .networkUnavailable)
    #expect(sut.savedApplications.count == 1)
  }

  // MARK: - Callback

  @Test("selectApplication invokes onApplicationSelected callback")
  func selectApplication_invokesCallback() {
    let (sut, _) = makeSUT()
    var selectedUid: String?
    sut.onApplicationSelected = { uid in selectedUid = uid }

    sut.selectApplication(uid: "BK/2026/0042")

    #expect(selectedUid == "BK/2026/0042")
  }
}
