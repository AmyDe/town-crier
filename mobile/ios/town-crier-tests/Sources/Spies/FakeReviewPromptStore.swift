import Foundation
import TownCrierDomain

/// In-memory ``ReviewPromptStore`` for tests — holds the state in a field and
/// counts saves.
final class FakeReviewPromptStore: ReviewPromptStore, @unchecked Sendable {
  var state: ReviewPromptState
  private(set) var saveCallCount = 0

  init(state: ReviewPromptState = ReviewPromptState()) {
    self.state = state
  }

  func load() -> ReviewPromptState {
    state
  }

  func save(_ state: ReviewPromptState) {
    self.state = state
    saveCallCount += 1
  }
}
