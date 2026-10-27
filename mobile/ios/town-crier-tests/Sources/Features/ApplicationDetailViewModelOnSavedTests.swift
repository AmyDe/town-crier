import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("ApplicationDetailViewModel — onSaved")
@MainActor
struct ApplicationDetailViewModelOnSavedTests {
  private func makeSUT(
    isSaved: Bool,
    saveResult: Result<Void, Error> = .success(())
  ) -> (ApplicationDetailViewModel, SpySavedApplicationRepository) {
    let repository = SpySavedApplicationRepository()
    repository.saveResult = saveResult
    let viewModel = ApplicationDetailViewModel(
      application: .withPortalUrl,
      savedApplicationRepository: repository,
      isSaved: isSaved
    )
    return (viewModel, repository)
  }

  @Test("a successful first-time save fires onSaved")
  func successfulSaveFiresCallback() async {
    let (sut, _) = makeSUT(isSaved: false)
    var savedCallCount = 0
    sut.onSaved = { savedCallCount += 1 }

    await sut.toggleSave()

    #expect(savedCallCount == 1)
  }

  @Test("unsaving does not fire onSaved")
  func unsaveDoesNotFireCallback() async {
    let (sut, _) = makeSUT(isSaved: true)
    var savedCallCount = 0
    sut.onSaved = { savedCallCount += 1 }

    await sut.toggleSave()

    #expect(savedCallCount == 0)
  }

  @Test("a failed save does not fire onSaved")
  func failedSaveDoesNotFireCallback() async {
    let (sut, _) = makeSUT(isSaved: false, saveResult: .failure(DomainError.networkUnavailable))
    var savedCallCount = 0
    sut.onSaved = { savedCallCount += 1 }

    await sut.toggleSave()

    #expect(savedCallCount == 0)
  }
}
