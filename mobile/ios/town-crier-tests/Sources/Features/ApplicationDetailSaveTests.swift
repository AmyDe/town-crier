import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("ApplicationDetailViewModel — save/unsave")
@MainActor
struct ApplicationDetailSaveTests {

  // MARK: - Helpers

  private func makeSUT(
    application: PlanningApplication = .pendingReview,
    isSaved: Bool = false
  ) -> (ApplicationDetailViewModel, SpySavedApplicationRepository) {
    let spy = SpySavedApplicationRepository()
    let vm = ApplicationDetailViewModel(
      application: application,
      savedApplicationRepository: spy,
      isSaved: isSaved
    )
    return (vm, spy)
  }

  // MARK: - Initial State

  @Test("isSaved is false by default")
  func isSaved_defaultsFalse() {
    let (sut, _) = makeSUT()

    #expect(!sut.isSaved)
  }

  @Test("isSaved matches initial value when provided as true")
  func isSaved_matchesInitialTrue() {
    let (sut, _) = makeSUT(isSaved: true)

    #expect(sut.isSaved)
  }

  // MARK: - Toggle Save (from unsaved)

  @Test("toggleSave calls repository save when not saved")
  func toggleSave_whenUnsaved_callsSave() async {
    let (sut, spy) = makeSUT()

    await sut.toggleSave()

    #expect(spy.saveCalls == [PlanningApplication.pendingReview.id.value])
  }

  @Test("toggleSave sets isSaved to true on success")
  func toggleSave_whenUnsaved_setsSavedTrue() async {
    let (sut, _) = makeSUT()

    await sut.toggleSave()

    #expect(sut.isSaved)
  }

  @Test("toggleSave preserves isSaved false on save failure")
  func toggleSave_whenUnsaved_preservesOnFailure() async {
    let (sut, spy) = makeSUT()
    spy.saveResult = .failure(DomainError.networkUnavailable)

    await sut.toggleSave()

    #expect(!sut.isSaved)
  }

  // MARK: - Toggle Save (from saved)

  @Test("toggleSave calls repository remove when already saved")
  func toggleSave_whenSaved_callsRemove() async {
    let (sut, spy) = makeSUT(isSaved: true)

    await sut.toggleSave()

    #expect(spy.removeCalls == [PlanningApplication.pendingReview.id.value])
  }

  @Test("toggleSave sets isSaved to false on unsave success")
  func toggleSave_whenSaved_setsSavedFalse() async {
    let (sut, _) = makeSUT(isSaved: true)

    await sut.toggleSave()

    #expect(!sut.isSaved)
  }

  @Test("toggleSave preserves isSaved true on unsave failure")
  func toggleSave_whenSaved_preservesOnFailure() async {
    let (sut, spy) = makeSUT(isSaved: true)
    spy.removeResult = .failure(DomainError.networkUnavailable)

    await sut.toggleSave()

    #expect(sut.isSaved)
  }

  // MARK: - No repository

  @Test("toggleSave is no-op when repository is nil")
  func toggleSave_withoutRepository_isNoOp() async {
    let sut = ApplicationDetailViewModel(application: .pendingReview)

    await sut.toggleSave()

    #expect(!sut.isSaved)
  }

  // MARK: - Save button visibility

  @Test("canSave is true when repository is present")
  func canSave_withRepository_isTrue() {
    let (sut, _) = makeSUT()

    #expect(sut.canSave)
  }

  @Test("canSave is false when repository is nil")
  func canSave_withoutRepository_isFalse() {
    let sut = ApplicationDetailViewModel(application: .pendingReview)

    #expect(!sut.canSave)
  }
}
