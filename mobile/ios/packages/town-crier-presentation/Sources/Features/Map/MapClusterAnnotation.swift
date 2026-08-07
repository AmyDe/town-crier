#if canImport(UIKit)
  import MapKit
  import TownCrierDomain

  /// A reference-type `MKAnnotation` wrapping a value-type ``MapCluster`` so
  /// MapKit can hold it. Carries the cluster ``ClusteredMapView/Coordinator``
  /// needs to style the marker and route a tap.
  final class MapClusterAnnotation: NSObject, MKAnnotation {
    let cluster: MapCluster
    let clusterId: String
    let coordinate: CLLocationCoordinate2D

    init(cluster: MapCluster) {
      self.cluster = cluster
      self.clusterId = cluster.id
      self.coordinate = CLLocationCoordinate2D(
        latitude: cluster.coordinate.latitude, longitude: cluster.coordinate.longitude)
    }
  }
#endif
