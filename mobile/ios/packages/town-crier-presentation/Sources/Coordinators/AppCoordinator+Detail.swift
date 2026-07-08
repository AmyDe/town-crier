import Foundation
import TownCrierDomain

/// Application-detail presentation, split out of `AppCoordinator` to keep the
/// root coordinator under the file-length limit (mirrors the
/// `AppCoordinator+Onboarding` / `AppCoordinator+WatchZones` split).
extension AppCoordinator {
  /// Presents the detail sheet synchronously from a row payload — bypasses the
  /// per-id fetch so the sheet appears instantly. The detail view model still
  /// runs `refresh()` in `.task` to keep saved-row snapshots fresh on the
  /// server (bd tc-sslz, tc-udby). Also used by the map summary sheet's
  /// "View full details" button (tc-1q07), which already holds the full object.
  func showApplicationDetail(_ application: PlanningApplication) {
    isDetailApplicationAnonymous = false
    detailApplication = application
  }

  /// Presents the detail sheet in anonymous mode from an already-loaded
  /// application (GH#879 Phase 2) — the anonymous map/summary sheet's "View
  /// full details" button. No network call, mirrors
  /// ``showApplicationDetail(_:)-(PlanningApplication)`` but flags the
  /// resulting detail view model as anonymous so
  /// ``makeApplicationDetailViewModel(application:)`` hides Save, skips
  /// saved-state, and refreshes via the by-slug read.
  public func showAnonymousApplicationDetail(_ application: PlanningApplication) {
    isDetailApplicationAnonymous = true
    detailApplication = application
  }

  func showApplicationDetail(_ id: PlanningApplicationId) {
    // Cancel any in-flight detail load so rapid 4× taps from a digest
    // email card collapse to a single presentation. Without this, multiple
    // overlapping tasks could each mutate `detailApplication` after their
    // `await` resumed, causing the sheet to flicker or fail to present
    // (tc-dt3x).
    pendingDetailLoad?.cancel()
    pendingDetailLoad = Task { [weak self] in
      guard let self else { return }
      do {
        let application = try await repository.fetchApplication(by: id)
        // The cancellation above only cancels the prior `Task`, which has
        // no effect on `try await` calls that don't check cooperatively.
        // After the await resumes we must check `Task.isCancelled` so a
        // superseded fetch does not stomp the latest tap's mutation.
        guard !Task.isCancelled else { return }
        isDetailApplicationAnonymous = false
        detailApplication = application
      } catch let domainError as DomainError {
        guard !Task.isCancelled else { return }
        deepLinkError = domainError
      } catch {
        guard !Task.isCancelled else { return }
        deepLinkError = .unexpected(error.localizedDescription)
      }
    }
  }

  /// Resolves an inbound public share Universal Link `/a/{authoritySlug}/{ref...}`
  /// (GH #738 Slice 4) into the native detail screen. Routes to the anonymous
  /// by-slug repository when there is no session (GH#879 Phase 2) — the authed
  /// `URLSessionAPIClient` throws `sessionExpired` before any HTTP call with no
  /// session, which previously surfaced a misleading "Session Expired" alert to
  /// a signed-out user tapping a share link. A signed-in session always keeps
  /// today's authed path: read-state marking and the by-id refresh-on-tap route
  /// are authed-only server behaviours. Falls back to the authed repository if
  /// no anonymous repository was injected, preserving prior behaviour. Same
  /// cancellation-guard pattern as ``showApplicationDetail(_:)-(PlanningApplicationId)``
  /// so overlapping opens collapse to a single presentation.
  func showApplicationDetail(bySlug authoritySlug: String, ref: String) {
    pendingDetailLoad?.cancel()
    pendingDetailLoad = Task { [weak self] in
      guard let self else { return }
      let isAnonymous = await authService.currentSession() == nil
      do {
        let application: PlanningApplication
        if isAnonymous, let anonymousApplicationDetailRepository {
          application = try await anonymousApplicationDetailRepository.fetchApplication(
            bySlug: authoritySlug, ref: ref)
        } else {
          application = try await repository.fetchApplication(bySlug: authoritySlug, ref: ref)
        }
        guard !Task.isCancelled else { return }
        isDetailApplicationAnonymous = isAnonymous && anonymousApplicationDetailRepository != nil
        detailApplication = application
      } catch let domainError as DomainError {
        guard !Task.isCancelled else { return }
        deepLinkError = domainError
      } catch {
        guard !Task.isCancelled else { return }
        deepLinkError = .unexpected(error.localizedDescription)
      }
    }
  }

  /// Test-only synchronisation: await the most recent
  /// `showApplicationDetail` fetch. Replaces flaky `Task.sleep` waits.
  public func waitForPendingDetailLoad() async {
    await pendingDetailLoad?.value
  }

  /// Factory for the detail sheet's view model (`TownCrierApp.swift`'s
  /// `.sheet(item: $coordinator.detailApplication)`). Configuration depends on
  /// ``isDetailApplicationAnonymous`` (GH#879 Phase 2): the anonymous path gets
  /// no Save/saved-state and refreshes via the by-slug read; the authed path is
  /// unchanged. Falls back to the authed configuration if no anonymous
  /// repository was injected.
  public func makeApplicationDetailViewModel(
    application: PlanningApplication
  ) -> ApplicationDetailViewModel {
    let viewModel: ApplicationDetailViewModel
    if isDetailApplicationAnonymous, let anonymousApplicationDetailRepository {
      viewModel = ApplicationDetailViewModel(
        application: application,
        anonymousApplicationDetailRepository: anonymousApplicationDetailRepository
      )
    } else {
      viewModel = ApplicationDetailViewModel(
        application: application,
        savedApplicationRepository: savedApplicationRepository,
        planningApplicationRepository: repository
      )
    }
    viewModel.onDismiss = { [weak self] in
      self?.detailApplication = nil
    }
    // Review-prompt value moments (GH #628): a portal tap-through and a save are
    // both genuine engagement peaks. The save callback fires only on a
    // successful false→true save (the view model guarantees this).
    viewModel.onOpenPortal = { [weak self] _ in
      self?.reviewPromptTracker?.record(.tappedPortal)
    }
    viewModel.onSaved = { [weak self] in
      self?.reviewPromptTracker?.record(.savedApplication)
    }
    viewModel.onRequestSignUp = { [weak self] in
      self?.onRequestSignUp?()
    }
    return viewModel
  }
}
