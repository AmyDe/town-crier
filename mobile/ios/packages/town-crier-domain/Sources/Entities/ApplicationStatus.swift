/// The lifecycle status of a planning application.
public enum ApplicationStatus: String, Equatable, Hashable, Sendable {
  case undecided
  case notAvailable
  case approved
  case refused
  case withdrawn
  case appealed
  case unknown
}
