import Testing

@testable import TownCrierPresentation

@Suite("LegalDocumentViewModel")
struct LegalDocumentViewModelTests {
  @Test func init_privacyPolicy_setsTitle() {
    let sut = LegalDocumentViewModel(documentType: .privacyPolicy)
    #expect(sut.title == "Privacy Policy")
  }

  @Test func init_termsOfService_setsTitle() {
    let sut = LegalDocumentViewModel(documentType: .termsOfService)
    #expect(sut.title == "Terms of Service")
  }

  @Test func init_privacyPolicy_hasNonEmptyContent() {
    let sut = LegalDocumentViewModel(documentType: .privacyPolicy)
    #expect(!sut.sections.isEmpty)
  }

  @Test func init_termsOfService_hasNonEmptyContent() {
    let sut = LegalDocumentViewModel(documentType: .termsOfService)
    #expect(!sut.sections.isEmpty)
  }

  @Test func init_privacyPolicy_coversDataCollection() {
    let sut = LegalDocumentViewModel(documentType: .privacyPolicy)
    let allContent = sut.sections.map(\.body).joined()
    #expect(allContent.contains("collect"))
  }

  @Test func init_privacyPolicy_coversDataProcessing() {
    let sut = LegalDocumentViewModel(documentType: .privacyPolicy)
    let allContent = sut.sections.map(\.body).joined()
    #expect(allContent.contains("process"))
  }

  @Test func init_privacyPolicy_coversDeletionRights() {
    let sut = LegalDocumentViewModel(documentType: .privacyPolicy)
    let allContent = sut.sections.map(\.body).joined()
    #expect(allContent.contains("delet"))
  }

  @Test func init_hasLastUpdatedDate() {
    let sut = LegalDocumentViewModel(documentType: .privacyPolicy)
    #expect(!sut.lastUpdated.isEmpty)
  }

  @Test func sections_haveNonEmptyHeadings() {
    let sut = LegalDocumentViewModel(documentType: .privacyPolicy)
    for section in sut.sections {
      #expect(!section.heading.isEmpty)
    }
  }

  @Test func sections_haveNonEmptyBodies() {
    let sut = LegalDocumentViewModel(documentType: .privacyPolicy)
    for section in sut.sections {
      #expect(!section.body.isEmpty)
    }
  }
}
