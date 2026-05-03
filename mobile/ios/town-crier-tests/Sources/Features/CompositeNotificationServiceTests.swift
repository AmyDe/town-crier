import Foundation
import Testing
import TownCrierData
import TownCrierDomain

@Suite("CompositeNotificationService")
struct CompositeNotificationServiceTests {
  private let baseURL = URL(string: "https://api-dev.towncrierapp.uk")

  private func makeSUT() throws -> (
    CompositeNotificationService,
    SpyNotificationPermissionProvider,
    StubHTTPTransport,
    SpyRemoteNotificationRegistering
  ) {
    let url = try #require(baseURL)
    let permissionSpy = SpyNotificationPermissionProvider()
    let transport = StubHTTPTransport()
    let authSpy = SpyAuthenticationService()
    authSpy.currentSessionResult = .valid
    let apiClient = URLSessionAPIClient(
      baseURL: url,
      authService: authSpy,
      transport: transport
    )
    let apiService = APINotificationService(apiClient: apiClient)
    let remoteRegistrar = SpyRemoteNotificationRegistering()
    let sut = CompositeNotificationService(
      permissionProvider: permissionSpy,
      apiService: apiService,
      remoteRegistrar: remoteRegistrar
    )
    return (sut, permissionSpy, transport, remoteRegistrar)
  }

  private func makeResponse() throws -> (Data, HTTPURLResponse) {
    let url = try #require(baseURL)
    return (Data("{}".utf8), httpResponse(url: url, statusCode: 204))
  }

  // MARK: - requestPermission

  @Test("requestPermission delegates to permission provider and returns true")
  func requestPermission_delegatesToProvider_returnsTrue() async throws {
    let (sut, permissionSpy, _, _) = try makeSUT()
    permissionSpy.requestPermissionResult = .success(true)

    let result = try await sut.requestPermission()

    #expect(result == true)
    #expect(permissionSpy.requestPermissionCallCount == 1)
  }

  @Test("requestPermission delegates to permission provider and returns false")
  func requestPermission_delegatesToProvider_returnsFalse() async throws {
    let (sut, permissionSpy, _, _) = try makeSUT()
    permissionSpy.requestPermissionResult = .success(false)

    let result = try await sut.requestPermission()

    #expect(result == false)
  }

  @Test("requestPermission propagates errors from permission provider")
  func requestPermission_propagatesError() async throws {
    let (sut, permissionSpy, _, _) = try makeSUT()
    permissionSpy.requestPermissionResult = .failure(DomainError.notificationPermissionDenied)

    await #expect(throws: DomainError.self) {
      try await sut.requestPermission()
    }
  }

  @Test("requestPermission triggers registerForRemoteNotifications when granted")
  func requestPermission_granted_triggersRemoteRegistration() async throws {
    let (sut, permissionSpy, _, remoteRegistrar) = try makeSUT()
    permissionSpy.requestPermissionResult = .success(true)

    _ = try await sut.requestPermission()

    #expect(remoteRegistrar.registerForRemoteNotificationsCallCount == 1)
  }

  @Test("requestPermission does NOT trigger registerForRemoteNotifications when denied")
  func requestPermission_denied_doesNotRegisterRemote() async throws {
    let (sut, permissionSpy, _, remoteRegistrar) = try makeSUT()
    permissionSpy.requestPermissionResult = .success(false)

    _ = try await sut.requestPermission()

    #expect(remoteRegistrar.registerForRemoteNotificationsCallCount == 0)
  }

  // MARK: - authorizationStatus

  @Test("authorizationStatus delegates to permission provider — notDetermined")
  func authorizationStatus_delegatesToProvider_notDetermined() async throws {
    let (sut, permissionSpy, _, _) = try makeSUT()
    permissionSpy.nextAuthorizationStatus = .notDetermined

    let status = await sut.authorizationStatus()

    #expect(status == .notDetermined)
    #expect(permissionSpy.authorizationStatusCallCount == 1)
  }

  @Test("authorizationStatus delegates to permission provider — denied")
  func authorizationStatus_delegatesToProvider_denied() async throws {
    let (sut, permissionSpy, _, _) = try makeSUT()
    permissionSpy.nextAuthorizationStatus = .denied

    let status = await sut.authorizationStatus()

    #expect(status == .denied)
  }

  @Test("authorizationStatus delegates to permission provider — authorized")
  func authorizationStatus_delegatesToProvider_authorized() async throws {
    let (sut, permissionSpy, _, _) = try makeSUT()
    permissionSpy.nextAuthorizationStatus = .authorized

    let status = await sut.authorizationStatus()

    #expect(status == .authorized)
  }

  // MARK: - registerDeviceToken

  @Test("registerDeviceToken delegates to API service")
  func registerDeviceToken_delegatesToAPIService() async throws {
    let (sut, _, transport, _) = try makeSUT()
    transport.responses = [try makeResponse()]

    try await sut.registerDeviceToken("test-token-abc")

    let request = try #require(transport.requests.first)
    #expect(request.httpMethod == "PUT")
    #expect(request.url?.path().contains("v1/me/device-token") == true)
    let body = try #require(request.httpBody)
    let json = try JSONSerialization.jsonObject(with: body) as? [String: Any]
    #expect(json?["token"] as? String == "test-token-abc")
  }

  // MARK: - removeDeviceToken

  @Test("removeDeviceToken delegates to API service")
  func removeDeviceToken_delegatesToAPIService() async throws {
    let (sut, _, transport, _) = try makeSUT()
    transport.responses = [try makeResponse(), try makeResponse()]

    // Must register first so there's a stored token to remove
    try await sut.registerDeviceToken("token-to-remove")
    try await sut.removeDeviceToken()

    let removeRequest = try #require(transport.requests.last)
    #expect(removeRequest.httpMethod == "DELETE")
    #expect(removeRequest.url?.path().contains("token-to-remove") == true)
  }

  @Test("removeDeviceToken with no prior registration is a no-op")
  func removeDeviceToken_noPriorRegistration_noOp() async throws {
    let (sut, _, transport, _) = try makeSUT()

    try await sut.removeDeviceToken()

    #expect(transport.requests.isEmpty)
  }
}
