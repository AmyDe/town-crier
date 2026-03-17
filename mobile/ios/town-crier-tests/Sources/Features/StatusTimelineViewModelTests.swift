import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("ApplicationDetailViewModel timeline")
@MainActor
struct StatusTimelineViewModelTests {

    // MARK: - Timeline Items from History

    @Test func timelineItems_withHistory_returnsFormattedItems() {
        let sut = ApplicationDetailViewModel(application: .approvedWithHistory)

        #expect(sut.timelineItems.count == 2)
        #expect(sut.timelineItems[0].label == "Pending")
        #expect(sut.timelineItems[1].label == "Approved")
    }

    @Test func timelineItems_withHistory_formatsDateInUKLocale() {
        let sut = ApplicationDetailViewModel(application: .approvedWithHistory)

        // 1_700_000_000 = 14 Nov 2023 UTC
        #expect(sut.timelineItems[0].dateFormatted == "14 Nov 2023")
    }

    @Test func timelineItems_lastItemIsCurrentStatus() {
        let sut = ApplicationDetailViewModel(application: .approvedWithHistory)

        #expect(sut.timelineItems[0].isCurrent == false)
        #expect(sut.timelineItems[1].isCurrent == true)
    }

    @Test func timelineItems_includesStatusIcon() {
        let sut = ApplicationDetailViewModel(application: .approvedWithHistory)

        #expect(sut.timelineItems[0].icon == "clock")
        #expect(sut.timelineItems[1].icon == "checkmark.circle")
    }

    // MARK: - Single Status (no history)

    @Test func timelineItems_noHistory_returnsSingleReceivedItem() {
        let sut = ApplicationDetailViewModel(application: .pendingReview)

        #expect(sut.timelineItems.count == 1)
        #expect(sut.timelineItems[0].label == "Pending")
        #expect(sut.timelineItems[0].isCurrent == true)
        #expect(sut.timelineItems[0].dateFormatted == "14 Nov 2023")
    }

    // MARK: - Refused outcome

    @Test func timelineItems_refused_showsRefusedAsCurrent() {
        let sut = ApplicationDetailViewModel(application: .refusedWithHistory)

        #expect(sut.timelineItems.last?.label == "Refused")
        #expect(sut.timelineItems.last?.isCurrent == true)
        #expect(sut.timelineItems.last?.icon == "xmark.circle")
    }

    // MARK: - Has Timeline

    @Test func hasTimeline_withHistory_returnsTrue() {
        let sut = ApplicationDetailViewModel(application: .approvedWithHistory)

        #expect(sut.hasTimeline)
    }

    @Test func hasTimeline_noHistory_singleItem_returnsTrue() {
        let sut = ApplicationDetailViewModel(application: .pendingReview)

        #expect(sut.hasTimeline)
    }
}
