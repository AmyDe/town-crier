import SwiftUI

/// The anonymous (pre-signup) tab shell (GH#879 Phase 3), parallel to the
/// authenticated `MainTabView` (`town-crier-app/Sources/MainTabView.swift`):
/// three tabs — Applications, Map, Settings. No Saved tab (saving is
/// account-bound, ADR 0035); no Zones tab yet (device-local zones arrive in
/// Phase 4). Replaces the bare `AnonymousMapView` as the post-postcode
/// destination, and as what a persisted anonymous session relaunches into.
///
/// The persistent ``AccountCTABanner`` appears over Applications and Map via
/// the shared `View.accountCTABanner(onCreateAccount:onSignIn:)` modifier,
/// applied INSIDE each of those tabs' own content — never on this `TabView`
/// itself. Hosting it at the `TabView` level was tried first and found, via
/// live simulator verification, to draw the banner over the tab bar
/// (`.safeAreaInset(edge: .bottom)` on a `TabView` insets against the
/// *window's* bottom edge, not the safe area above the tab bar), making
/// Map/Settings entirely unreachable — see `AccountCTABanner.swift` for the
/// full writeup. It is omitted on Settings — Settings already has its own
/// prominent "Create free account" section, so a second copy of the same
/// pitch would be redundant clutter (design-language: calm clarity, one
/// hero element per screen).
public struct AnonymousMainTabView: View {
  @ObservedObject var coordinator: AnonymousBrowseCoordinator

  public init(coordinator: AnonymousBrowseCoordinator) {
    self.coordinator = coordinator
  }

  public var body: some View {
    TabView(selection: $coordinator.selectedTab) {
      applicationsTab
      mapTab
      settingsTab
    }
    .tint(Color.tcAmber)
  }

  // MARK: - Tabs

  @ViewBuilder
  private var applicationsTab: some View {
    NavigationStack {
      if let listViewModel = coordinator.makeApplicationListViewModel() {
        AnonymousApplicationListView(viewModel: listViewModel)
          .accountCTABanner(
            onCreateAccount: { coordinator.requestSignIn() },
            onSignIn: { coordinator.requestSignIn() }
          )
      }
    }
    .tabItem {
      Label("Applications", systemImage: "doc.text.magnifyingglass")
    }
    .tag(AnonymousBrowseCoordinator.Tab.applications)
  }

  @ViewBuilder
  private var mapTab: some View {
    NavigationStack {
      if let mapViewModel = coordinator.mapViewModel {
        AnonymousMapView(viewModel: mapViewModel)
          .navigationTitle("Map")
          #if os(iOS)
            .navigationBarTitleDisplayMode(.inline)
          #endif
          .accountCTABanner(
            onCreateAccount: { coordinator.requestSignIn() },
            onSignIn: { coordinator.requestSignIn() }
          )
      }
    }
    .tabItem {
      Label("Map", systemImage: "map")
    }
    .tag(AnonymousBrowseCoordinator.Tab.map)
  }

  private var settingsTab: some View {
    NavigationStack {
      AnonymousSettingsView(
        viewModel: coordinator.makeSettingsViewModel(),
        onCreateAccount: { coordinator.requestSignIn() },
        onSignIn: { coordinator.requestSignIn() },
        onPrivacyPolicy: { coordinator.showPrivacyPolicy() },
        onTermsOfService: { coordinator.showTermsOfService() },
        onRateApp: { coordinator.requestRateApp() }
      )
    }
    .tabItem {
      Label("Settings", systemImage: "gearshape")
    }
    .tag(AnonymousBrowseCoordinator.Tab.settings)
  }
}
