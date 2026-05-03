import SwiftUI
import TownCrierDomain

/// In-app notification preferences screen — replaces the iOS-Settings deep
/// link that previously fronted the "Notification Preferences" row.
///
/// Sections:
///   1. Permission (only when status is `.notDetermined` or `.denied`)
///   2. Saved Applications (push + email toggles)
///   3. Email Digest (weekly toggle + day picker)
///   4. Watch Zones (read-only count → navigation row that switches tabs)
///
/// A footer link deep-links into iOS system Settings for *how* notifications
/// are delivered (banners/sounds/Focus modes/badges) — hidden when status is
/// `.notDetermined` because iOS has no per-app subpage to deep-link to until
/// the user makes a choice (the link otherwise drops them into the system
/// Notifications hub).
public struct NotificationPreferencesView: View {
  @StateObject private var viewModel: NotificationPreferencesViewModel
  @Environment(\.scenePhase) private var scenePhase
  private var onZonesTap: (() -> Void)?
  private var onSystemSettingsTap: (() -> Void)?

  public init(
    viewModel: NotificationPreferencesViewModel,
    onZonesTap: (() -> Void)? = nil,
    onSystemSettingsTap: (() -> Void)? = nil
  ) {
    _viewModel = StateObject(wrappedValue: viewModel)
    self.onZonesTap = onZonesTap
    self.onSystemSettingsTap = onSystemSettingsTap
  }

  /// `true` when the iOS-Settings footer link should be hidden. Mirrors the
  /// spec table: hide only when `.notDetermined` (no subpage exists yet);
  /// show in all other cases including the still-loading `nil` state to
  /// avoid a flash of UI as the status resolves.
  public var shouldHideSystemSettingsLink: Bool {
    viewModel.authorizationStatus == .notDetermined
  }

  /// Test-only seam: invoke the watch-zones callback as if the user had
  /// tapped the row. Mirrors `SettingsView.requestNotificationPreferences()`
  /// so the wiring is verifiable without UI-level automation.
  public func requestZonesTap() {
    onZonesTap?()
  }

  /// Test-only seam: invoke the iOS-Settings footer-link callback as if the
  /// user had tapped it.
  public func requestSystemSettingsTap() {
    onSystemSettingsTap?()
  }

  /// Test-only seam: invoke the permission-section "Turn on notifications"
  /// button as if the user had tapped it.
  public func requestPermissionButtonTap() async {
    await viewModel.requestPermission()
  }

  /// Test-only seam: simulate the View transitioning to the active scene
  /// phase. Mirrors the `.onChange(of: scenePhase)` handler that triggers a
  /// foreground refresh of the authorization status.
  public func requestScenePhaseActive() async {
    await viewModel.refreshAuthorizationStatus()
  }

  public var body: some View {
    Form {
      permissionSection
      savedApplicationsSection
      emailDigestSection
      watchZonesSection
      if !shouldHideSystemSettingsLink {
        systemSettingsFooter
      }
      if let error = viewModel.error {
        errorSection(error)
      }
    }
    .background(Color.tcBackground)
    .scrollContentBackground(.hidden)
    .navigationTitle("Notifications")
    #if os(iOS)
      .navigationBarTitleDisplayMode(.inline)
    #endif
    .task { await viewModel.load() }
    .onChange(of: scenePhase) { _, newPhase in
      guard newPhase == .active else { return }
      Task { await viewModel.refreshAuthorizationStatus() }
    }
  }

  // MARK: - Permission section

  @ViewBuilder
  private var permissionSection: some View {
    switch viewModel.authorizationStatus {
    case .notDetermined:
      Section {
        Text("Town Crier needs permission to send you notifications.")
          .font(TCTypography.body)
          .foregroundStyle(Color.tcTextPrimary)
        Button {
          Task { await viewModel.requestPermission() }
        } label: {
          Text("Turn on notifications")
            .font(TCTypography.body)
            .foregroundStyle(Color.tcAmber)
        }
        .accessibilityLabel("Turn on notifications")
      } header: {
        Text("Notifications")
          .font(TCTypography.captionEmphasis)
      }
    case .denied:
      Section {
        Text(
          "Notifications are turned off for Town Crier. "
            + "Turn them on in iOS Settings to receive alerts."
        )
        .font(TCTypography.body)
        .foregroundStyle(Color.tcTextPrimary)
      } header: {
        Text("Notifications")
          .font(TCTypography.captionEmphasis)
      }
    case .authorized, .none:
      EmptyView()
    }
  }

  // MARK: - Sections

  private var savedApplicationsSection: some View {
    Section {
      Toggle(
        "Push",
        isOn: Binding(
          get: { viewModel.savedDecisionPush },
          set: { newValue in
            Task { await viewModel.setSavedDecisionPush(newValue) }
          }
        )
      )
      .tint(Color.tcAmber)
      .accessibilityLabel("Saved applications — push")

      Toggle(
        "Email",
        isOn: Binding(
          get: { viewModel.savedDecisionEmail },
          set: { newValue in
            Task { await viewModel.setSavedDecisionEmail(newValue) }
          }
        )
      )
      .tint(Color.tcAmber)
      .accessibilityLabel("Saved applications — email")
    } header: {
      Text("Saved applications")
        .font(TCTypography.captionEmphasis)
    } footer: {
      Text("Get notified when there's a decision on an application you've saved.")
        .font(TCTypography.caption)
        .foregroundStyle(Color.tcTextSecondary)
    }
  }

  private var emailDigestSection: some View {
    Section {
      Toggle(
        "Weekly digest",
        isOn: Binding(
          get: { viewModel.emailDigestEnabled },
          set: { newValue in
            Task { await viewModel.setEmailDigestEnabled(newValue) }
          }
        )
      )
      .tint(Color.tcAmber)
      .accessibilityLabel("Weekly digest")

      Picker(
        "Send on",
        selection: Binding(
          get: { viewModel.digestDay },
          set: { newValue in
            Task { await viewModel.setDigestDay(newValue) }
          }
        )
      ) {
        ForEach(DayOfWeek.allCases, id: \.self) { day in
          Text(day.displayName).tag(day)
        }
      }
      .disabled(!viewModel.emailDigestEnabled)
      .accessibilityLabel("Digest day")
    } header: {
      Text("Email digest")
        .font(TCTypography.captionEmphasis)
    } footer: {
      Text("A weekly summary of new applications in your watched zones.")
        .font(TCTypography.caption)
        .foregroundStyle(Color.tcTextSecondary)
    }
  }

  private var watchZonesSection: some View {
    Section {
      Button {
        onZonesTap?()
      } label: {
        HStack {
          Label("Watch zones", systemImage: "mappin.and.ellipse")
            .font(TCTypography.body)
            .foregroundStyle(Color.tcTextPrimary)
          Spacer()
          if let count = viewModel.watchZoneCount {
            Text(zonesValueLabel(for: count))
              .font(TCTypography.body)
              .foregroundStyle(Color.tcTextSecondary)
          } else {
            ProgressView()
              .accessibilityLabel("Loading watch-zone count")
          }
          Image(systemName: "chevron.right")
            .font(.system(.caption))
            .foregroundStyle(Color.tcTextTertiary)
        }
      }
      .accessibilityLabel("Watch zones")
    } header: {
      Text("Watch zones")
        .font(TCTypography.captionEmphasis)
    } footer: {
      Text("Per-zone notification preferences are managed in the Zones tab.")
        .font(TCTypography.caption)
        .foregroundStyle(Color.tcTextSecondary)
    }
  }

  private var systemSettingsFooter: some View {
    Section {
      Button {
        onSystemSettingsTap?()
      } label: {
        HStack {
          Label("Open iOS notification settings", systemImage: "gearshape")
            .font(TCTypography.body)
            .foregroundStyle(Color.tcTextPrimary)
          Spacer()
          Image(systemName: "arrow.up.right.square")
            .font(.system(.caption))
            .foregroundStyle(Color.tcTextTertiary)
        }
      }
      .accessibilityLabel("Open iOS notification settings")
    } footer: {
      Text("Banner style, sounds, badges, and Focus modes are managed by iOS.")
        .font(TCTypography.caption)
        .foregroundStyle(Color.tcTextSecondary)
    }
  }

  private func errorSection(_ error: DomainError) -> some View {
    Section {
      Label {
        Text(error.userMessage)
          .font(TCTypography.body)
          .foregroundStyle(Color.tcStatusRejected)
      } icon: {
        Image(systemName: "exclamationmark.triangle.fill")
          .foregroundStyle(Color.tcStatusRejected)
      }
    }
  }

  private func zonesValueLabel(for count: Int) -> String {
    switch count {
    case 0:
      return "No zones yet"
    case 1:
      return "1 zone"
    default:
      return "\(count) zones"
    }
  }
}
