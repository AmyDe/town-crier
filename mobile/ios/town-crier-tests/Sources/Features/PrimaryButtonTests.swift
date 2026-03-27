import SwiftUI
import Testing

@testable import TownCrierPresentation

@Suite("PrimaryButton")
@MainActor
struct PrimaryButtonTests {

    // MARK: - Initialization

    @Test func init_withTitleAndAction_createsButton() {
        var actionCalled = false
        let sut = PrimaryButton("Sign in") { actionCalled = true }
        _ = sut
        // The button should exist without crashing.
        #expect(!actionCalled)
    }

    @Test func init_acceptsStringTitle() {
        let sut = PrimaryButton("Subscribe") {}
        _ = sut.body
        // Should compile and render without crashing.
    }

    // MARK: - Custom label

    @Test func init_withCustomLabel_rendersBody() {
        let sut = PrimaryButton {
            // action
        } label: {
            HStack {
                Image(systemName: "safari")
                Text("View on Council Portal")
            }
        }
        _ = sut.body
        // Should compile and render without crashing.
    }
}
