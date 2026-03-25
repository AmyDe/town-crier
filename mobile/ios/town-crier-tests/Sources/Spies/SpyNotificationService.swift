import Foundation
import TownCrierDomain

final class SpyNotificationService: NotificationService, @unchecked Sendable {
    private(set) var requestPermissionCallCount = 0
    var requestPermissionResult: Result<Bool, Error> = .success(true)

    func requestPermission() async throws -> Bool {
        requestPermissionCallCount += 1
        return try requestPermissionResult.get()
    }

    private(set) var registerDeviceTokenCalls: [String] = []
    var registerDeviceTokenResult: Result<Void, Error> = .success(())

    func registerDeviceToken(_ token: String) async throws {
        registerDeviceTokenCalls.append(token)
        try registerDeviceTokenResult.get()
    }

    private(set) var removeDeviceTokenCallCount = 0
    var removeDeviceTokenResult: Result<Void, Error> = .success(())

    func removeDeviceToken() async throws {
        removeDeviceTokenCallCount += 1
        try removeDeviceTokenResult.get()
    }
}
