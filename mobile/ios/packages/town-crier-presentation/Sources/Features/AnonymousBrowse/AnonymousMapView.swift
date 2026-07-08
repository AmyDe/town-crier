import MapKit
import SwiftUI
import TownCrierDomain

/// The anonymous (pre-signup) map (GH#868 Phase 3) — the only screen in
/// anonymous mode: no tab bar, no list, no settings. Centred on the stored
/// coordinate, fetches pins via ``AnonymousMapViewModel``, clusters them
/// on-device (GH#868 Phase 3 refinement), and pins a persistent
/// ``AccountCTABanner`` — with a live monitoring-radius picker above it — over
/// the bottom safe area. A pin tap shows a reduced-feature summary preview; a
/// cluster tap zooms in, unless its members are coincident (same address),
/// in which case it opens a ``StackedApplicationsSheet`` disambiguation list
/// instead (GH#877); anything deeper than the summary preview routes to
/// sign-up.
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
            latitudinalMeters: viewModel.selectedRadiusMetres * 2.5,
            longitudinalMeters: viewModel.selectedRadiusMetres * 2.5
          )))
    #endif
  }

  public var body: some View {
    ZStack(alignment: .bottom) {
      mapBody
      VStack(spacing: TCSpacing.small) {
        radiusPickerCard
        AccountCTABanner(
          onCreateAccount: { viewModel.requestSignUp() },
          onSignIn: { viewModel.requestSignUp() }
        )
      }
    }
    .background(Color.tcBackground)
    .task { await viewModel.loadInitial() }
    #if !canImport(UIKit)
      .onChange(of: viewModel.selectedRadiusMetres) { _, _ in
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

  // MARK: - Radius picker

  /// Live monitoring-radius `Slider`, mirroring `RadiusPickerStepView`'s
  /// paradigm (same `formatRadius` label, `tcAmber` tint) so the anonymous
  /// preview and the post-signup wizard feel like one continuous control.
  private var radiusPickerCard: some View {
    VStack(spacing: TCSpacing.small) {
      Text(formatRadius(viewModel.selectedRadiusMetres))
        .font(TCTypography.bodyEmphasis)
        .foregroundStyle(Color.tcTextPrimary)

      Slider(
        value: Binding(
          get: { viewModel.selectedRadiusMetres },
          set: { viewModel.updateSelectedRadius($0) }
        ),
        in: AnonymousMapViewModel.minSelectedRadiusMetres...AnonymousMapViewModel
          .maxSelectedRadiusMetres,
        step: 100
      )
      .tint(Color.tcAmber)
      .accessibilityLabel("Monitoring radius")
      .accessibilityValue(formatRadius(viewModel.selectedRadiusMetres))
    }
    .padding(TCSpacing.medium)
    .background(Color.tcSurfaceElevated)
    .clipShape(RoundedRectangle(cornerRadius: TCCornerRadius.large))
    .padding(.horizontal, TCSpacing.medium)
  }

  // MARK: - Map body

  private var mapBody: some View {
    ZStack {
      #if canImport(UIKit)
        AnonymousClusteredMapView(viewModel: viewModel)
          .ignoresSafeArea(edges: .bottom)
      #else
        macOSMapContent
      #endif
      if viewModel.isLoading && viewModel.applications.isEmpty {
        ProgressView()
      }
      if let error = viewModel.error, viewModel.applications.isEmpty {
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
        ForEach(viewModel.applications) { application in
          if let location = application.location {
            Annotation(
              application.reference.value,
              coordinate: CLLocationCoordinate2D(
                latitude: location.latitude, longitude: location.longitude),
              anchor: .bottom
            ) {
              pin(for: application)
                .onTapGesture { viewModel.selectApplication(application) }
            }
          }
        }

        MapCircle(
          center: CLLocationCoordinate2D(
            latitude: viewModel.anchorCoordinate.latitude,
            longitude: viewModel.anchorCoordinate.longitude
          ),
          radius: viewModel.selectedRadiusMetres
        )
        .foregroundStyle(Color.tcAmber.opacity(0.08))
        .stroke(Color.tcAmber.opacity(0.3), lineWidth: 1.5)
      }
      .mapStyle(.standard(elevation: .flat))
      .onMapCameraChange(frequency: .onEnd) { context in
        let span = max(context.region.span.latitudeDelta, context.region.span.longitudeDelta)
        viewModel.regionDidChange(
          centreLat: context.region.center.latitude,
          centreLon: context.region.center.longitude,
          // Half the span's on-the-ground metres, matching MapViewModel's own
          // degrees-to-metres approximation for a rough "visible radius".
          radiusMetres: span * 111_320 / 2
        )
      }
    }

    private func pin(for application: PlanningApplication) -> some View {
      Image(systemName: "mappin.circle.fill")
        .font(.system(.title2))
        .foregroundStyle(application.status.displayColor)
        .background(
          Circle()
            .fill(Color.tcSurface)
            .frame(width: 20, height: 20)
        )
        .accessibilityLabel(application.status.displayLabel)
    }

    private func updateCameraPosition() {
      cameraPosition = .region(
        MKCoordinateRegion(
          center: CLLocationCoordinate2D(
            latitude: viewModel.anchorCoordinate.latitude,
            longitude: viewModel.anchorCoordinate.longitude
          ),
          latitudinalMeters: viewModel.selectedRadiusMetres * 2.5,
          longitudinalMeters: viewModel.selectedRadiusMetres * 2.5
        )
      )
    }
  #endif
}
