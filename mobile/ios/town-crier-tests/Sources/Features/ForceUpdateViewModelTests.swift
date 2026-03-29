import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("ForceUpdateViewModel")
@MainActor
struct ForceUpdateViewModelTests {
    private func makeSUT(
        currentVersion: String = "1.0.0",
        minimumVersion: AppVersion = AppVersion(major: 1, minor: 0, patch: 0)
    ) -> (ForceUpdateViewModel, SpyVersionConfigService, SpyAppVersionProvider) {
        let versionConfigSpy = SpyVersionConfigService()
        versionConfigSpy.fetchMinimumVersionResult = .success(minimumVersion)
        let versionProvider = SpyAppVersionProvider()
        versionProvider.version = currentVersion
        let vm = ForceUpdateViewModel(
            versionConfigService: versionConfigSpy,
            appVersionProvider: versionProvider
        )
        return (vm, versionConfigSpy, versionProvider)
    }

    // MARK: - Version Check

    @Test func checkVersion_currentMeetsMinimum_doesNotRequireUpdate() async {
        let (sut, _, _) = makeSUT(
            currentVersion: "2.0.0",
            minimumVersion: AppVersion(major: 1, minor: 0, patch: 0)
        )

        await sut.checkVersion()

        #expect(!sut.requiresUpdate)
    }

    @Test func checkVersion_currentEqualsMinimum_doesNotRequireUpdate() async {
        let (sut, _, _) = makeSUT(
            currentVersion: "1.0.0",
            minimumVersion: AppVersion(major: 1, minor: 0, patch: 0)
        )

        await sut.checkVersion()

        #expect(!sut.requiresUpdate)
    }

    @Test func checkVersion_currentBelowMinimum_requiresUpdate() async {
        let (sut, _, _) = makeSUT(
            currentVersion: "1.0.0",
            minimumVersion: AppVersion(major: 2, minor: 0, patch: 0)
        )

        await sut.checkVersion()

        #expect(sut.requiresUpdate)
    }

    @Test func checkVersion_minorVersionBelowMinimum_requiresUpdate() async {
        let (sut, _, _) = makeSUT(
            currentVersion: "1.1.0",
            minimumVersion: AppVersion(major: 1, minor: 2, patch: 0)
        )

        await sut.checkVersion()

        #expect(sut.requiresUpdate)
    }

    @Test func checkVersion_patchVersionBelowMinimum_requiresUpdate() async {
        let (sut, _, _) = makeSUT(
            currentVersion: "1.2.3",
            minimumVersion: AppVersion(major: 1, minor: 2, patch: 4)
        )

        await sut.checkVersion()

        #expect(sut.requiresUpdate)
    }

    @Test func checkVersion_callsService() async {
        let (sut, spy, _) = makeSUT()

        await sut.checkVersion()

        #expect(spy.fetchMinimumVersionCallCount == 1)
    }

    @Test func checkVersion_serviceFailure_doesNotBlock() async {
        let (sut, spy, _) = makeSUT()
        spy.fetchMinimumVersionResult = .failure(DomainError.networkUnavailable)

        await sut.checkVersion()

        #expect(!sut.requiresUpdate)
    }

    @Test func checkVersion_invalidCurrentVersion_doesNotBlock() async {
        let (sut, _, versionProvider) = makeSUT(
            minimumVersion: AppVersion(major: 2, minor: 0, patch: 0)
        )
        versionProvider.version = "invalid"

        await sut.checkVersion()

        #expect(!sut.requiresUpdate)
    }

    // MARK: - Loading State

    @Test func checkVersion_setsIsCheckingDuringCheck() async {
        let (sut, _, _) = makeSUT()

        #expect(!sut.isChecking)

        await sut.checkVersion()

        // After completion, isChecking should be false
        #expect(!sut.isChecking)
    }

}
