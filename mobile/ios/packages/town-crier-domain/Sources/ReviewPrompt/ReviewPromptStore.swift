/// Persists the local ``ReviewPromptState`` for the App Store review-prompt
/// feature (GH #628).
///
/// The contract is a whole-state load/save: the store reconstructs the full
/// value on ``load()`` and writes it back on ``save(_:)``. All storage is
/// device-local — there is no server, network, or PII involved. Mirrors the
/// ``OnboardingRepository`` persistence shape.
public protocol ReviewPromptStore: Sendable {
  /// Returns the persisted state, or a default-initialised state on first run.
  func load() -> ReviewPromptState
  /// Persists the supplied state.
  func save(_ state: ReviewPromptState)
}
