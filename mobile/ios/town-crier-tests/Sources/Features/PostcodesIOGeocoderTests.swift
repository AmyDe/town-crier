import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierData

@Suite("PostcodesIOGeocoder")
struct PostcodesIOGeocoderTests {

  // MARK: - Helpers

  // swiftlint:disable:next force_unwrapping
  private let baseURL = URL(string: "https://api.postcodes.io")!

  private func makeSUT(
    responses: [(Data, URLResponse)]
  ) -> (PostcodesIOGeocoder, StubHTTPTransport) {
    let transport = StubHTTPTransport()
    transport.responses = responses
    let sut = PostcodesIOGeocoder(baseURL: baseURL, transport: transport)
    return (sut, transport)
  }

  // swiftlint:disable force_unwrapping
  private func httpResponse(statusCode: Int) -> HTTPURLResponse {
    HTTPURLResponse(url: baseURL, statusCode: statusCode, httpVersion: nil, headerFields: nil)!
  }
  // swiftlint:enable force_unwrapping

  // MARK: - Success

  @Test("geocode sends GET to postcodes.io directly, no /v1/geocode")
  func geocode_sendsDirectRequest_toPostcodesIO() async throws {
    let json =
      #"{"status":200,"result":{"postcode":"CB1 2AD","latitude":52.2053,"longitude":0.1218}}"#
    let (sut, transport) = makeSUT(responses: [(Data(json.utf8), httpResponse(statusCode: 200))])

    let postcode = try Postcode("CB1 2AD")
    _ = try await sut.geocode(postcode)

    #expect(transport.requests.count == 1)
    let request = transport.requests[0]
    #expect(request.url?.host == "api.postcodes.io")
    #expect(request.url?.path.contains("/postcodes/") == true)
    #expect(request.url?.path.contains("/v1/geocode") == false)
  }

  @Test("geocode returns the mapped coordinate on success")
  func geocode_success_returnsMappedCoordinate() async throws {
    let json =
      #"{"status":200,"result":{"postcode":"CB1 2AD","latitude":52.2053,"longitude":0.1218}}"#
    let (sut, _) = makeSUT(responses: [(Data(json.utf8), httpResponse(statusCode: 200))])

    let result = try await sut.geocode(try Postcode("CB1 2AD"))

    #expect(result == (try Coordinate(latitude: 52.2053, longitude: 0.1218)))
  }

  // MARK: - Error mapping

  @Test(
    "geocode with postcodes.io 404 throws the same geocodingFailed DomainError APIPostcodeGeocoder uses"
  )
  func geocode_notFound_throwsGeocodingFailed() async throws {
    // Real postcodes.io shape for an unknown postcode.
    let json = #"{"status":404,"error":"Postcode not found"}"#
    let (sut, _) = makeSUT(responses: [(Data(json.utf8), httpResponse(statusCode: 404))])

    await #expect(throws: DomainError.geocodingFailed("ZZ9 9ZZ")) {
      _ = try await sut.geocode(try Postcode("ZZ9 9ZZ"))
    }
  }

  @Test("geocode with server error throws geocodingFailed")
  func geocode_serverError_throwsGeocodingFailed() async throws {
    let (sut, _) = makeSUT(responses: [(Data(), httpResponse(statusCode: 500))])

    await #expect(throws: DomainError.geocodingFailed("CB1 2AD")) {
      _ = try await sut.geocode(try Postcode("CB1 2AD"))
    }
  }

  @Test("geocode with network error throws networkUnavailable")
  func geocode_networkError_throwsNetworkUnavailable() async throws {
    let transport = StubHTTPTransport()
    transport.error = URLError(.notConnectedToInternet)
    let sut = PostcodesIOGeocoder(baseURL: baseURL, transport: transport)

    await #expect(throws: DomainError.networkUnavailable) {
      _ = try await sut.geocode(try Postcode("CB1 2AD"))
    }
  }

  @Test("geocode with malformed body throws geocodingFailed")
  func geocode_malformedBody_throwsGeocodingFailed() async throws {
    let (sut, _) = makeSUT(responses: [(Data("not json".utf8), httpResponse(statusCode: 200))])

    await #expect(throws: DomainError.geocodingFailed("CB1 2AD")) {
      _ = try await sut.geocode(try Postcode("CB1 2AD"))
    }
  }
}
