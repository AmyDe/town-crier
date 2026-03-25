import Foundation
import TownCrierDomain

/// Fetches the minimum supported app version from the Town Crier API.
/// This is an unauthenticated endpoint — it must work before the user logs in
/// so the force-update check can block outdated app versions.
public struct APIVersionConfigService: VersionConfigService {
    private let baseURL: URL
    private let transport: HTTPTransport

    public init(baseURL: URL, transport: HTTPTransport = URLSession.shared) {
        self.baseURL = baseURL
        self.transport = transport
    }

    public func fetchMinimumVersion() async throws -> AppVersion {
        let url = baseURL.appendingPathComponent("v1/version-config")
        var request = URLRequest(url: url)
        request.httpMethod = "GET"
        request.setValue("application/json", forHTTPHeaderField: "Accept")

        let data: Data
        let response: URLResponse
        do {
            (data, response) = try await transport.data(for: request)
        } catch is URLError {
            throw DomainError.networkUnavailable
        } catch {
            throw DomainError.unexpected(error.localizedDescription)
        }

        guard let httpResponse = response as? HTTPURLResponse,
              (200...299).contains(httpResponse.statusCode)
        else {
            throw DomainError.unexpected("Version config request failed")
        }

        let dto: VersionConfigDTO
        do {
            dto = try JSONDecoder().decode(VersionConfigDTO.self, from: data)
        } catch {
            throw DomainError.unexpected("Invalid version config response")
        }

        guard let version = AppVersion(dto.minimumVersion) else {
            throw DomainError.unexpected("Invalid version string: \(dto.minimumVersion)")
        }

        return version
    }
}

private struct VersionConfigDTO: Decodable {
    let minimumVersion: String
}
