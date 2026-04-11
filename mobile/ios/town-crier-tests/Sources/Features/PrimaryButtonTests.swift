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

  // MARK: - Conditional label content

  @Test func init_withConditionalLabel_rendersBodyWhenNotLoading() {
    let isLoading = false
    let sut = PrimaryButton {
      // action
    } label: {
      if isLoading {
        ProgressView()
      } else {
        Text("Continue")
      }
    }
    _ = sut.body
    // PrimaryButton must support conditional @ViewBuilder labels
    // for onboarding views that show a ProgressView during loading.
  }

  @Test func init_withConditionalLabel_rendersBodyWhenLoading() {
    let isLoading = true
    let sut = PrimaryButton {
      // action
    } label: {
      if isLoading {
        ProgressView()
      } else {
        Text("Continue")
      }
    }
    _ = sut.body
    // Verify the ProgressView branch also renders.
  }
}
