/// Port for anonymous (no-account, no-session) planning application detail
/// reads (GH#879 Phase 2): the public by-slug share endpoint, with no
/// `AuthenticationService` requirement. A distinct protocol from
/// ``PlanningApplicationRepository`` — rather than making that repository's
/// `fetchApplication(bySlug:ref:)` do double duty — so the anonymous detail
/// path can never accidentally depend on (or be satisfied by a fake for) the
/// full authenticated repository surface.
public protocol AnonymousApplicationDetailRepository: Sendable {
  /// Fetches a single application by its public share identity — the
  /// API-emitted authority slug plus the full area-prefixed PlanIt ref
  /// (slashes preserved). Backs both an inbound share Universal Link
  /// resolved with no session, and the anonymous detail screen's
  /// stale-while-revalidate ``refresh()``.
  func fetchApplication(bySlug authoritySlug: String, ref: String) async throws
    -> PlanningApplication
}
