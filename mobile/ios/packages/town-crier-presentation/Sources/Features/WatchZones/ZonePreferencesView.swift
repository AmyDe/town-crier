import SwiftUI
import TownCrierDomain

/// Per-zone notification preferences screen with four per-channel toggles
/// (push/email × new-application/decision) grouped under two semantic sections.
///
/// Mirrors the web counterpart's accessibility shape — each toggle exposes an
/// em-dash-separated accessibility label (e.g. "New applications — push") so
/// VoiceOver and UI tests can address each control unambiguously.
public struct ZonePreferencesView: View {
  @StateObject private var viewModel: ZonePreferencesViewModel
  @Environment(\.dismiss) private var dismiss

  public init(viewModel: ZonePreferencesViewModel) {
    _viewModel = StateObject(wrappedValue: viewModel)
  }

  public var body: some View {
    NavigationStack {
      Form {
        newApplicationsSection
        decisionUpdatesSection
        if let error = viewModel.error {
          errorSection(error)
        }
      }
      .navigationTitle(viewModel.zoneName)
      #if os(iOS)
        .navigationBarTitleDisplayMode(.inline)
      #endif
      .toolbar {
        ToolbarItem(placement: .cancellationAction) {
          Button("Cancel") { dismiss() }
        }
        ToolbarItem(placement: .confirmationAction) {
          Button("Save") {
            Task {
              await viewModel.savePreferences()
              if viewModel.error == nil {
                dismiss()
              }
            }
          }
          .disabled(viewModel.isLoading)
        }
      }
      .task {
        await viewModel.loadPreferences()
      }
    }
  }

  // MARK: - Sections

  private var newApplicationsSection: some View {
    Section {
      Toggle("Push", isOn: $viewModel.newApplicationPush)
        .tint(Color.tcAmber)
        .accessibilityLabel("New applications — push")

      Toggle("Email", isOn: $viewModel.newApplicationEmail)
        .tint(Color.tcAmber)
        .accessibilityLabel("New applications — email")
    } header: {
      Text("New applications")
    } footer: {
      Text("Get notified when a new planning application is submitted in this zone.")
        .font(.system(.caption))
        .foregroundStyle(Color.tcTextSecondary)
    }
  }

  private var decisionUpdatesSection: some View {
    Section {
      Toggle("Push", isOn: $viewModel.decisionPush)
        .tint(Color.tcAmber)
        .accessibilityLabel("Decision updates — push")

      Toggle("Email", isOn: $viewModel.decisionEmail)
        .tint(Color.tcAmber)
        .accessibilityLabel("Decision updates — email")
    } header: {
      Text("Decision updates")
    } footer: {
      Text("Get notified when a decision is made on an application in this zone.")
        .font(.system(.caption))
        .foregroundStyle(Color.tcTextSecondary)
    }
  }

  private func errorSection(_ error: DomainError) -> some View {
    Section {
      Label {
        Text(error.userMessage)
          .font(.system(.body))
          .foregroundStyle(Color.tcStatusRejected)
      } icon: {
        Image(systemName: "exclamationmark.triangle.fill")
          .foregroundStyle(Color.tcStatusRejected)
      }
    }
  }
}
