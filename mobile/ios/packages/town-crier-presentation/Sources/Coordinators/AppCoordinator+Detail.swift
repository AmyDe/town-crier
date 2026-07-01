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
  /// (GH #738 Slice 4) into the native detail screen. The anonymous by-slug read
  /// returns the full application, so this presents it directly — no round-trip
  /// through the by-id fetch. Same cancellation-guard pattern as
  /// ``showApplicationDetail(_:)-(PlanningApplicationId)`` so overlapping opens
  /// collapse to a single presentation.
  func showApplicationDetail(bySlug authoritySlug: String, ref: String) {
    pendingDetailLoad?.cancel()
    pendingDetailLoad = Task { [weak self] in
      guard let self else { return }
      do {
        let application = try await repository.fetchApplication(bySlug: authoritySlug, ref: ref)
        guard !Task.isCancelled else { return }
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
}
