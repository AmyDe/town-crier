/// Persists the device-local anonymous browse session (GH#868 Phase 3): the
/// postcode/coordinate a pre-signup user entered, so a relaunch with no
/// authenticated session returns straight to the anonymous map, and so a
/// successful sign-up can hand the location to the onboarding wizard.
public protocol AnonymousBrowseStateRepository: Sendable {
  /// The persisted state, or `nil` if the user has never completed anonymous
  /// postcode entry, or it has since been cleared (sign-out or post-signup
  /// handoff).
  func load() -> AnonymousBrowseState?

  /// Persists `state`, replacing any previously saved state.
  func save(_ state: AnonymousBrowseState)

  /// Clears any persisted state. Called on sign-out (a deliberate return to
  /// the welcome screen) and after a successful post-signup handoff into
  /// onboarding.
  func clear()
}
