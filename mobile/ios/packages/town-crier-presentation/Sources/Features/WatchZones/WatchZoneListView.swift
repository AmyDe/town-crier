#if os(iOS)
import MapKit
import SwiftUI
import TownCrierDomain

/// Displays the user's watch zones with add/edit/delete actions.
public struct WatchZoneListView: View {
    @StateObject private var viewModel: WatchZoneListViewModel

    public init(viewModel: WatchZoneListViewModel) {
        _viewModel = StateObject(wrappedValue: viewModel)
    }

    public var body: some View {
        List {
            if viewModel.zones.isEmpty && !viewModel.isLoading {
                emptyState
            } else {
                ForEach(viewModel.zones) { zone in
                    WatchZoneRow(zone: zone)
                        .contentShape(Rectangle())
                        .onTapGesture { viewModel.editZone(zone) }
                }
                .onDelete { indexSet in
                    guard let index = indexSet.first else { return }
                    let zone = viewModel.zones[index]
                    Task { await viewModel.deleteZone(zone) }
                }
            }
        }
        .navigationTitle("Watch Zones")
        .toolbar {
            ToolbarItem(placement: .primaryAction) {
                Button {
                    viewModel.addZone()
                } label: {
                    Image(systemName: "plus")
                }
            }
        }
        .overlay {
            if viewModel.isLoading {
                ProgressView()
            }
        }
        .task {
            await viewModel.load()
        }
    }

    private var emptyState: some View {
        Section {
            VStack(spacing: TCSpacing.medium) {
                Image(systemName: "mappin.and.ellipse")
                    .font(.system(.largeTitle))
                    .foregroundStyle(Color.tcTextTertiary)
                Text("No Watch Zones")
                    .font(.system(.headline).weight(.semibold))
                Text("Add a watch zone to start monitoring planning applications in your area.")
                    .font(.system(.body))
                    .foregroundStyle(Color.tcTextSecondary)
                    .multilineTextAlignment(.center)
                Button {
                    viewModel.addZone()
                } label: {
                    Text("Add Watch Zone")
                        .font(.system(.body).weight(.semibold))
                        .frame(maxWidth: .infinity)
                }
                .buttonStyle(.borderedProminent)
                .tint(Color.tcAmber)
            }
            .padding(.vertical, TCSpacing.extraLarge)
        }
    }
}

private struct WatchZoneRow: View {
    let zone: WatchZone

    var body: some View {
        HStack(spacing: TCSpacing.medium) {
            ZoneMapPreview(centre: zone.centre, radiusMetres: zone.radiusMetres)
                .frame(width: 56, height: 56)
                .clipShape(RoundedRectangle(cornerRadius: TCCornerRadius.small))

            VStack(alignment: .leading, spacing: TCSpacing.extraSmall) {
                Text(zone.postcode.value)
                    .font(.system(.headline).weight(.semibold))
                Text(radiusLabel(zone.radiusMetres))
                    .font(.system(.caption))
                    .foregroundStyle(Color.tcTextSecondary)
            }

            Spacer()

            Image(systemName: "chevron.right")
                .font(.system(.caption))
                .foregroundStyle(Color.tcTextTertiary)
        }
        .padding(.vertical, TCSpacing.extraSmall)
    }

    private func radiusLabel(_ metres: Double) -> String {
        if metres >= 1000 {
            let km = metres / 1000
            return km.truncatingRemainder(dividingBy: 1) == 0
                ? "\(Int(km)) km radius"
                : String(format: "%.1f km radius", km)
        }
        return "\(Int(metres)) m radius"
    }
}

/// A small non-interactive map preview showing the zone circle.
private struct ZoneMapPreview: View {
    let centre: Coordinate
    let radiusMetres: Double

    var body: some View {
        Map(initialPosition: .region(region)) {
            MapCircle(center: clLocation, radius: radiusMetres)
                .foregroundStyle(Color.tcAmber.opacity(0.2))
                .stroke(Color.tcAmber, lineWidth: 1)
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
#endif
