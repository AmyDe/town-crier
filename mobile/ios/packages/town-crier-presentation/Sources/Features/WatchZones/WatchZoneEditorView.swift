import MapKit
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
                    .disabled(viewModel.geocodedCoordinate == nil || viewModel.isLoading)
                }
            }
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
                ZoneMapPreviewLarge(
                    centre: coordinate,
                    radiusMetres: viewModel.selectedRadiusMetres
                )
                .frame(height: 220)
                .clipShape(RoundedRectangle(cornerRadius: TCCornerRadius.medium))
                .listRowInsets(EdgeInsets(
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

/// Larger map preview for the editor screen.
private struct ZoneMapPreviewLarge: View {
    let centre: Coordinate
    let radiusMetres: Double

    var body: some View {
        Map(initialPosition: .region(region)) {
            MapCircle(center: clLocation, radius: radiusMetres)
                .foregroundStyle(Color.tcAmber.opacity(0.2))
                .stroke(Color.tcAmber, lineWidth: 2)
        }
        .mapStyle(.standard(elevation: .flat))
        .allowsHitTesting(false)
    }

    private var clLocation: CLLocationCoordinate2D {
        CLLocationCoordinate2D(latitude: centre.latitude, longitude: centre.longitude)
    }

    private var region: MKCoordinateRegion {
        MKCoordinateRegion(
            center: clLocation,
            latitudinalMeters: radiusMetres * 2.5,
            longitudinalMeters: radiusMetres * 2.5
        )
    }
}
