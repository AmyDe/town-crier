import Combine
import TownCrierDomain

/// The minimal surface `BoundaryDrawingMapView` needs from whatever
/// ViewModel is driving a custom-shape boundary drawing session (GH#1031).
///
/// Factored out so the map view can be reused from more than one feature —
/// `WatchZoneEditorViewModel` in the watch-zone editor (tc-6he3x.8) and
/// `OnboardingViewModel` in the onboarding wizard (tc-6he3x.10) — without
/// either feature depending on the other, and without `BoundaryDrawingMapView`
/// being locked to a single concrete ViewModel type.
@MainActor
public protocol BoundaryDrawingViewModel: ObservableObject {
  /// The custom-shape ring being drawn, as an OPEN sequence of vertices in
  /// drop order — NOT closed (the first vertex is not repeated at the end).
  var boundaryVertices: [Coordinate] { get }

  /// Appends a new vertex at the end of the ring being drawn.
  func addVertex(_ coordinate: Coordinate)

  /// Moves the vertex at `index` to `coordinate` (e.g. after a drag). A
  /// stale or out-of-range index is expected to be a no-op rather than a
  /// crash.
  func moveVertex(at index: Int, to coordinate: Coordinate)

  /// Validates the current vertices as a closed ring without saving —
  /// invoked when the user taps the first vertex to close the shape while
  /// drawing.
  func finishDrawing()
}
