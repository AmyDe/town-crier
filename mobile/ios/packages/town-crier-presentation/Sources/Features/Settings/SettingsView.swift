import SwiftUI
import TownCrierDomain

/// Centralised settings screen: account, notifications, subscription, attribution, and account management.
public struct SettingsView: View {
  @StateObject private var viewModel: SettingsViewModel
  private var onNotificationPreferences: (() -> Void)?
  private var onManageSubscription: (() -> Void)?
  private var onPrivacyPolicy: (() -> Void)?
  private var onTermsOfService: (() -> Void)?

  public init(
    viewModel: SettingsViewModel,
    onNotificationPreferences: (() -> Void)? = nil,
    onManageSubscription: (() -> Void)? = nil,
    onPrivacyPolicy: (() -> Void)? = nil,
    onTermsOfService: (() -> Void)? = nil
  ) {
    _viewModel = StateObject(wrappedValue: viewModel)
    self.onNotificationPreferences = onNotificationPreferences
    self.onManageSubscription = onManageSubscription
    self.onPrivacyPolicy = onPrivacyPolicy
    self.onTermsOfService = onTermsOfService
  }

  public var body: some View {
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
    .background(Color.tcBackground)
    .scrollContentBackground(.hidden)
    .navigationTitle("Settings")
    .task { await viewModel.load() }
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

  /// A label-value row: primary-styled label on the left, secondary-styled value on the right.
  private func settingRow(label: String, value: String) -> some View {
    HStack {
      settingLabel(label)
      Spacer()
      settingValue(value)
    }
  }

  /// Body text styled as a setting label (primary foreground).
  private func settingLabel(_ text: String) -> some View {
    Text(text)
      .font(TCTypography.body)
      .foregroundStyle(Color.tcTextPrimary)
  }

  /// Body text styled as a setting value (secondary foreground).
  private func settingValue(_ text: String) -> some View {
    Text(text)
      .font(TCTypography.body)
      .foregroundStyle(Color.tcTextSecondary)
  }

  /// Caption text styled for metadata (secondary foreground).
  private func settingCaption(_ text: String) -> some View {
    Text(text)
      .font(TCTypography.caption)
      .foregroundStyle(Color.tcTextSecondary)
  }

  /// A tappable row with a label, SF Symbol icon, and trailing chevron disclosure indicator.
  private func navigationRow(
    _ title: String, systemImage: String, action: @escaping () -> Void
  ) -> some View {
    Button(action: action) {
      HStack {
        Label(title, systemImage: systemImage)
          .font(TCTypography.body)
          .foregroundStyle(Color.tcTextPrimary)
        Spacer()
        settingChevron
      }
    }
  }

  /// Trailing chevron indicator for navigation rows.
  private var settingChevron: some View {
    Image(systemName: "chevron.right")
      .font(.system(.caption))
      .foregroundStyle(Color.tcTextTertiary)
  }

  // MARK: - Account

  private var accountSection: some View {
    Section {
      if let email = viewModel.userEmail {
        settingRow(label: "Email", value: email)

        if let name = viewModel.userName {
          settingRow(label: "Name", value: name)
        }

        if let method = viewModel.authMethod {
          settingRow(label: "Sign-in Method", value: method.displayName)
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
      navigationRow("Notification Preferences", systemImage: "bell") {
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
        settingLabel("Current Plan")
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

      navigationRow("Manage Subscription", systemImage: "creditcard") {
        onManageSubscription?()
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
          settingCaption(item.detail)
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
      navigationRow("Privacy Policy", systemImage: "hand.raised") {
        onPrivacyPolicy?()
      }

      navigationRow("Terms of Service", systemImage: "doc.text") {
        onTermsOfService?()
      }
    } header: {
      Text("Legal")
        .font(TCTypography.captionEmphasis)
    }
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
      HStack {
        settingLabel("Version")
        Spacer()
        settingCaption(viewModel.appVersion)
      }
    }
  }
}
