import MapKit
import SwiftUI
import TownCrierDomain

/// The anonymous (pre-signup) map (GH#868 Phase 3) — the only screen in
/// anonymous mode: no tab bar, no list, no settings, no clustering. Centred on
/// the stored coordinate, fetches pins via ``AnonymousMapViewModel``, and pins
/// a persistent ``AccountCTABanner`` above the bottom safe area. A pin tap
/// shows a reduced-feature summary preview; anything deeper routes to
/// sign-up.
///
/// A self-contained SwiftUI `Map` rather than the authenticated `MapView`'s
/// `ClusteredMapView` (`MKMapView` + clustering delegate): the anonymous
/// surface is deliberately a reduced feature set, so a smaller parallel view
/// is lower risk than widening the authenticated map's machinery behind a new
/// shared protocol. See the Phase 3 handoff notes for the full rationale.
public struct AnonymousMapView: View {
  @StateObject private var viewModel: AnonymousMapViewModel
  @State private var cameraPosition: MapCameraPosition

  public init(viewModel: AnonymousMapViewModel) {
    _viewModel = StateObject(wrappedValue: viewModel)
    _cameraPosition = State(
      initialValue: .region(
        MKCoordinateRegion(
          center: CLLocationCoordinate2D(
            latitude: viewModel.centreLat, longitude: viewModel.centreLon),
          latitudinalMeters: viewModel.radiusMetres * 2.5,
          longitudinalMeters: viewModel.radiusMetres * 2.5
        )))
  }

  public var body: some View {
    ZStack(alignment: .bottom) {
      mapBody
      AccountCTABanner(
        onCreateAccount: { viewModel.requestSignUp() },
        onSignIn: { viewModel.requestSignUp() }
      )
    }
    .background(Color.tcBackground)
    .task { await viewModel.loadInitial() }
    .sheet(
      item: Binding(
        get: { viewModel.selectedApplication },
        set: { _ in viewModel.clearSelection() }
      )
    ) { application in
      AnonymousApplicationSummaryView(
        application: application, onSignUp: { viewModel.requestSignUp() })
    }
  }

  private var mapBody: some View {
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
    .overlay {
      if viewModel.isLoading && viewModel.applications.isEmpty {
        ProgressView()
      }
    }
    .overlay {
      if let error = viewModel.error, viewModel.applications.isEmpty {
        ErrorStateView(error: error) {
          await viewModel.loadInitial()
        }
        .background(Color.tcBackground)
      }
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
}
