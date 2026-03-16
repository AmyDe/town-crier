import Combine
import Foundation
import TownCrierDomain

/// ViewModel exposing display-ready properties for a planning application detail screen.
@MainActor
public final class ApplicationDetailViewModel: ObservableObject {
    public let description: String
    public let address: String
    public let reference: String
    public let authorityName: String
    public let receivedDateFormatted: String
    public let statusLabel: String
    public let statusIcon: String
    public let status: ApplicationStatus
    public let portalUrl: URL?

    public var onOpenPortal: ((URL) -> Void)?
    public var onDismiss: (() -> Void)?

    public var hasPortalUrl: Bool {
        portalUrl != nil
    }

    private static let dateFormatter: DateFormatter = {
        let formatter = DateFormatter()
        formatter.dateFormat = "d MMM yyyy"
        formatter.locale = Locale(identifier: "en_GB")
        formatter.timeZone = TimeZone(identifier: "UTC")
        return formatter
    }()

    public init(application: PlanningApplication) {
        description = application.description
        address = application.address
        reference = application.reference.value
        authorityName = application.authority.name
        receivedDateFormatted = Self.dateFormatter.string(from: application.receivedDate)
        status = application.status
        portalUrl = application.portalUrl

        switch application.status {
        case .underReview:
            statusLabel = "Pending"
            statusIcon = "clock"
        case .approved:
            statusLabel = "Approved"
            statusIcon = "checkmark.circle"
        case .refused:
            statusLabel = "Refused"
            statusIcon = "xmark.circle"
        case .withdrawn:
            statusLabel = "Withdrawn"
            statusIcon = "arrow.uturn.backward.circle"
        case .appealed:
            statusLabel = "Appealed"
            statusIcon = "exclamationmark.triangle"
        case .unknown:
            statusLabel = "Unknown"
            statusIcon = "questionmark.circle"
        }
    }

    public func openPortal() {
        guard let url = portalUrl else { return }
        onOpenPortal?(url)
    }

    public func dismiss() {
        onDismiss?()
    }
}
