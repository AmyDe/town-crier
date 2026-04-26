import MapKit
import SwiftUI
import TownCrierDomain

/// Map view displaying planning application pins colour-coded by status.
public struct MapView: View {
  @StateObject private var viewModel: MapViewModel
  @State private var mapPosition: MapCameraPosition = .automatic

  public init(viewModel: MapViewModel) {
    _viewModel = StateObject(wrappedValue: viewModel)
  }

  public var body: some View {
    VStack(spacing: 0) {
      if viewModel.showZonePicker {
        zonePickerSection
      }
      if viewModel.canFilter || viewModel.canSave {
        filterHeader
      }
      mapBody
    }
    .background(Color.tcBackground)
    .task {
      await viewModel.loadApplications()
      updateMapPosition()
    }
    .onChange(of: viewModel.selectedZone?.id) { _, _ in
      withAnimation {
        updateMapPosition()
      }
    }
    .sheet(
      item: Binding(
        get: { viewModel.selectedApplication },
        set: { _ in viewModel.clearSelection() }
      )
    ) { application in
      ApplicationSummarySheet(application: application, viewModel: viewModel)
    }
  }

  @ViewBuilder
  private var mapContent: some View {
    Map(position: $mapPosition) {
      ForEach(viewModel.filteredAnnotations) { annotation in
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
      .foregroundStyle(annotation.status.displayColor)
      .background(
        Circle()
          .fill(Color.tcSurface)
          .frame(width: 20, height: 20)
      )
      .accessibilityLabel("\(annotation.title), \(annotation.status.displayLabel)")
  }

  // MARK: - Zone Picker

  private var zonePickerSection: some View {
    ScrollView(.horizontal, showsIndicators: false) {
      HStack(spacing: TCSpacing.small) {
        ForEach(viewModel.zones) { zone in
          zoneChip(zone: zone, isSelected: zone.id == viewModel.selectedZone?.id)
        }
      }
      .padding(.horizontal, TCSpacing.medium)
      .padding(.vertical, TCSpacing.small)
    }
    .background(Color.tcBackground)
  }

  private func zoneChip(zone: WatchZone, isSelected: Bool) -> some View {
    Text(zone.name)
      .font(TCTypography.captionEmphasis)
      .foregroundStyle(isSelected ? Color.tcTextOnAccent : Color.tcTextPrimary)
      .padding(.horizontal, TCSpacing.small)
      .padding(.vertical, TCSpacing.extraSmall)
      .background(isSelected ? Color.tcAmber : Color.tcSurface)
      .clipShape(Capsule())
      .overlay(
        Capsule()
          .stroke(Color.tcBorder, lineWidth: isSelected ? 0 : 1)
      )
      .contentShape(Capsule())
      .onTapGesture {
        Task {
          await viewModel.selectZone(zone)
        }
      }
  }

  // MARK: - Filter Section

  private var filterHeader: some View {
    ScrollView(.horizontal, showsIndicators: false) {
      HStack(spacing: TCSpacing.small) {
        if viewModel.canSave {
          savedFilterChip
        }
        if viewModel.canFilter {
          filterChip(label: "All", status: nil)
          filterChip(label: "Pending", status: .undecided)
          filterChip(label: "Granted", status: .permitted)
          filterChip(label: "Granted with conditions", status: .conditions)
          filterChip(label: "Refused", status: .rejected)
          filterChip(label: "Withdrawn", status: .withdrawn)
          filterChip(label: "Appealed", status: .appealed)
        }
      }
      .padding(.horizontal, TCSpacing.medium)
      .padding(.vertical, TCSpacing.small)
    }
    .background(Color.tcBackground)
  }

  private var savedFilterChip: some View {
    Label("Saved", systemImage: "bookmark.fill")
      .font(TCTypography.captionEmphasis)
      .foregroundStyle(
        viewModel.isSavedFilterActive ? Color.tcTextOnAccent : Color.tcTextPrimary
      )
      .padding(.horizontal, TCSpacing.small)
      .padding(.vertical, TCSpacing.extraSmall)
      .background(viewModel.isSavedFilterActive ? Color.tcAmber : Color.tcSurface)
      .clipShape(Capsule())
      .overlay(
        Capsule()
          .stroke(Color.tcBorder, lineWidth: viewModel.isSavedFilterActive ? 0 : 1)
      )
      .contentShape(Capsule())
      .onTapGesture {
        if viewModel.isSavedFilterActive {
          viewModel.deactivateSavedFilter()
        } else {
          Task { await viewModel.activateSavedFilter() }
        }
      }
  }

  private func filterChip(label: String, status: ApplicationStatus?) -> some View {
    let isSelected = viewModel.selectedStatusFilter == status
      && !viewModel.isSavedFilterActive
    return Text(label)
      .font(TCTypography.captionEmphasis)
      .foregroundStyle(isSelected ? Color.tcTextOnAccent : Color.tcTextPrimary)
      .padding(.horizontal, TCSpacing.small)
      .padding(.vertical, TCSpacing.extraSmall)
      .background(isSelected ? Color.tcAmber : Color.tcSurface)
      .clipShape(Capsule())
      .overlay(
        Capsule()
          .stroke(Color.tcBorder, lineWidth: isSelected ? 0 : 1)
      )
      .contentShape(Capsule())
      .onTapGesture {
        viewModel.selectedStatusFilter = status
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
        if viewModel.isSavedFilterActive {
          EmptyStateView(
            icon: "bookmark",
            title: "No Saved Applications",
            description:
              "No saved applications. Tap the bookmark icon on any application to save it."
          )
          .frame(maxWidth: .infinity, maxHeight: .infinity)
          .background(Color.tcBackground)
        } else {
          EmptyStateView(
            icon: "map",
            title: "No Applications",
            description:
              "No planning applications found in your watch zone yet. Check back soon."
          )
          .frame(maxWidth: .infinity, maxHeight: .infinity)
          .background(Color.tcBackground)
        }
      } else {
        mapContent
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
}
