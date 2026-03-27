import Combine
import Foundation
import TownCrierDomain

/// A single display-ready item in the status timeline.
public struct TimelineItem: Equatable, Sendable {
    public let label: String
    public let icon: String
    public let dateFormatted: String
    public let isCurrent: Bool
    public let status: ApplicationStatus
}

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
    public let timelineItems: [TimelineItem]

    public var onOpenPortal: ((URL) -> Void)?
    public var onDismiss: (() -> Void)?

    public var hasPortalUrl: Bool {
        portalUrl != nil
    }

    public var hasTimeline: Bool {
        !timelineItems.isEmpty
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

        statusLabel = application.status.displayLabel
        statusIcon = application.status.displayIcon

        let events = application.statusHistory.isEmpty
            ? [StatusEvent(status: application.status, date: application.receivedDate)]
            : application.statusHistory.sorted()

        timelineItems = events.enumerated().map { index, event in
            let isLast = index == events.count - 1
            return TimelineItem(
                label: event.status.displayLabel,
                icon: event.status.displayIcon,
                dateFormatted: Self.dateFormatter.string(from: event.date),
                isCurrent: isLast,
                status: event.status
            )
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
