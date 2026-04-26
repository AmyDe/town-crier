import SwiftUI
import TownCrierDomain

/// Per-zone notification preferences screen with entitlement-gated toggles.
///
/// - `newApplications` is available to all tiers.
/// - `statusChanges` and `decisionUpdates` use ``GatedToggle`` and require Personal+.
///
/// Presents a ``SubscriptionUpsellSheet`` via the `.entitlementGateSheet` modifier
/// when a Free user taps a gated toggle or the API returns 403.
public struct ZonePreferencesView: View {
  @StateObject private var viewModel: ZonePreferencesViewModel
  @Environment(\.dismiss) private var dismiss

  public init(viewModel: ZonePreferencesViewModel) {
    _viewModel = StateObject(wrappedValue: viewModel)
  }

  public var body: some View {
    NavigationStack {
      Form {
        ungatedSection
        gatedSection
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
              if viewModel.error == nil && viewModel.entitlementGate == nil {
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
      .entitlementGateSheet(entitlement: $viewModel.entitlementGate) {
        // onViewPlans -- parent coordinator would handle navigation to SubscriptionView
      }
    }
  }

  // MARK: - Sections

  private var ungatedSection: some View {
    Section {
      Toggle("New Applications", isOn: $viewModel.newApplications)
        .tint(Color.tcAmber)
    } header: {
      Text("Alerts")
    } footer: {
      Text("Get notified when a new planning application is submitted in this zone.")
        .font(.system(.caption))
        .foregroundStyle(Color.tcTextSecondary)
    }
  }

  private var gatedSection: some View {
    Section {
      GatedToggle(
        label: "Status Changes",
        isOn: $viewModel.statusChanges,
        entitlement: .statusChangeAlerts,
        featureGate: viewModel.featureGate
      ) {
        viewModel.showUpgradeSheet(for: .statusChangeAlerts)
      }

      GatedToggle(
        label: "Decision Updates",
        isOn: $viewModel.decisionUpdates,
        entitlement: .decisionUpdateAlerts,
        featureGate: viewModel.featureGate
      ) {
        viewModel.showUpgradeSheet(for: .decisionUpdateAlerts)
      }
    } header: {
      Text("Premium Alerts")
    } footer: {
      Text("Requires a Personal or Pro subscription.")
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
