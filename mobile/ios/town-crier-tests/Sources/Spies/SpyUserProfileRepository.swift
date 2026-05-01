import TownCrierDomain

final class SpyUserProfileRepository: UserProfileRepository, @unchecked Sendable {
  private(set) var createCallCount = 0
  var createResult: Result<ServerProfile, Error> = .success(
    ServerProfile(
      userId: "spy-user",
      tier: .free,
      pushEnabled: true,
      digestDay: .monday,
      emailDigestEnabled: true
    )
  )

  func create() async throws -> ServerProfile {
    createCallCount += 1
    return try createResult.get()
  }

  private(set) var fetchCallCount = 0
  var fetchResult: Result<ServerProfile?, Error> = .success(nil)

  func fetch() async throws -> ServerProfile? {
    fetchCallCount += 1
    return try fetchResult.get()
  }

  struct UpdateCall: Equatable {
    let pushEnabled: Bool
    let digestDay: DayOfWeek
    let emailDigestEnabled: Bool
    let savedDecisionPush: Bool
    let savedDecisionEmail: Bool
  }

  private(set) var updateCalls: [UpdateCall] = []
  var updateResult: Result<ServerProfile, Error> = .success(
    ServerProfile(
      userId: "spy-user",
      tier: .free,
      pushEnabled: true,
      digestDay: .monday,
      emailDigestEnabled: true
    )
  )

  func update(
    pushEnabled: Bool,
    digestDay: DayOfWeek,
    emailDigestEnabled: Bool,
    savedDecisionPush: Bool,
    savedDecisionEmail: Bool
  ) async throws -> ServerProfile {
    updateCalls.append(
      UpdateCall(
        pushEnabled: pushEnabled,
        digestDay: digestDay,
        emailDigestEnabled: emailDigestEnabled,
        savedDecisionPush: savedDecisionPush,
        savedDecisionEmail: savedDecisionEmail
      ))
    return try updateResult.get()
  }

  private(set) var deleteCallCount = 0
  var deleteResult: Result<Void, Error> = .success(())
  var onDelete: (() -> Void)?

  func delete() async throws {
    deleteCallCount += 1
    onDelete?()
    try deleteResult.get()
  }
}
