import Foundation

/// Tracks one view model's load lifecycle. Only the latest `begin()` owns the
/// state: a superseded load's `end(_:)` is ignored, and a delay task that is
/// cancelled or superseded never reports slow.
@MainActor
final class SlowLoadTracker {
  private var generation = 0
  private var delayTask: Task<Void, Never>?
  private let delay: Duration
  private let onChange: @MainActor (_ isLoading: Bool, _ isSlow: Bool) -> Void

  init(delay: Duration, onChange: @escaping @MainActor (_ isLoading: Bool, _ isSlow: Bool) -> Void) {
    self.delay = delay
    self.onChange = onChange
  }

  func begin() -> Int {
    generation += 1
    let token = generation
    onChange(true, false)
    delayTask?.cancel()
    delayTask = Task { [delay] in
      try? await Task.sleep(for: delay)
      guard !Task.isCancelled, token == generation else { return }
      onChange(true, true)
    }
    return token
  }

  func end(_ token: Int) {
    guard token == generation else { return }
    delayTask?.cancel()
    delayTask = nil
    onChange(false, false)
  }
}
