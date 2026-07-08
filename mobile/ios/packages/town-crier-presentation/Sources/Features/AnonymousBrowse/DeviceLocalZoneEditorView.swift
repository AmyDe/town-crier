import SwiftUI
import TownCrierDomain

/// Create or edit a device-local zone: name → postcode entry → radius picker
/// → map preview → save (GH#879 Phase 4).
///
/// Visual conventions mirror the authed `WatchZoneEditorView`, but this is a
/// separate, simpler anonymous editor — no entitlement gate, no per-zone
/// notification toggles (any alert affordance lives on the zone row, not
/// here).
public struct DeviceLocalZoneEditorView: View {
  @StateObject private var viewModel: DeviceLocalZoneEditorViewModel
  @Environment(\.dismiss) private var dismiss

  public init(viewModel: DeviceLocalZoneEditorViewModel) {
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
        if let error = viewModel.error {
          errorSection(error)
        }
      }
      .navigationTitle(viewModel.isEditing ? "Edit Area" : "New Area")
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
              // Dismiss only on success. On a cap breach the list dismisses
              // the sheet itself (via onRequestSignUp) and shows the sign-up
              // CTA; on any other failure the editor stays open and shows
              // the inline error.
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
      TextField("e.g. Home, Allotment", text: $viewModel.nameInput)
        .autocorrectionDisabled()
    } header: {
      Text("Area Name")
    } footer: {
      Text("A friendly name to identify this area.")
        .font(.system(.caption))
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
      Text("Enter a UK postcode to centre this area.")
        .font(.system(.caption))
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
          in: viewModel.minRadiusMetres...viewModel.maxRadiusMetres,
          step: 100
        )
        .tint(Color.tcAmber)
        .accessibilityLabel("Radius")
        .accessibilityValue(formatRadius(viewModel.selectedRadiusMetres))

        HStack {
          Text(formatRadius(viewModel.minRadiusMetres))
          Spacer()
          Text(formatRadius(viewModel.maxRadiusMetres))
        }
        .font(TCTypography.caption)
        .foregroundStyle(Color.tcTextSecondary)
      }
    }
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
          .font(.system(.body))
          .foregroundStyle(Color.tcStatusRejected)
      } icon: {
        Image(systemName: "exclamationmark.triangle.fill")
          .foregroundStyle(Color.tcStatusRejected)
      }
    }
  }
}
