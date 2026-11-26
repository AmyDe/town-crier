import Foundation
import TownCrierDomain

/// ``WebhookRepository`` backed by the `/v1/me/webhook` endpoints.
public final class APIWebhookRepository: WebhookRepository, Sendable {
  private static let path = "/v1/me/webhook"

  private let apiClient: URLSessionAPIClient

  public init(apiClient: URLSessionAPIClient) {
    self.apiClient = apiClient
  }

  public func fetch() async throws -> Webhook? {
    do {
      let dto: WebhookDTO = try await apiClient.request(.get(Self.path))
      return try dto.toDomain()
    } catch APIError.notFound {
      return nil
    } catch {
      throw domainError(from: error)
    }
  }

  public func save(url: String, format: WebhookFormat) async throws -> WebhookSaveResult {
    do {
      let body = SaveWebhookDTO(url: url, format: format.rawValue)
      let dto: WebhookDTO = try await apiClient.request(.put(Self.path, body: body))
      return WebhookSaveResult(webhook: try dto.toDomain(), secret: dto.secret)
    } catch {
      throw domainError(from: error)
    }
  }

  public func remove() async throws {
    do {
      let _: EmptyResponse = try await apiClient.request(.delete(Self.path))
    } catch {
      throw domainError(from: error)
    }
  }

  public func rotateSecret() async throws -> String {
    do {
      let dto: WebhookSecretDTO = try await apiClient.request(.post("\(Self.path)/secret"))
      return dto.secret
    } catch APIError.notFound {
      throw DomainError.webhookNotFound
    } catch {
      throw domainError(from: error)
    }
  }

  public func sendTest() async throws -> WebhookTestResult {
    do {
      let dto: WebhookTestResultDTO = try await apiClient.request(.post("\(Self.path)/test"))
      return dto.toDomain()
    } catch APIError.notFound {
      throw DomainError.webhookNotFound
    } catch {
      throw domainError(from: error)
    }
  }

  private func domainError(from error: any Error) -> any Error {
    if error is DomainError { return error }
    return error.toThrowableDomainError()
  }
}

// MARK: - DTO

private struct SaveWebhookDTO: Encodable, Sendable {
  let url: String
  let format: String
}

private struct WebhookDTO: Decodable, Sendable {
  let url: String
  let format: String
  let status: String
  let disabledReason: String?
  let paused: Bool
  let createdAt: String
  let lastSuccessAt: String?
  let lastFailureAt: String?
  let lastFailureReason: String?
  let lastFailureStatus: Int?
  let secret: String?

  func toDomain() throws -> Webhook {
    let formatter = ISO8601DateFormatter()
    guard
      let format = WebhookFormat(rawValue: format),
      let status = WebhookStatus(rawValue: status),
      let createdAt = formatter.date(from: createdAt)
    else {
      throw APIError.decodingFailed("Unrecognised webhook response")
    }
    return Webhook(
      url: url,
      format: format,
      status: status,
      disabledReason: disabledReason.flatMap(WebhookDisabledReason.init(rawValue:)),
      isPaused: paused,
      createdAt: createdAt,
      lastSuccessAt: lastSuccessAt.flatMap(formatter.date(from:)),
      lastFailureAt: lastFailureAt.flatMap(formatter.date(from:)),
      lastFailureReason: lastFailureReason.flatMap(WebhookFailureReason.init(rawValue:)),
      lastFailureStatus: lastFailureStatus
    )
  }
}

private struct WebhookSecretDTO: Decodable, Sendable {
  let secret: String
}

private struct WebhookTestResultDTO: Decodable, Sendable {
  let delivered: Bool
  let status: Int?
  let failureReason: String?

  func toDomain() -> WebhookTestResult {
    WebhookTestResult(
      delivered: delivered,
      status: status,
      failureReason: failureReason.flatMap(WebhookFailureReason.init(rawValue:))
    )
  }
}
