import TownCrierDomain

/// The status-derived color category for a map pin.
public enum StatusColor: Sendable {
    case pending
    case approved
    case refused
    case withdrawn
    case appealed
    case unknown
}

/// A lightweight value representing a single pin on the map.
public struct MapAnnotationItem: Identifiable, Sendable {
    public let id: String
    public let applicationId: PlanningApplicationId
    public let latitude: Double
    public let longitude: Double
    public let statusColor: StatusColor
    public let title: String
    public let address: String

    public init(application: PlanningApplication, coordinate: Coordinate) {
        self.id = application.id.value
        self.applicationId = application.id
        self.latitude = coordinate.latitude
        self.longitude = coordinate.longitude
        self.statusColor = Self.color(for: application.status)
        self.title = application.description
        self.address = application.address
    }

    private static func color(for status: ApplicationStatus) -> StatusColor {
        switch status {
        case .underReview: return .pending
        case .approved: return .approved
        case .refused: return .refused
        case .withdrawn: return .withdrawn
        case .appealed: return .appealed
        case .unknown: return .unknown
        }
    }
}
