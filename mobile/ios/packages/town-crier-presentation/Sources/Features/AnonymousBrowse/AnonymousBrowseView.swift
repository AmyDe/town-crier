import SwiftUI

/// Root view for the anonymous (pre-signup) browse flow (GH#868 Phase 3):
/// renders whichever screen ``AnonymousBrowseCoordinator`` says is current.
/// Rendered by the app shell whenever there is no authenticated session — an
/// authenticated session always wins over this branch entirely.
public struct AnonymousBrowseView: View {
  @ObservedObject var coordinator: AnonymousBrowseCoordinator

  public init(coordinator: AnonymousBrowseCoordinator) {
    self.coordinator = coordinator
  }

  public var body: some View {
    Group {
      switch coordinator.screen {
      case .welcome:
        WelcomeView(viewModel: coordinator.makeWelcomeViewModel())
      case .postcodeEntry:
        AnonymousPostcodeEntryView(viewModel: coordinator.makePostcodeEntryViewModel())
      case .tabs:
        AnonymousMainTabView(coordinator: coordinator)
      }
    }
  }
}
