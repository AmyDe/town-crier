import TownCrierDomain

/// Sends `POST /v1/share-events` through ``AnonymousURLSessionAPIClient``, so the
/// request has no `Authorization` header.
public final class APIShareEventReporter: ShareEventReporter, Sendable {
  private struct Body: Encodable, Sendable {
    let destination: String
  }

  private let apiClient: AnonymousURLSessionAPIClient

  public init(apiClient: AnonymousURLSessionAPIClient) {
    self.apiClient = apiClient
  }

  public func report(_ destination: ShareDestination) async {
    let endpoint = APIEndpoint.post(
      "/v1/share-events", body: Body(destination: destination.rawValue))
    let _: EmptyResponse? = try? await apiClient.request(endpoint)
  }
}
