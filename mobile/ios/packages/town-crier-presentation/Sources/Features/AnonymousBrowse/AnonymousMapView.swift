import MapKit
import SwiftUI
import TownCrierDomain

/// The Map tab of the anonymous (pre-signup) tab shell (GH#868 Phase 3;
/// promoted from the sole anonymous screen to a tab in GH#879 Phase 3).
/// Centred on the active device-local zone's coordinate, fetches pins via
/// ``AnonymousMapViewModel``, and clusters them on-device (GH#868 Phase 3
/// refinement). The persistent ``AccountCTABanner`` is hosted once by
/// `AnonymousMainTabView` above the tab bar (GH#879 Phase 3) rather than
/// here, so it appears over every tab, not just this one. A pin tap shows a
/// reduced-feature summary preview; a cluster tap zooms in, unless its
/// members are coincident (same address), in which case it opens a
/// ``StackedApplicationsSheet`` disambiguation list instead (GH#877);
/// anything deeper than the summary preview presents the full detail screen
/// (GH#879 Phase 2).
///
/// GH#912 Phase 4 ("honest anon map"): the radius slider that used to float
/// over the bottom safe area is gone — the monitoring radius is now chosen
/// once, on the postcode-entry screen (``AnonymousPostcodeEntryView``), and
/// afterwards only editable via `DeviceLocalZoneEditorView` on the Zones
/// tab. The map is now a pure viewer: its drawn circle always exactly
/// matches the actual `near-point` fetch boundary (``AnonymousMapViewModel/radiusMetres``),
/// and panning never changes it.
///
/// On iOS, pins render via ``AnonymousClusteredMapView`` (`MKMapView` +
/// MapKit's built-in client-side clustering — near-point returns at most 200
/// points, so on-device clustering is correct with no new backend endpoint).
/// On macOS (the SPM compile target for `swift build`/`swift test`, which has
/// no UIKit), a SwiftUI `Map` fallback plots pins directly with no clustering
/// — mirrors the authenticated `MapView`'s dual-path pattern.
public struct AnonymousMapView: View {
  @StateObject private var viewModel: AnonymousMapViewModel
  #if !canImport(UIKit)
    @State private var cameraPosition: MapCameraPosition
  #endif

  public init(viewModel: AnonymousMapViewModel) {
    _viewModel = StateObject(wrappedValue: viewModel)
    #if !canImport(UIKit)
      _cameraPosition = State(
        initialValue: .region(
          MKCoordinateRegion(
            center: CLLocationCoordinate2D(
              latitude: viewModel.anchorCoordinate.latitude,
              longitude: viewModel.anchorCoordinate.longitude),
            latitudinalMeters: viewModel.radiusMetres * 2.5,
            longitudinalMeters: viewModel.radiusMetres * 2.5
          )))
    #endif
  }

  public var body: some View {
    mapBody
      .background(Color.tcBackground)
      .task { await viewModel.loadInitial() }
      #if !canImport(UIKit)
        .onChange(of: viewModel.radiusMetres) { _, _ in
          withAnimation {
            updateCameraPosition()
          }
        }
      #endif
      .sheet(
        item: Binding(
          get: { viewModel.selectedApplication },
          set: { _ in viewModel.clearSelection() }
        ),
        onDismiss: { viewModel.presentPendingDetailIfNeeded() },
        content: { application in
          AnonymousApplicationSummaryView(application: application) {
            viewModel.requestFullDetail()
          }
        }
      )
      // Disambiguation list for a coincident ("stacked") cluster tap (GH#877).
      // Its `onDismiss` presents the chosen row's summary, so the list always
      // finishes dismissing before the summary appears — the two `.sheet`s are
      // never both up (mirrors `MapView.swift`).
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

  // MARK: - Map body

  private var mapBody: some View {
    ZStack {
      #if canImport(UIKit)
        // Full-bleed (tc-3b1hj): the caller (`AnonymousMainTabView`) hides
        // the nav bar on this screen, so the map extends to every physical
        // edge, including under the status bar/notch at the top.
        AnonymousClusteredMapView(viewModel: viewModel)
          .ignoresSafeArea()
      #else
        macOSMapContent
      #endif
      if viewModel.isLoading && viewModel.clusters.isEmpty {
        ProgressView()
      }
      if let error = viewModel.error, viewModel.clusters.isEmpty {
        ErrorStateView(error: error) {
          await viewModel.loadInitial()
        }
        .background(Color.tcBackground)
      }
    }
  }

  // MARK: - macOS SwiftUI Map fallback (not compiled on iOS)

  #if !canImport(UIKit)
    @ViewBuilder
    private var macOSMapContent: some View {
      Map(position: $cameraPosition) {
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
                // cell opens the disambiguation list; everything else
                // point-reads its single member by slug. Splittable bubbles
                // have no zoom-in affordance in this macOS fallback, so they
                // no-op as before (mirrors `MapView.mapContent`).
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
            latitude: viewModel.anchorCoordinate.latitude,
            longitude: viewModel.anchorCoordinate.longitude
          ),
          radius: viewModel.radiusMetres
        )
        .foregroundStyle(Color.tcAmber.opacity(0.08))
        .stroke(Color.tcAmber.opacity(0.3), lineWidth: 1.5)
      }
      .mapStyle(.standard(elevation: .flat))
    }

    private static func annotationLabel(for cluster: AnonymousMapCluster) -> String {
      cluster.count > 1
        ? "\(cluster.count) applications"
        : (cluster.memberStatus ?? .unknown).displayLabel
    }

    @ViewBuilder
    private func pinView(for cluster: AnonymousMapCluster) -> some View {
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

    private func updateCameraPosition() {
      cameraPosition = .region(
        MKCoordinateRegion(
          center: CLLocationCoordinate2D(
            latitude: viewModel.anchorCoordinate.latitude,
            longitude: viewModel.anchorCoordinate.longitude
          ),
          latitudinalMeters: viewModel.radiusMetres * 2.5,
          longitudinalMeters: viewModel.radiusMetres * 2.5
        )
      )
    }
  #endif
}
