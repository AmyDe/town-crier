import Foundation
import TownCrierDomain

/// Registers and removes APNs device tokens via the Town Crier API.
/// Stores the last registered token so `removeDeviceToken()` can unregister
/// without callers needing to track the token value.
public actor APINotificationService {
  private let apiClient: URLSessionAPIClient
  private var storedToken: String?

  public init(apiClient: URLSessionAPIClient) {
    self.apiClient = apiClient
  }

  public func registerDeviceToken(_ token: String) async throws {
    let body = RegisterDeviceTokenBody(token: token, platform: "Ios")
    let _: EmptyResponse = try await apiClient.request(.put("v1/me/device-token", body: body))
    storedToken = token
  }

  public func removeDeviceToken() async throws {
    guard let token = storedToken else { return }
    let _: EmptyResponse = try await apiClient.request(.delete("v1/me/device-token/\(token)"))
    storedToken = nil
  }
}

private struct RegisterDeviceTokenBody: Encodable, Sendable {
  let token: String
  let platform: String
}
