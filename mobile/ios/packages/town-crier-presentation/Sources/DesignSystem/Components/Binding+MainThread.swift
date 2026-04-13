import SwiftUI

extension Binding: @retroactive @unchecked Sendable {}

extension Binding where Value: Sendable {
  /// Returns a new binding that dispatches its setter onto the main thread.
  ///
  /// Use this when a framework (e.g. StoreKit's `manageSubscriptionsSheet`)
  /// calls the binding setter from a background thread while the backing
  /// store is `@MainActor`-isolated. `DispatchQueue.main.async` integrates
  /// directly with the run loop, avoiding the race window that
  /// `Task { @MainActor in }` introduces via Swift Concurrency's
  /// cooperative executor.
  public func dispatchingSetOnMain() -> Binding<Value> {
    Binding<Value>(
      get: { self.wrappedValue },
      set: { newValue in
        if Thread.isMainThread {
          self.wrappedValue = newValue
        } else {
          DispatchQueue.main.async {
            self.wrappedValue = newValue
          }
        }
      }
    )
  }
}
