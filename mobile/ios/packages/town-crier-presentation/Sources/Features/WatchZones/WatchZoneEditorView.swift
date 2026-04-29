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
      .toolbar {
        ToolbarItem(placement: .cancellationAction) {
          Button("Cancel") { dismiss() }
        }
        ToolbarItem(placement: .confirmationAction) {
          Button("Save") {
            Task {
              await viewModel.save()
              dismiss()
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
      Text("Enter a UK postcode to centre your watch zone.")
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
      }
    }
  }

  private var notificationsSection: some View {
    Section {
      Toggle("Send push notifications", isOn: $viewModel.pushEnabled)
        .tint(Color.tcAmber)
      Toggle("Send instant emails", isOn: $viewModel.emailInstantEnabled)
        .tint(Color.tcAmber)
    } header: {
      Text("Notifications")
    } footer: {
      Text("Choose how this zone alerts you when new applications match.")
        .font(.system(.caption))
        .foregroundStyle(Color.tcTextSecondary)
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
