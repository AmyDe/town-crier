import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// Tests for the self-service GDPR data export flow in `SettingsViewModel`.
///
/// Tapping "Export your data" calls `UserProfileRepository.exportData()`
/// (GET /v1/me/data), writes the opaque server bytes to a temp `.json` file,
/// and exposes the file URL so the View can present the iOS share sheet.
/// Loading and error states are handled in the ViewModel; the View only renders.
@Suite("SettingsViewModel data export")
@MainActor
struct SettingsViewModelDataExportTests {
  private func makeSUT(
    exportResult: Result<Data, Error> = .success(Data(#"{"profile":{}}"#.utf8)),
    writer: ((Data) throws -> URL)? = nil
  ) -> (SettingsViewModel, SpyUserProfileRepository) {
    let authSpy = SpyAuthenticationService()
    authSpy.currentSessionResult = .valid
    let subscriptionSpy = SpySubscriptionService()
    let profileSpy = SpyUserProfileRepository()
    profileSpy.exportDataResult = exportResult
    let versionProvider = SpyAppVersionProvider()
    let notificationSpy = SpyNotificationService()
    let defaults = UserDefaults(suiteName: "SettingsVMExportTests.\(UUID().uuidString)")
    let vm = SettingsViewModel(
      authService: authSpy,
      subscriptionService: subscriptionSpy,
      userProfileRepository: profileSpy,
      appVersionProvider: versionProvider,
      notificationService: notificationSpy,
      defaults: defaults ?? .standard,
      exportFileWriter: writer ?? { data in
        let url = FileManager.default.temporaryDirectory
          .appendingPathComponent("export-test-\(UUID().uuidString).json")
        try data.write(to: url)
        return url
      }
    )
    return (vm, profileSpy)
  }

  // MARK: - Success

  @Test func exportData_callsRepositoryExport() async {
    let (sut, profileSpy) = makeSUT()

    await sut.exportData()

    #expect(profileSpy.exportDataCallCount == 1)
  }

  @Test func exportData_success_writesServerBytesVerbatimAndSetsFileURL() async {
    let bytes = Data(#"{"profile":{"id":"abc"},"watchZones":[]}"#.utf8)
    var written: Data?
    let stagedURL = FileManager.default.temporaryDirectory
      .appendingPathComponent("staged-export.json")
    let (sut, _) = makeSUT(exportResult: .success(bytes)) { data in
      written = data
      return stagedURL
    }

    await sut.exportData()

    #expect(written == bytes, "the server bytes must be preserved as-is")
    #expect(sut.exportFileURL == stagedURL)
    #expect(sut.error == nil)
    #expect(!sut.isExporting)
  }

  @Test func exportData_clearsExportingFlagAfterSuccess() async {
    let (sut, _) = makeSUT()

    await sut.exportData()

    #expect(!sut.isExporting)
  }

  // MARK: - Failure

  @Test func exportData_failure_setsErrorAndProducesNoArtifact() async {
    let (sut, _) = makeSUT(exportResult: .failure(DomainError.networkUnavailable))

    await sut.exportData()

    #expect(sut.error == .networkUnavailable)
    #expect(sut.exportFileURL == nil, "no shareable artifact on failure")
    #expect(!sut.isExporting)
  }

  @Test func exportData_writerThrows_setsErrorAndNoArtifact() async {
    struct WriteFailure: Error {}
    let (sut, _) = makeSUT { _ in throw WriteFailure() }

    await sut.exportData()

    #expect(sut.error != nil)
    #expect(sut.exportFileURL == nil)
    #expect(!sut.isExporting)
  }

  // MARK: - Share-sheet lifecycle

  @Test func dismissExportShare_clearsFileURL() async {
    let (sut, _) = makeSUT()
    await sut.exportData()
    #expect(sut.exportFileURL != nil)

    sut.dismissExportShare()

    #expect(sut.exportFileURL == nil)
  }
}
