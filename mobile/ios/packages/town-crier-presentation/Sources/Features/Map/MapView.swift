import MapKit
import SwiftUI
import TownCrierDomain

/// Map view displaying planning application pins colour-coded by status.
public struct MapView: View {
    @StateObject private var viewModel: MapViewModel

    public init(viewModel: MapViewModel) {
        _viewModel = StateObject(wrappedValue: viewModel)
    }

    public var body: some View {
        ZStack {
            if viewModel.isLoading && !viewModel.hasLoaded {
                mapPlaceholder
            } else if let error = viewModel.error {
                ErrorStateView(error: error) {
                    await viewModel.loadApplications()
                }
                .frame(maxWidth: .infinity, maxHeight: .infinity)
                .background(Color.tcBackground)
            } else if viewModel.isEmpty {
                EmptyStateView(
                    icon: "map",
                    title: "No Applications",
                    description: "No planning applications found in your watch zone yet. Check back soon."
                )
                .frame(maxWidth: .infinity, maxHeight: .infinity)
                .background(Color.tcBackground)
            } else {
                mapContent
                if viewModel.isLoading {
                    ProgressView()
                        .controlSize(.large)
                }
            }
        }
        .background(Color.tcBackground)
        .task {
            await viewModel.loadApplications()
        }
        .sheet(
            item: Binding(
                get: { viewModel.selectedApplication },
                set: { _ in viewModel.clearSelection() }
            )
        ) { application in
            ApplicationSummarySheet(application: application)
        }
    }

    @ViewBuilder
    private var mapContent: some View {
        Map(
            initialPosition: .region(
                MKCoordinateRegion(
                    center: CLLocationCoordinate2D(
                        latitude: viewModel.centreLat,
                        longitude: viewModel.centreLon
                    ),
                    latitudinalMeters: viewModel.radiusMetres * 2.5,
                    longitudinalMeters: viewModel.radiusMetres * 2.5
                )
            )
        ) {
            ForEach(viewModel.annotations) { annotation in
                Annotation(
                    annotation.title,
                    coordinate: CLLocationCoordinate2D(
                        latitude: annotation.latitude,
                        longitude: annotation.longitude
                    ),
                    anchor: .bottom
                ) {
                    pinView(for: annotation)
                        .onTapGesture {
                            viewModel.selectApplication(annotation.applicationId)
                        }
                }
            }

            MapCircle(
                center: CLLocationCoordinate2D(
                    latitude: viewModel.centreLat,
                    longitude: viewModel.centreLon
                ),
                radius: viewModel.radiusMetres
            )
            .foregroundStyle(Color.tcAmber.opacity(0.08))
            .stroke(Color.tcAmber.opacity(0.3), lineWidth: 1.5)
        }
        .mapStyle(.standard(elevation: .flat))
    }

    private func pinView(for annotation: MapAnnotationItem) -> some View {
        Image(systemName: "mappin.circle.fill")
            .font(.system(.title2))
            .foregroundStyle(pinColor(for: annotation.statusColor))
            .background(
                Circle()
                    .fill(Color.tcSurface)
                    .frame(width: 20, height: 20)
            )
            .accessibilityLabel("\(annotation.title), \(statusLabel(for: annotation.statusColor))")
    }

    private func pinColor(for status: StatusColor) -> Color {
        switch status {
        case .pending:
            return .tcStatusPending
        case .approved:
            return .tcStatusApproved
        case .refused:
            return .tcStatusRefused
        case .withdrawn:
            return .tcStatusWithdrawn
        case .appealed:
            return .tcStatusAppealed
        case .unknown:
            return .tcTextTertiary
        }
    }

    private func statusLabel(for status: StatusColor) -> String {
        switch status {
        case .pending:
            return "Pending"
        case .approved:
            return "Approved"
        case .refused:
            return "Refused"
        case .withdrawn:
            return "Withdrawn"
        case .appealed:
            return "Appealed"
        case .unknown:
            return "Unknown"
        }
    }

    private var mapPlaceholder: some View {
        ZStack {
            Color.tcBackground.ignoresSafeArea()
            MapSkeletonView()
            ProgressView()
                .controlSize(.large)
        }
    }
}
