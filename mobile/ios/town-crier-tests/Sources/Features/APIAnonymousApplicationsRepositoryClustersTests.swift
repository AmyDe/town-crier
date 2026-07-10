import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierData

/// Covers the anonymous server-side map clustering fetch (GH#924 Phase 2):
/// sends `?lat=&lng=&radius=&bbox=&zoom=` (no `status` — the anonymous map has
/// no filter chips), decodes the bare `[AnonymousMapClusterDTO]` body, and maps
/// each cell — including the `authoritySlug` carried on cluster members — into
/// an ``AnonymousMapCluster``. Mirrors `APIPlanningApplicationRepositoryClustersTests`.
@Suite("APIAnonymousApplicationsRepository — clusters fetch")
struct APIAnonymousApplicationsRepositoryClustersTests {
  /// A `guard`/`fatalError` — not `URL(string:)!` — keeps this literal,
  /// well-formed URL free of `force_unwrapping` regardless of which
  /// SwiftLint version is enforcing the rule: local and CI swiftlint have
  /// drifted on whether this exact `!` pattern triggers it (tc-2wu29 PR
  /// review).
  private var baseURL: URL {
    guard let url = URL(string: "https://api-dev.towncrierapp.uk") else {
      fatalError("Invalid literal test API base URL")
    }
    return url
  }

  private let viewport = MapViewport(west: -0.32, south: 51.40, east: -0.26, north: 51.43)

  private func makeSUT(
    responses: [(Data, URLResponse)]
  ) -> (APIAnonymousApplicationsRepository, StubHTTPTransport) {
    let transport = StubHTTPTransport()
    transport.responses = responses
    let apiClient = AnonymousURLSessionAPIClient(baseURL: baseURL, transport: transport)
    return (APIAnonymousApplicationsRepository(apiClient: apiClient), transport)
  }

  // swiftlint:disable force_unwrapping
  private func httpResponse(statusCode: Int) -> HTTPURLResponse {
    HTTPURLResponse(url: baseURL, statusCode: statusCode, httpVersion: nil, headerFields: nil)!
  }
  // swiftlint:enable force_unwrapping

  private func queryItems(_ transport: StubHTTPTransport, at index: Int) throws -> [URLQueryItem] {
    let url = try #require(transport.requests[index].url)
    let components = URLComponents(url: url, resolvingAgainstBaseURL: false)
    return components?.queryItems ?? []
  }

  private static let clustersJSON = """
    [
      {
        "latitude": 51.41,
        "longitude": -0.30,
        "count": 82,
        "statusCounts": { "Permitted": 55, "Rejected": 11, "Undecided": 16 },
        "applicationId": null
      },
      {
        "latitude": 51.412,
        "longitude": -0.288,
        "count": 1,
        "statusCounts": { "Permitted": 1 },
        "applicationId": { "authority": "314", "name": "Kingston/22/02956/CLC", "authoritySlug": "kingston" }
      }
    ]
    """

  @Test("sends lat/lng/radius/bbox/zoom and no status param")
  func fetchClusters_sendsCorrectRequest() async throws {
    let (sut, transport) = makeSUT(responses: [
      (Data("[]".utf8), httpResponse(statusCode: 200))
    ])

    _ = try await sut.fetchClusters(
      latitude: 51.414, longitude: -0.29, radiusMetres: 2000, viewport: viewport, zoom: 13)

    let url = try #require(transport.requests[0].url)
    #expect(url.path.contains("/v1/applications/clusters"))
    #expect(transport.requests[0].value(forHTTPHeaderField: "Authorization") == nil)
    let items = try queryItems(transport, at: 0)
    #expect(items.contains(URLQueryItem(name: "lat", value: "51.414")))
    #expect(items.contains(URLQueryItem(name: "lng", value: "-0.29")))
    #expect(items.contains(URLQueryItem(name: "radius", value: "2000.0")))
    #expect(items.contains(URLQueryItem(name: "bbox", value: "-0.32,51.4,-0.26,51.43")))
    #expect(items.contains(URLQueryItem(name: "zoom", value: "13")))
    #expect(!items.contains { $0.name == "status" })
  }

  @Test("decodes the cluster array including the single-member case with authoritySlug")
  func fetchClusters_decodesMultiAndSingleMemberCells() async throws {
    let (sut, _) = makeSUT(responses: [
      (Data(Self.clustersJSON.utf8), httpResponse(statusCode: 200))
    ])

    let clusters = try await sut.fetchClusters(
      latitude: 51.414, longitude: -0.29, radiusMetres: 2000, viewport: viewport, zoom: 13)

    #expect(clusters.count == 2)

    let bubble = try #require(clusters.first { $0.count > 1 })
    #expect(bubble.member == nil)
    #expect(bubble.statusCounts[.permitted] == 55)
    #expect(bubble.statusCounts[.rejected] == 11)
    #expect(bubble.statusCounts[.undecided] == 16)

    let single = try #require(clusters.first { $0.count == 1 })
    #expect(
      single.member
        == AnonymousClusterMember(authority: "314", name: "Kingston/22/02956/CLC", authoritySlug: "kingston"))
    #expect(single.memberStatus == .permitted)
  }

  private static let stackedClustersJSON = """
    [
      {
        "latitude": 51.413,
        "longitude": -0.293,
        "count": 2,
        "statusCounts": { "Permitted": 2 },
        "applicationId": null,
        "applicationIds": [
          { "authority": "314", "name": "Kingston/20/00025/CPU", "authoritySlug": "kingston" },
          { "authority": "314", "name": "Kingston/20/00026/HOU", "authoritySlug": "kingston" }
        ]
      },
      {
        "latitude": 51.416,
        "longitude": -0.291,
        "count": 40,
        "statusCounts": { "Permitted": 40 },
        "applicationId": null
      }
    ]
    """

  @Test("decodes applicationIds into AnonymousMapCluster.members for an unsplittable cell")
  func fetchClusters_decodesStackedMembers() async throws {
    let (sut, _) = makeSUT(responses: [
      (Data(Self.stackedClustersJSON.utf8), httpResponse(statusCode: 200))
    ])

    let clusters = try await sut.fetchClusters(
      latitude: 51.414, longitude: -0.29, radiusMetres: 300, viewport: viewport, zoom: 19)

    #expect(clusters.count == 2)

    let stackedCell = clusters.first(where: \.isStacked)
    let stacked = try #require(stackedCell)
    #expect(stacked.count == 2)
    #expect(
      stacked.members == [
        AnonymousClusterMember(authority: "314", name: "Kingston/20/00025/CPU", authoritySlug: "kingston"),
        AnonymousClusterMember(authority: "314", name: "Kingston/20/00026/HOU", authoritySlug: "kingston"),
      ])

    let splittable = try #require(clusters.first { $0.count == 40 })
    #expect(splittable.members.isEmpty)
    #expect(!splittable.isStacked)
  }

  @Test("a member with an absent authoritySlug decodes to an empty slug")
  func fetchClusters_absentAuthoritySlug_decodesToEmptyString() async throws {
    let json = """
      [
        {
          "latitude": 51.412,
          "longitude": -0.288,
          "count": 1,
          "statusCounts": { "Permitted": 1 },
          "applicationId": { "authority": "314", "name": "Kingston/22/02956/CLC" }
        }
      ]
      """
    let (sut, _) = makeSUT(responses: [(Data(json.utf8), httpResponse(statusCode: 200))])

    let clusters = try await sut.fetchClusters(
      latitude: 51.414, longitude: -0.29, radiusMetres: 2000, viewport: viewport, zoom: 13)

    let single = try #require(clusters.first)
    #expect(single.member?.authoritySlug.isEmpty == true)
  }

  @Test("maps a network error to DomainError.networkUnavailable")
  func fetchClusters_networkError_mapsToDomainError() async throws {
    let transport = StubHTTPTransport()
    transport.error = URLError(.notConnectedToInternet)
    let apiClient = AnonymousURLSessionAPIClient(baseURL: baseURL, transport: transport)
    let sut = APIAnonymousApplicationsRepository(apiClient: apiClient)

    await #expect(throws: DomainError.networkUnavailable) {
      _ = try await sut.fetchClusters(
        latitude: 51.414, longitude: -0.29, radiusMetres: 2000, viewport: viewport, zoom: 13)
    }
  }
}
