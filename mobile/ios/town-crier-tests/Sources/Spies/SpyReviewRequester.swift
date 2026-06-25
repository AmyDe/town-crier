import TownCrierPresentation

/// Records whether a review request was made, in place of the real
/// `requestReview` OS call.
@MainActor
final class SpyReviewRequester: ReviewRequesting {
  private(set) var requestReviewCallCount = 0

  func requestReview() {
    requestReviewCallCount += 1
  }
}
