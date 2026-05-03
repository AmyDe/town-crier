import SwiftUI
import TownCrierDomain

/// In-app notification preferences screen — replaces the iOS-Settings deep
/// link that previously fronted the "Notification Preferences" row.
///
/// Three sections:
///   1. Saved Applications (push + email toggles)
///   2. Email Digest (weekly toggle + day picker)
///   3. Watch Zones (read-only count → navigation row that switches tabs)
///
/// A footer link still deep-links into iOS system Settings for *how*
/// notifications are delivered (banners/sounds/Focus modes/badges).
public struct NotificationPreferencesView: View {
  @StateObject private var viewModel: NotificationPreferencesViewModel
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

  public var body: some View {
    Form {
      savedApplicationsSection
      emailDigestSection
      watchZonesSection
      systemSettingsFooter
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
          Text(zonesValueLabel)
            .font(TCTypography.body)
            .foregroundStyle(Color.tcTextSecondary)
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

  private var zonesValueLabel: String {
    switch viewModel.watchZoneCount {
    case 0:
      return "No zones yet"
    case 1:
      return "1 zone"
    default:
      return "\(viewModel.watchZoneCount) zones"
    }
  }
}
