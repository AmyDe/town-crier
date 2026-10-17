/// The server-side filter applied to the paged watch-zone applications
/// endpoint. Status and unread are mutually exclusive (the server 400s when
/// both are sent), so they are modelled as a single sum type.
public enum ApplicationFilter: Sendable, Equatable {
  /// No filter — the "All" chip with the Unread toggle off. Sends neither
  /// `?status=` nor `?unread=`.
  case all
  /// Restrict to a single PlanIt `app_state`. Sends `?status=<rawValue>`.
  case status(ApplicationStatus)
  /// Restrict to applications with an unread notification for the caller.
  /// Sends `?unread=true`.
  case unread
}
