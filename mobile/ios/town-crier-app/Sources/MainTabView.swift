import SwiftUI
import TownCrierPresentation

/// The main authenticated tab view (Applications, Saved, Map, Zones) plus its
/// Settings sheet — extracted from `TownCrierApp.swift` into its own `View`
/// (rather than an extension on the App struct) so `coordinator` and
/// `settingsViewModel` can stay genuinely `private` `@StateObject`s on
/// `TownCrierApp` (SwiftUI state properties should be private) while this
/// view observes them via `@ObservedObject`. Also keeps `TownCrierApp.swift`
/// within the project's file-length limit — GH#868 Phase 3's anonymous
/// browse composition root wiring pushed it over 400 lines.
struct MainTabView: View {
  @ObservedObject var coordinator: AppCoordinator
  @ObservedObject var settingsViewModel: SettingsViewModel

  var body: some View {
    TabView(selection: $coordinator.selectedTab) {
      // 1. Applications
      NavigationStack {
        VStack(spacing: 0) {
          // Paid-user push-permission nudge (issue #624, Prong 2). Hidden
          // unless the user is on a paid tier and notifications are not
          // authorized. The `.id` is hoisted onto the VStack so both the
          // banner and the list rebuild when the resolved tier changes (e.g.
          // straight after a purchase).
          PushNudgeBanner(viewModel: coordinator.makePushNudgeViewModel())
          ApplicationListView(viewModel: coordinator.makeApplicationListViewModel())
        }
        .id(coordinator.subscriptionTier)
        .settingsToolbar { coordinator.showSettings() }
      }
      .tabItem {
        Label("Applications", systemImage: "doc.text.magnifyingglass")
      }
      .tag(MainTab.applications)

      // 2. Saved
      NavigationStack {
        SavedApplicationListView(
          viewModel: coordinator.makeSavedApplicationListViewModel()
        )
        .id(coordinator.subscriptionTier)
        .settingsToolbar { coordinator.showSettings() }
      }
      .tabItem {
        Label("Saved", systemImage: "bookmark.fill")
      }
      .tag(MainTab.saved)

      // 3. Map
      NavigationStack {
        MapView(viewModel: coordinator.makeMapViewModel())
          .id(coordinator.subscriptionTier)
          .navigationTitle("Map")
          #if os(iOS)
            .navigationBarTitleDisplayMode(.inline)
          #endif
          .settingsToolbar { coordinator.showSettings() }
      }
      .tabItem {
        Label("Map", systemImage: "map")
      }
      .tag(MainTab.map)

      // 4. Zones
      NavigationStack {
        WatchZoneListView(viewModel: coordinator.makeWatchZoneListViewModel())
          .id(coordinator.subscriptionTier)
          .settingsToolbar { coordinator.showSettings() }
      }
      .sheet(isPresented: $coordinator.isAddingWatchZone) {
        WatchZoneEditorView(
          viewModel: coordinator.makeWatchZoneEditorViewModel()
        )
      }
      .sheet(item: $coordinator.editingWatchZone) { zone in
        WatchZoneEditorView(
          viewModel: coordinator.makeWatchZoneEditorViewModel(editing: zone)
        )
      }
      .tabItem {
        Label("Zones", systemImage: "mappin.and.ellipse")
      }
      .tag(MainTab.zones)
    }
    .tint(Color.tcAmber)
    .sheet(isPresented: $coordinator.isSettingsPresented) {
      settingsSheet
    }
    // Subscription paywall — presented when an upsell (e.g. "View Plans" in
    // the watch-zone quota banner) sets `isSubscriptionPresented`. Hoisted to
    // the TabView so the paywall reaches the user regardless of active tab.
    // On dismiss, re-resolve the tier so a successful purchase unlocks gated
    // features (e.g. the larger watch-zone radius) live, without an app
    // relaunch — the tier-keyed views rebuild on the change (tc-w3cb.3).
    .sheet(
      isPresented: $coordinator.isSubscriptionPresented,
      onDismiss: { Task { await coordinator.resolveSubscriptionTier() } },
      content: {
        NavigationStack {
          SubscriptionView(viewModel: coordinator.makeSubscriptionViewModel())
        }
      }
    )
    // Post-signup "Add your other areas" conversion sheet (GH#879 Phase 5):
    // presented once immediately after the wizard completes when unconverted
    // device-local zones remain, and reopened from the authed Zones tab's
    // dismissible row for as long as any remain. Content is rebuilt fresh on
    // every presentation so it never shows already-converted/deleted zones.
    // Hoisted to the TabView, mirroring the paywall sheet above, so it
    // reaches the user regardless of active tab.
    .sheet(isPresented: $coordinator.isDeviceLocalZoneConversionPresented) {
      if let viewModel = coordinator.makeDeviceLocalZoneConversionViewModel() {
        DeviceLocalZoneConversionView(viewModel: viewModel)
          .selfSizingSheet()
      }
    }
  }

  /// Settings sheet — presented from the gear icon installed on every tab.
  /// Hosts the existing SettingsView and the legal/notification/manage-sub
  /// side-effects that were previously bound to the Settings tab.
  @ViewBuilder
  private var settingsSheet: some View {
    NavigationStack {
      SettingsView(
        viewModel: settingsViewModel,
        onNotificationPreferences: {
          coordinator.showNotificationPreferences()
        },
        onManageSubscription: {
          coordinator.showManageSubscription()
        },
        onPrivacyPolicy: {
          coordinator.showPrivacyPolicy()
        },
        onTermsOfService: {
          coordinator.showTermsOfService()
        },
        // Surfacing the "Redeem Offer Code" row only when an OfferCodeService
        // was injected — SettingsView hides the row when this callback is nil.
        onRedeemOfferCode: coordinator.isOfferCodeRedemptionAvailable
          ? { coordinator.showRedeemOfferCode() }
          : nil,
        onRateApp: coordinator.rateApp
      )
      .navigationDestination(isPresented: $coordinator.isNotificationPreferencesPresented) {
        NotificationPreferencesView(
          viewModel: coordinator.makeNotificationPreferencesViewModel(),
          onZonesTap: {
            coordinator.isSettingsPresented = false
            coordinator.selectedTab = .zones
          },
          onSystemSettingsTap: {
            coordinator.showSystemNotificationSettings()
          }
        )
      }
    }
    .sheet(item: $coordinator.presentedLegalDocument) { documentType in
      NavigationStack {
        LegalDocumentView(viewModel: LegalDocumentViewModel(documentType: documentType))
      }
    }
    // Offer-code redemption — presented from the "Redeem Offer Code" row in
    // Settings (ADR 0022). The factory returns nil if no OfferCodeService was
    // injected, in which case the row is also hidden, so the sheet body is a
    // no-op fallback that should never render.
    .sheet(isPresented: $coordinator.isRedeemOfferCodePresented) {
      NavigationStack {
        if let viewModel = coordinator.makeRedeemOfferCodeViewModel() {
          RedeemOfferCodeView(viewModel: viewModel)
        }
      }
    }
    #if os(iOS)
      .manageSubscriptionsSheet(
        isPresented: $coordinator.isManageSubscriptionPresented.dispatchingSetOnMain()
      )
    #endif
    // App-layer edge for coordinator-driven deep links: open the URL and reset
    // the flag. Extracted into a shared modifier (mirrors ReviewPromptRequest-
    // Modifier) so the file stays UIKit-free and within length limits.
    .openingSystemNotificationSettings(when: coordinator)
    .openingAppStoreReview(when: coordinator)
  }
}
