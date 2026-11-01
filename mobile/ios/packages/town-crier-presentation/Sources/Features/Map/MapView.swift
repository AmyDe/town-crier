import MapKit
import SwiftUI
import TownCrierDomain

/// Map view displaying planning application pins colour-coded by status.
///
/// The screen is full-bleed with no nav bar; Settings is a floating button
/// wired via `onSettingsTapped`.
public struct MapView: View {
  @StateObject private var viewModel: MapViewModel
  private let onSettingsTapped: () -> Void
  // mapPosition and updateMapPosition are only needed on macOS, where
  // ClusteredMapView (UIViewRepresentable) is unavailable and the SwiftUI
  // Map(position:) fallback handles camera framing.
  #if !canImport(UIKit)
    @State private var mapPosition: MapCameraPosition = .automatic
  #endif

  public init(viewModel: MapViewModel, onSettingsTapped: @escaping () -> Void) {
    _viewModel = StateObject(wrappedValue: viewModel)
    self.onSettingsTapped = onSettingsTapped
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
    ZStack(alignment: .topTrailing) {
      mapBody
      floatingSettingsButton
    }
    .safeAreaInset(edge: .top, spacing: 0) {
      headerSection
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
    // Disambiguation list for an unsplittable (stacked) cluster (GH#722). Its
    // `onDismiss` presents the chosen row's summary, so the list always finishes
    // dismissing before the summary appears — the two `.sheet`s are never both up.
    .sheet(
      item: Binding(
        get: { viewModel.stackedApplications },
        set: { _ in viewModel.clearStack() }
      ),
      onDismiss: { viewModel.presentPendingSummaryIfNeeded() },
      content: { stacked in
        StackedApplicationsSheet(stacked: stacked, onSelect: viewModel.selectFromStack)
      }
    )
  }

  // MARK: - Header (zone picker + status filters)

  /// Donated as a `safeAreaInset` rather than a leading `VStack` sibling so
  /// the map layer below keeps its full-bleed frame — edge to edge, behind
  /// this header — instead of being squeezed down to whatever height
  /// remains (tc-3b1hj: "the map gains real height"). A `VStack` with two
  /// `if`-gated children that are both absent collapses to zero size, so an
  /// empty header reserves no dead space either.
  @ViewBuilder
  private var headerSection: some View {
    VStack(spacing: 0) {
      if viewModel.showZonePicker {
        zonePickerSection
      }
      if viewModel.showsLargeZoneNotice {
        LargeZoneNoticeView()
          .padding(.horizontal, TCSpacing.medium)
          .padding(.vertical, TCSpacing.extraSmall)
          .frame(maxWidth: .infinity, alignment: .leading)
          .background(Color.tcBackground)
      }
      if viewModel.showStatusFilters {
        filterHeader
      }
    }
  }

  // MARK: - Floating Settings Button

  /// Replaces the nav-bar gear (`.settingsToolbar`) now the nav bar is
  /// hidden on this screen (tc-3b1hj). A `ZStack` sibling of the (safe-
  /// area-ignoring) map, rather than an `.overlay` on top of it, so it
  /// keeps the ordinary safe-area layout the map opts out of — it lands
  /// below the status bar/notch, and below `headerSection` when that's
  /// showing, with no manual geometry math. `.ultraThinMaterial` mirrors
  /// the look of MapKit's own floating HUD controls (e.g. the compass).
  private var floatingSettingsButton: some View {
    Button(action: onSettingsTapped) {
      Image(systemName: "gearshape")
        .font(TCTypography.headline)
        .foregroundStyle(Color.tcTextPrimary)
        .frame(width: 44, height: 44)
        .background(.ultraThinMaterial, in: Circle())
        .shadow(color: .black.opacity(0.15), radius: 8, x: 0, y: 2)
    }
    .accessibilityLabel("Settings")
    .padding(.top, TCSpacing.small)
    .padding(.trailing, TCSpacing.medium)
  }

  // MARK: - Zone Picker

  private var zonePickerSection: some View {
    ScrollView(.horizontal, showsIndicators: false) {
      HStack(spacing: TCSpacing.small) {
        ForEach(viewModel.zones) { zone in
          CapsuleChipView(
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
    CapsuleChipView(label: label, isSelected: viewModel.selectedStatusFilter == status) {
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
      } else if viewModel.showMap {
        // macOS (SPM compile target) falls back to SwiftUI Map so the package
        // compiles for swift test without UIKit.
        //
        // Stays mounted even with zero clusters: an empty-state takeover would
        // leave the user no way to zoom back out.
        #if canImport(UIKit)
          // Full-bleed: `headerSection` and `floatingSettingsButton` still
          // land in the ordinary safe area via `.safeAreaInset`.
          ClusteredMapView(viewModel: viewModel)
            .ignoresSafeArea()
        #else
          mapContent
        #endif
        if viewModel.isLoading {
          loadingIndicator
        }
      }
    }
  }

  private var mapPlaceholder: some View {
    ZStack {
      Color.tcBackground.ignoresSafeArea()
      MapSkeletonView()
      loadingIndicator
    }
  }

  private var loadingIndicator: some View {
    VStack(spacing: TCSpacing.small) {
      ProgressView()
        .controlSize(.large)
      if viewModel.showsLargeZoneLoadingMessage {
        LargeZoneLoadingMessageView()
      }
    }
    .padding(.horizontal, TCSpacing.extraLarge)
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
                // Mirror the UIKit didSelect routing: a stacked (unsplittable)
                // cell opens the disambiguation list; everything else point-reads
                // its single member (GH#722). Splittable bubbles have no zoom-in
                // affordance in this macOS fallback, so they no-op as before.
                if cluster.isStacked {
                  Task { await viewModel.selectStack(cluster) }
                } else {
                  Task { await viewModel.selectCluster(cluster) }
                }
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
          .font(TCTypography.captionEmphasis)
          .foregroundStyle(Color.tcTextOnAccent)
          .padding(TCSpacing.small)
          .background(Circle().fill(Color.tcAmber))
          .accessibilityLabel("\(cluster.count) applications")
      } else {
        let status = cluster.memberStatus ?? .unknown
        Image(systemName: "mappin.circle.fill")
          .font(TCTypography.displaySmall)
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
