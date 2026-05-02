import Foundation
import TownCrierDomain

/// Test double for `SavedApplicationRepository` that suspends `loadAll()` until
/// `resume(with:)` is called. Used to assert in-flight loading state on
/// `ApplicationListViewModel` and `MapViewModel`.
final class ControllableSavedApplicationRepository: SavedApplicationRepository, @unchecked Sendable {
  private var continuation: CheckedContinuation<[SavedApplication], Error>?
  private var pendingCallSignal: CheckedContinuation<Void, Never>?
  private var didReceiveCall = false
  private let lock = NSLock()

  func save(application: PlanningApplication) async throws {}

  func remove(applicationUid: String) async throws {}

  func loadAll() async throws -> [SavedApplication] {
    try await withCheckedThrowingContinuation { cont in
      lock.lock()
      continuation = cont
      didReceiveCall = true
      let signal = pendingCallSignal
      pendingCallSignal = nil
      lock.unlock()
      signal?.resume()
    }
  }

  /// Suspends until `loadAll()` has been entered. If it has already been entered,
  /// returns immediately.
  func waitForCall() async {
    await withCheckedContinuation { (cont: CheckedContinuation<Void, Never>) in
      lock.lock()
      if didReceiveCall {
        lock.unlock()
        cont.resume()
        return
      }
      pendingCallSignal = cont
      lock.unlock()
    }
  }

  /// Completes the in-flight `loadAll()` call with the given result.
  func resume(with result: Result<[SavedApplication], Error>) {
    lock.lock()
    let cont = continuation
    continuation = nil
    lock.unlock()
    switch result {
    case .success(let value):
      cont?.resume(returning: value)
    case .failure(let error):
      cont?.resume(throwing: error)
    }
  }
}
