import MapKit
import SwiftUI
import TownCrierDomain

/// Map view displaying planning application pins colour-coded by status.
public struct MapView: View {
  @StateObject private var viewModel: MapViewModel
  // mapPosition and updateMapPosition are only needed on macOS, where
  // ClusteredMapView (UIViewRepresentable) is unavailable and the SwiftUI
  // Map(position:) fallback handles camera framing.
  #if !canImport(UIKit)
    @State private var mapPosition: MapCameraPosition = .automatic
  #endif

  public init(viewModel: MapViewModel) {
    _viewModel = StateObject(wrappedValue: viewModel)
  }

  public var body: some View {
    // On iOS, ClusteredMapView owns the camera — no onChange needed.
    // On macOS (SPM compile-time target), fall back to SwiftUI Map + updateMapPosition.
    #if canImport(UIKit)
      contentStack
    #else
      contentStack
        .onChange(of: viewModel.selectedZone?.id) { _, _ in
          withAnimation {
            updateMapPosition()
          }
        }
    #endif
  }

  // MARK: - Shared content

  private var contentStack: some View {
    VStack(spacing: 0) {
      if viewModel.showZonePicker {
        zonePickerSection
      }
      if viewModel.showStatusFilters {
        filterHeader
      }
      mapBody
    }
    .background(Color.tcBackground)
    .task {
      await viewModel.loadApplications()
      #if !canImport(UIKit)
        updateMapPosition()
      #endif
    }
    .sheet(
      item: Binding(
        get: { viewModel.selectedApplication },
        set: { _ in viewModel.clearSelection() }
      ),
      onDismiss: { viewModel.presentPendingDetailIfNeeded() },
      content: { application in
        ApplicationSummarySheet(application: application, viewModel: viewModel)
      }
    )
  }

  // MARK: - Zone Picker

  private var zonePickerSection: some View {
    ScrollView(.horizontal, showsIndicators: false) {
      HStack(spacing: TCSpacing.small) {
        ForEach(viewModel.zones) { zone in
          ZoneChipView(
            label: zone.name,
            isSelected: zone.id == viewModel.selectedZone?.id
          ) {
            Task {
              await viewModel.selectZone(zone)
            }
          }
        }
      }
      .padding(.horizontal, TCSpacing.medium)
      .padding(.vertical, TCSpacing.small)
    }
    .background(Color.tcBackground)
  }

  // MARK: - Filter Section

  private var filterHeader: some View {
    ScrollView(.horizontal, showsIndicators: false) {
      HStack(spacing: TCSpacing.small) {
        filterChip(label: "All", status: nil)
        filterChip(label: "Pending", status: .undecided)
        filterChip(label: "Granted", status: .permitted)
        filterChip(label: "Granted with conditions", status: .conditions)
        filterChip(label: "Refused", status: .rejected)
        filterChip(label: "Withdrawn", status: .withdrawn)
        filterChip(label: "Appealed", status: .appealed)
      }
      .padding(.horizontal, TCSpacing.medium)
      .padding(.vertical, TCSpacing.small)
    }
    .background(Color.tcBackground)
  }

  private func filterChip(label: String, status: ApplicationStatus?) -> some View {
    FilterChipView(label: label, isSelected: viewModel.selectedStatusFilter == status) {
      Task { await viewModel.applyStatusFilter(status) }
    }
  }

  // MARK: - Map Body

  private var mapBody: some View {
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
          description:
            "No planning applications found in your watch zone yet. Check back soon."
        )
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .background(Color.tcBackground)
      } else {
        // iOS: MKMapView with native clustering (GH#542, GH#682 slice 5).
        // macOS (SPM compile target): fall back to SwiftUI Map so the package
        // still compiles for swift test without UIKit.
        #if canImport(UIKit)
          ClusteredMapView(viewModel: viewModel)
            .ignoresSafeArea(edges: .bottom)
        #else
          mapContent
        #endif
        if viewModel.isLoading {
          ProgressView()
            .controlSize(.large)
        }
      }
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

  // MARK: - macOS SwiftUI Map fallback (not compiled on iOS)

  #if !canImport(UIKit)
    @ViewBuilder
    private var mapContent: some View {
      Map(position: $mapPosition) {
        ForEach(viewModel.clusters) { cluster in
          Annotation(
            Self.annotationLabel(for: cluster),
            coordinate: CLLocationCoordinate2D(
              latitude: cluster.coordinate.latitude,
              longitude: cluster.coordinate.longitude
            ),
            anchor: .bottom
          ) {
            pinView(for: cluster)
              .onTapGesture {
                Task { await viewModel.selectCluster(cluster) }
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

    private static func annotationLabel(for cluster: MapCluster) -> String {
      cluster.count > 1
        ? "\(cluster.count) applications"
        : (cluster.memberStatus ?? .unknown).displayLabel
    }

    @ViewBuilder
    private func pinView(for cluster: MapCluster) -> some View {
      if cluster.count > 1 {
        Text(cluster.count > 999 ? "999+" : "\(cluster.count)")
          .font(.system(.caption, weight: .semibold))
          .foregroundStyle(Color.tcTextOnAccent)
          .padding(TCSpacing.small)
          .background(Circle().fill(Color.tcAmber))
          .accessibilityLabel("\(cluster.count) applications")
      } else {
        let status = cluster.memberStatus ?? .unknown
        Image(systemName: "mappin.circle.fill")
          .font(.system(.title2))
          .foregroundStyle(status.displayColor)
          .background(
            Circle()
              .fill(Color.tcSurface)
              .frame(width: 20, height: 20)
          )
          .accessibilityLabel(status.displayLabel)
      }
    }

    private func updateMapPosition() {
      mapPosition = .region(
        MKCoordinateRegion(
          center: CLLocationCoordinate2D(
            latitude: viewModel.centreLat,
            longitude: viewModel.centreLon
          ),
          latitudinalMeters: viewModel.radiusMetres * 2.5,
          longitudinalMeters: viewModel.radiusMetres * 2.5
        )
      )
    }
  #endif
}
