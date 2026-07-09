import SwiftUI
import TownCrierDomain

/// The anonymous (pre-signup) Settings tab (GH#879 Phase 3). Exposes
/// exactly: a "Create free account" / "Sign in" section, Appearance, Data
/// Attribution, Legal (Privacy Policy + Terms of Service), and About (Rate
/// the App + Version). MUST NOT render account info, notification
/// preferences, subscription, data export, sign out, or delete account —
/// there is no such data to show `AnonymousSettingsViewModel` at all.
///
/// Row/section chrome is shared with the authenticated `SettingsView` via
/// `SettingsRowStyling`. Navigation (sign-up entry, legal document
/// presentation, App Store review) is decided by the coordinator, passed in
/// as closures — mirrors `SettingsView`'s own closure-injection shape.
public struct AnonymousSettingsView: View {
  @StateObject private var viewModel: AnonymousSettingsViewModel
  private let onCreateAccount: () -> Void
  private let onSignIn: () -> Void
  private let onPrivacyPolicy: () -> Void
  private let onTermsOfService: () -> Void
  private let onRateApp: () -> Void

  public init(
    viewModel: AnonymousSettingsViewModel,
    onCreateAccount: @escaping () -> Void,
    onSignIn: @escaping () -> Void,
    onPrivacyPolicy: @escaping () -> Void,
    onTermsOfService: @escaping () -> Void,
    onRateApp: @escaping () -> Void
  ) {
    _viewModel = StateObject(wrappedValue: viewModel)
    self.onCreateAccount = onCreateAccount
    self.onSignIn = onSignIn
    self.onPrivacyPolicy = onPrivacyPolicy
    self.onTermsOfService = onTermsOfService
    self.onRateApp = onRateApp
  }

  // MARK: - Test-only seams
  //
  // Mirror `SettingsView`'s test-only seams: invoke a callback as if the
  // user had tapped the corresponding row, verifiable without ViewInspector
  // or UI-level automation.

  public func requestCreateAccount() { onCreateAccount() }
  public func requestSignIn() { onSignIn() }
  public func requestPrivacyPolicy() { onPrivacyPolicy() }
  public func requestTermsOfService() { onTermsOfService() }
  public func requestRateApp() { onRateApp() }

  public var body: some View {
    List {
      createAccountSection
      appearanceSection
      attributionSection
      legalSection
      aboutSection
    }
    .scrollContentBackground(.hidden)
    .background(Color.tcBackground)
    .navigationTitle("Settings")
  }

  // MARK: - Create account

  private var createAccountSection: some View {
    Section {
      Button {
        onCreateAccount()
      } label: {
        // Amber rationing (GH#857/#896): this row is a List navigation
        // action, not a filled CTA container, so it takes the same
        // tcTextPrimary treatment as the authed SettingsView's "Manage
        // Subscription" row rather than amber.
        Text("Create free account")
          .font(TCTypography.bodyEmphasis)
          .foregroundStyle(Color.tcTextPrimary)
      }
      Button {
        onSignIn()
      } label: {
        Text("Sign in")
          .font(TCTypography.body)
          .foregroundStyle(Color.tcTextPrimary)
      }
    } header: {
      sectionHeader("Account")
    } footer: {
      Text(
        "Create a free account to save applications, set up alerts, and manage watch zones."
      )
      .font(TCTypography.caption)
      .foregroundStyle(Color.tcTextSecondary)
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
      sectionHeader("Appearance")
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
      sectionHeader("Data Attribution")
    }
  }

  // MARK: - Legal

  private var legalSection: some View {
    Section {
      SettingsRowStyling.navigationRow("Privacy Policy", systemImage: "hand.raised") {
        onPrivacyPolicy()
      }
      SettingsRowStyling.navigationRow("Terms of Service", systemImage: "doc.text") {
        onTermsOfService()
      }
    } header: {
      sectionHeader("Legal")
    }
  }

  // MARK: - About

  private var aboutSection: some View {
    Section {
      SettingsRowStyling.navigationRow("Rate the App", systemImage: "star") {
        onRateApp()
      }
      HStack {
        SettingsRowStyling.settingLabel("Version")
        Spacer()
        SettingsRowStyling.settingCaption(viewModel.appVersion)
      }
    } header: {
      sectionHeader("About")
    }
  }

  // MARK: - Section header styling (GH#857/#896)

  /// Small-caps kerned section label — the same eyebrow/stamp text
  /// treatment used elsewhere in the design language (``MastheadView``,
  /// ``StatusBadgeView``), applied here to plain `List` section headers.
  /// Scoped to this file only: the authenticated `SettingsView` shares row
  /// chrome via `SettingsRowStyling`, but its section headers are untouched
  /// by this bead (out of scope — that surface was #857's).
  private func sectionHeader(_ title: String) -> some View {
    Text(title)
      .font(TCTypography.captionEmphasis)
      .textCase(.uppercase)
      .kerning(0.6)
  }
}
