import SwiftUI
import TownCrierDomain

/// Create or edit a watch zone: postcode entry → radius picker → map preview → save.
public struct WatchZoneEditorView: View {
  @StateObject private var viewModel: WatchZoneEditorViewModel
  @Environment(\.dismiss) private var dismiss

  public init(viewModel: WatchZoneEditorViewModel) {
    _viewModel = StateObject(wrappedValue: viewModel)
  }

  public var body: some View {
    NavigationStack {
      Form {
        nameSection
        if viewModel.isPostcodeFieldVisible {
          postcodeSection
        }
        if viewModel.geocodedCoordinate != nil {
          radiusSection
          mapPreviewSection
        }
        if viewModel.areNotificationTogglesVisible {
          notificationsSection
        }
        if let error = viewModel.error {
          errorSection(error)
        }
      }
      .navigationTitle(viewModel.isEditing ? "Edit Watch Zone" : "New Watch Zone")
      #if os(iOS)
        .navigationBarTitleDisplayMode(.inline)
      #endif
      .entitlementGateSheet(entitlement: $viewModel.entitlementGate) {
        viewModel.viewPlans()
      }
      .toolbar {
        ToolbarItem(placement: .cancellationAction) {
          Button("Cancel") { dismiss() }
        }
        ToolbarItem(placement: .confirmationAction) {
          Button("Save") {
            Task {
              // Dismiss only on success. On a quota breach the coordinator
              // closes the sheet and opens the paywall (tc-gpjk); on any other
              // failure the editor stays open and shows the inline error.
              if await viewModel.save() {
                dismiss()
              }
            }
          }
          .disabled(
            viewModel.geocodedCoordinate == nil || viewModel.isLoading
              || viewModel.nameInput.trimmingCharacters(in: .whitespaces).isEmpty
          )
        }
      }
    }
  }

  private var nameSection: some View {
    Section {
      TextField("e.g. My Home, Office", text: $viewModel.nameInput)
        .autocorrectionDisabled()
    } header: {
      Text("Zone Name")
    } footer: {
      Text("A friendly name to identify this watch zone.")
        .font(TCTypography.caption)
        .foregroundStyle(Color.tcTextSecondary)
    }
  }

  private var postcodeSection: some View {
    Section {
      HStack {
        TextField("Postcode", text: $viewModel.postcodeInput)
          .textContentType(.postalCode)
          .autocorrectionDisabled()
          #if os(iOS)
            .textInputAutocapitalization(.characters)
          #endif

        if viewModel.isLoading {
          ProgressView()
        } else {
          Button("Look up") {
            Task { await viewModel.submitPostcode() }
          }
          .disabled(viewModel.postcodeInput.isEmpty)
        }
      }
    } header: {
      Text("Postcode")
    } footer: {
      Text("Enter a UK postcode to centre your watch zone.")
        .font(TCTypography.caption)
        .foregroundStyle(Color.tcTextSecondary)
    }
  }

  private var radiusSection: some View {
    Section("Radius") {
      VStack(alignment: .leading, spacing: TCSpacing.small) {
        Text(formatRadius(viewModel.selectedRadiusMetres))
          .font(TCTypography.bodyEmphasis)
          .foregroundStyle(Color.tcTextPrimary)

        Slider(
          value: $viewModel.selectedRadiusMetres,
          in: 100...viewModel.maxRadiusMetres,
          step: 100
        )
        .tint(Color.tcAmber)
        .accessibilityLabel("Radius")
        .accessibilityValue(formatRadius(viewModel.selectedRadiusMetres))

        HStack {
          Text(formatRadius(100))
          Spacer()
          Text(formatRadius(viewModel.maxRadiusMetres))
        }
        .font(TCTypography.caption)
        .foregroundStyle(Color.tcTextSecondary)

        if viewModel.canUnlockLargerRadius {
          // Radius upsell (tc-w3cb.3). Routes through viewPlans()/onUpgradeRequired
          // like the instant-alert upsell: the editor closes and the paywall
          // opens. After purchase the list rebuilds on the new tier (.id), so a
          // reopened editor offers the larger range.
          UnlockLargerZonesChip {
            viewModel.viewPlans()
          }
          .padding(.top, TCSpacing.small)
        }

        if viewModel.showsLargeRadiusWarning {
          LargeRadiusWarningView()
            .padding(.top, TCSpacing.small)
        }
      }
    }
  }

  private var notificationsSection: some View {
    Section {
      GatedToggle(
        label: "Send push notifications",
        isOn: $viewModel.pushEnabled,
        entitlement: viewModel.instantAlertEntitlement,
        featureGate: viewModel.featureGate
      ) {
        viewModel.requestInstantAlertUpgrade()
      }
      GatedToggle(
        label: "Send instant emails",
        isOn: $viewModel.emailInstantEnabled,
        entitlement: viewModel.instantAlertEntitlement,
        featureGate: viewModel.featureGate
      ) {
        viewModel.requestInstantAlertUpgrade()
      }
    } header: {
      Text("Notifications")
    } footer: {
      Text(notificationsFooterText)
        .font(TCTypography.caption)
        .foregroundStyle(Color.tcTextSecondary)
    }
  }

  private var notificationsFooterText: String {
    if viewModel.featureGate.hasEntitlement(viewModel.instantAlertEntitlement) {
      return "Choose how this zone alerts you when new applications match."
    }
    return "Instant push and email alerts are available on Personal and Pro. "
      + "Free accounts receive a weekly email digest."
  }

  @ViewBuilder
  private var mapPreviewSection: some View {
    if let coordinate = viewModel.geocodedCoordinate {
      Section("Preview") {
        ZoneMapPreview(
          centre: coordinate,
          radiusMetres: viewModel.selectedRadiusMetres,
          strokeWidth: 2
        )
        .frame(height: 220)
        .clipShape(RoundedRectangle(cornerRadius: TCCornerRadius.medium))
        .listRowInsets(
          EdgeInsets(
            top: TCSpacing.small,
            leading: TCSpacing.medium,
            bottom: TCSpacing.small,
            trailing: TCSpacing.medium
          ))
      }
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

}
