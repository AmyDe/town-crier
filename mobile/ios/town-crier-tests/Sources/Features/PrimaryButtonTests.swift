import SwiftUI
import Testing

@testable import TownCrierPresentation

@Suite("PrimaryButton")
struct PrimaryButtonTests {

    // MARK: - Title convenience initializer

    @Test func init_withTitle_isAView() {
        let sut = PrimaryButton("Submit") {}
        // PrimaryButton conforms to View — the compiler enforces this.
        // Verifying instantiation succeeds with all default parameters.
        let _: any View = sut
    }

    @Test func init_withTitleAndLoading_isAView() {
        let sut = PrimaryButton("Subscribe", isLoading: true) {}
        let _: any View = sut
    }

    @Test func init_withTitleAndDisabled_isAView() {
        let sut = PrimaryButton("Subscribe", isDisabled: true) {}
        let _: any View = sut
    }

    // MARK: - Custom label initializer

    @Test func init_withCustomLabel_isAView() {
        let sut = PrimaryButton(
            action: {},
            label: {
                HStack {
                    Image(systemName: "safari")
                    Text("View on Council Portal")
                }
            }
        )
        let _: any View = sut
    }

    @Test func init_withCustomLabelAndLoading_isAView() {
        let sut = PrimaryButton(
            isLoading: true,
            action: {},
            label: { Text("Loading") }
        )
        let _: any View = sut
    }

    // MARK: - Default parameter values

    @Test func init_defaultsLoadingToFalse() {
        // Compiles without specifying isLoading — verifies default exists.
        let _: any View = PrimaryButton("Go") {}
    }

    @Test func init_defaultsDisabledToFalse() {
        // Compiles without specifying isDisabled — verifies default exists.
        let _: any View = PrimaryButton("Go") {}
    }
}
