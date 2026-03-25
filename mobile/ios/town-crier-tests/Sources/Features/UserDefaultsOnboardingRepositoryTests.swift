import Foundation
import Testing
import TownCrierData

@Suite("UserDefaultsOnboardingRepository")
struct UserDefaultsOnboardingRepositoryTests {
    private func makeSUT() throws -> (UserDefaultsOnboardingRepository, UserDefaults) {
        let defaults = try #require(UserDefaults(suiteName: UUID().uuidString))
        let sut = UserDefaultsOnboardingRepository(defaults: defaults)
        return (sut, defaults)
    }

    @Test("isOnboardingComplete defaults to false")
    func defaultsToFalse() throws {
        let (sut, _) = try makeSUT()

        #expect(!sut.isOnboardingComplete)
    }

    @Test("markOnboardingComplete sets isOnboardingComplete to true")
    func markCompleteSetsTrueFlag() throws {
        let (sut, _) = try makeSUT()

        sut.markOnboardingComplete()

        #expect(sut.isOnboardingComplete)
    }

    @Test("isOnboardingComplete reads from UserDefaults")
    func readsFromUserDefaults() throws {
        let defaults = try #require(UserDefaults(suiteName: UUID().uuidString))
        defaults.set(true, forKey: "isOnboardingComplete")
        let sut = UserDefaultsOnboardingRepository(defaults: defaults)

        #expect(sut.isOnboardingComplete)
    }

    @Test("markOnboardingComplete persists to UserDefaults")
    func persistsToUserDefaults() throws {
        let (sut, defaults) = try makeSUT()

        sut.markOnboardingComplete()

        #expect(defaults.bool(forKey: "isOnboardingComplete"))
    }
}
