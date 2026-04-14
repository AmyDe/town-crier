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
        postcodeSection
        if viewModel.geocodedCoordinate != nil {
          radiusSection
          mapPreviewSection
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
      Picker("Radius", selection: $viewModel.selectedRadiusMetres) {
        ForEach(viewModel.availableRadiusOptions, id: \.self) { option in
          Text(formatRadius(option)).tag(option)
        }
      }
      .pickerStyle(.segmented)
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
          .foregroundStyle(Color.tcStatusRefused)
      } icon: {
        Image(systemName: "exclamationmark.triangle.fill")
          .foregroundStyle(Color.tcStatusRefused)
      }
    }
  }

}
