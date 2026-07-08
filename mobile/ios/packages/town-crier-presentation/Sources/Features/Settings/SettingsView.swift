import SwiftUI
import TownCrierDomain

/// Centralised settings screen: account, notifications, subscription, attribution, and account management.
public struct SettingsView: View {
  @StateObject private var viewModel: SettingsViewModel
  private var onNotificationPreferences: (() -> Void)?
  private var onManageSubscription: (() -> Void)?
  private var onPrivacyPolicy: (() -> Void)?
  private var onTermsOfService: (() -> Void)?
  private var onRedeemOfferCode: (() -> Void)?
  private var onRateApp: (() -> Void)?

  public init(
    viewModel: SettingsViewModel,
    onNotificationPreferences: (() -> Void)? = nil,
    onManageSubscription: (() -> Void)? = nil,
    onPrivacyPolicy: (() -> Void)? = nil,
    onTermsOfService: (() -> Void)? = nil,
    onRedeemOfferCode: (() -> Void)? = nil,
    onRateApp: (() -> Void)? = nil
  ) {
    _viewModel = StateObject(wrappedValue: viewModel)
    self.onNotificationPreferences = onNotificationPreferences
    self.onManageSubscription = onManageSubscription
    self.onPrivacyPolicy = onPrivacyPolicy
    self.onTermsOfService = onTermsOfService
    self.onRedeemOfferCode = onRedeemOfferCode
    self.onRateApp = onRateApp
  }

  /// Test-only seam: invoke the redeem-offer-code callback as if the user had
  /// tapped the row in Settings. Production code routes through SwiftUI's
  /// `Button` action; this mirror keeps the callback testable without
  /// requiring ViewInspector or UI-level automation.
  public func requestRedeemOfferCode() {
    onRedeemOfferCode?()
  }

  /// Test-only seam: invoke the notification-preferences callback as if the
  /// user had tapped the row in Settings. Mirrors `requestRedeemOfferCode` so
  /// the wiring is verifiable without UI-level automation.
  public func requestNotificationPreferences() {
    onNotificationPreferences?()
  }

  /// Test-only seam: trigger the data-export flow as if the user had tapped the
  /// "Export your data" row. Mirrors the other seams so the wiring is
  /// verifiable without UI-level automation.
  public func requestExportData() async {
    await viewModel.exportData()
  }

  /// Test-only seam: invoke the rate-app callback as if the user had tapped the
  /// "Rate the App" row in Settings. Mirrors `requestRedeemOfferCode` so the
  /// wiring is verifiable without UI-level automation (GH #629).
  public func requestRateApp() {
    onRateApp?()
  }

  public var body: some View {
    listContent
      .background(Color.tcBackground)
      .scrollContentBackground(.hidden)
      .navigationTitle("Settings")
      .task { await viewModel.load() }
      .modifier(ExportShareSheetModifier(viewModel: viewModel))
      .alert(
        "Export Failed",
        isPresented: Binding(
          get: { viewModel.exportErrorMessage != nil },
          set: { if !$0 { viewModel.dismissExportError() } }
        )
      ) {
        Button("OK", role: .cancel) { viewModel.dismissExportError() }
      } message: {
        Text(viewModel.exportErrorMessage ?? "")
      }
  }

  private var listContent: some View {
    List {
      accountSection
      appearanceSection
      notificationSection
      subscriptionSection
      attributionSection
      legalSection
      dangerZoneSection
      appInfoSection
    }
    .alert("Delete Account", isPresented: $viewModel.isShowingDeleteConfirmation) {
      Button("Delete", role: .destructive) {
        Task { await viewModel.confirmDeleteAccount() }
      }
      Button("Cancel", role: .cancel) {
        viewModel.cancelDeletion()
      }
    } message: {
      Text(
        "This will permanently delete your account and all associated data. This action cannot be undone."
      )
    }
  }

  // MARK: - Helpers
  //
  // Row/section chrome (`settingRow`, `settingLabel`, `settingValue`,
  // `settingCaption`, `navigationRow`, `settingChevron`) lives in
  // `SettingsRowStyling`, shared with `AnonymousSettingsView` (GH#879 Phase 3)
  // so the two Settings surfaces render identical row chrome with no
  // duplicated helpers.

  // MARK: - Account

  private var accountSection: some View {
    Section {
      if let email = viewModel.userEmail {
        SettingsRowStyling.settingRow(label: "Email", value: email)

        if let name = viewModel.userName {
          SettingsRowStyling.settingRow(label: "Name", value: name)
        }

        if let method = viewModel.authMethod {
          SettingsRowStyling.settingRow(label: "Sign-in Method", value: method.displayName)
        }
      }
    } header: {
      Text("Account")
        .font(TCTypography.captionEmphasis)
    }
  }

  // MARK: - Appearance

  private var appearanceSection: some View {
    Section {
      Picker(selection: $viewModel.appearanceMode) {
        ForEach(AppearanceMode.allCases, id: \.self) { mode in
          Text(mode.displayName).tag(mode)
        }
      } label: {
        Label("Appearance", systemImage: "paintbrush")
          .font(TCTypography.body)
          .foregroundStyle(Color.tcTextPrimary)
      }
    } header: {
      Text("Appearance")
        .font(TCTypography.captionEmphasis)
    }
  }

  // MARK: - Notifications

  private var notificationSection: some View {
    Section {
      SettingsRowStyling.navigationRow("Notification Preferences", systemImage: "bell") {
        onNotificationPreferences?()
      }
    } header: {
      Text("Notifications")
        .font(TCTypography.captionEmphasis)
    }
  }

  // MARK: - Subscription

  private var subscriptionSection: some View {
    Section {
      HStack {
        SettingsRowStyling.settingLabel("Current Plan")
        Spacer()
        HStack(spacing: TCSpacing.extraSmall) {
          Text(viewModel.subscriptionTier.rawValue.capitalized)
            .font(TCTypography.bodyEmphasis)
            .foregroundStyle(Color.tcTextPrimary)
          if viewModel.isTrialPeriod {
            Text("Trial")
              .font(TCTypography.captionEmphasis)
              .foregroundStyle(Color.tcStatusPending)
          }
        }
      }

      SettingsRowStyling.navigationRow("Manage Subscription", systemImage: "creditcard") {
        onManageSubscription?()
      }

      if onRedeemOfferCode != nil {
        SettingsRowStyling.navigationRow("Redeem Offer Code", systemImage: "ticket") {
          onRedeemOfferCode?()
        }
      }
    } header: {
      Text("Subscription")
        .font(TCTypography.captionEmphasis)
    }
  }

  // MARK: - Attribution

  private var attributionSection: some View {
    Section {
      ForEach(viewModel.attributionItems, id: \.name) { item in
        VStack(alignment: .leading, spacing: TCSpacing.extraSmall) {
          Text(item.name)
            .font(TCTypography.bodyEmphasis)
            .foregroundStyle(Color.tcTextPrimary)
          SettingsRowStyling.settingCaption(item.detail)
        }
      }
    } header: {
      Text("Data Attribution")
        .font(TCTypography.captionEmphasis)
    }
  }

  // MARK: - Legal

  private var legalSection: some View {
    Section {
      SettingsRowStyling.navigationRow("Privacy Policy", systemImage: "hand.raised") {
        onPrivacyPolicy?()
      }

      SettingsRowStyling.navigationRow("Terms of Service", systemImage: "doc.text") {
        onTermsOfService?()
      }

      exportDataRow
    } header: {
      Text("Legal")
        .font(TCTypography.captionEmphasis)
    }
  }

  /// "Export your data" row: triggers the GDPR data export. Shows a spinner and
  /// disables while the export is in flight; the resulting file is handed to
  /// the iOS share sheet by the body's `.sheet` modifier.
  private var exportDataRow: some View {
    Button {
      Task { await viewModel.exportData() }
    } label: {
      HStack {
        Label("Export your data", systemImage: "square.and.arrow.up")
          .font(TCTypography.body)
          .foregroundStyle(Color.tcTextPrimary)
        Spacer()
        if viewModel.isExporting {
          ProgressView()
        } else {
          SettingsRowStyling.settingChevron
        }
      }
    }
    .disabled(viewModel.isExporting)
  }

  // MARK: - Danger Zone

  private var dangerZoneSection: some View {
    Section {
      Button {
        Task { await viewModel.logout() }
      } label: {
        Label("Sign Out", systemImage: "rectangle.portrait.and.arrow.right")
          .font(TCTypography.body)
          .foregroundStyle(Color.tcTextPrimary)
      }

      Button(role: .destructive) {
        viewModel.requestAccountDeletion()
      } label: {
        Label("Delete Account", systemImage: "trash")
          .font(TCTypography.body)
      }
    } header: {
      Text("Account")
        .font(TCTypography.captionEmphasis)
    }
  }

  // MARK: - App Info

  private var appInfoSection: some View {
    Section {
      SettingsRowStyling.navigationRow("Rate the App", systemImage: "star") {
        onRateApp?()
      }

      HStack {
        SettingsRowStyling.settingLabel("Version")
        Spacer()
        SettingsRowStyling.settingCaption(viewModel.appVersion)
      }
    } header: {
      Text("About")
        .font(TCTypography.captionEmphasis)
    }
  }
}

/// Presents the data-export share sheet over Settings when the ViewModel has a
/// finished export file ready. The `UIActivityViewController` is iOS-only, so
/// the modifier is a no-op on other platforms (keeping the View buildable for
/// macOS unit tests).
private struct ExportShareSheetModifier: ViewModifier {
  @ObservedObject var viewModel: SettingsViewModel

  func body(content: Content) -> some View {
    #if os(iOS)
      content.sheet(
        isPresented: Binding(
          get: { viewModel.exportFileURL != nil },
          set: { if !$0 { viewModel.dismissExportShare() } }
        )
      ) {
        if let url = viewModel.exportFileURL {
          ShareSheet(activityItems: [url])
        }
      }
    #else
      content
    #endif
  }
}
