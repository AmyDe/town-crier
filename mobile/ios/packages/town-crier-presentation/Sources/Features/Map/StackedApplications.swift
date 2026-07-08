import TownCrierDomain

/// The applications stacked at one unsplittable map cell, wrapped as an
/// `Identifiable` value so it can drive a `.sheet(item:)` for the
/// disambiguation list (GH#722). `id` is the source cluster's id, so
/// re-tapping the same stacked cell re-presents the same list rather than a
/// spurious second sheet.
///
/// Shared between the authenticated map (``MapViewModel``) and the anonymous
/// map (``AnonymousMapViewModel``, GH#877) — both present the same
/// ``StackedApplicationsSheet``, so this wrapper lives in its own file rather
/// than inside either view model.
struct StackedApplications: Identifiable, Equatable, Sendable {
  let id: String
  let applications: [PlanningApplication]
}
