import TownCrierDomain

/// Navigation targets reachable via deep links or notification taps.
public enum DeepLink: Equatable, Sendable {
  case applicationDetail(PlanningApplicationId)
  /// The Applications tab root — used when a Universal Link points at
  /// `/applications` exactly (e.g. the digest email's bottom CTA).
  case applicationsList
  /// A public share link `/a/{authoritySlug}/{ref...}` (GH #738 Slice 4). The
  /// `authoritySlug` is the API-emitted slug and `ref` is the application's full
  /// area-prefixed PlanIt name, verbatim (slashes preserved). Resolved into the
  /// native detail screen via the anonymous by-slug read.
  case shareApplication(authoritySlug: String, ref: String)
}
