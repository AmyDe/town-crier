/// Errors originating from the domain layer.
public enum DomainError: Error, Equatable, Sendable {
    case invalidPostcode(String)
    case invalidStatusTransition(from: ApplicationStatus, to: ApplicationStatus)
    case applicationNotFound(PlanningApplicationId)
    case invalidCoordinate
    case invalidWatchZoneRadius
    case networkUnavailable
    case unexpected(String)
}
