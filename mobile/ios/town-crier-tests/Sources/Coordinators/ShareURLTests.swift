import Foundation
import Testing

@testable import TownCrierPresentation

@Suite("ShareURL")
struct ShareURLTests {
  @Test func origin_isTheCanonicalShareSubdomain() {
    #expect(ShareURL.origin == "https://share.towncrierapp.uk")
  }

  @Test func build_preservesRefSlashesAsPathSeparators() {
    // The ref is the application's full area-prefixed PlanIt `name`, verbatim —
    // it contains slashes and must land as path separators, not be encoded.
    let url = ShareURL.build(authoritySlug: "kingston", ref: "Kingston/25/02755/CLC")

    #expect(
      url?.absoluteString == "https://share.towncrierapp.uk/a/kingston/Kingston/25/02755/CLC")
  }

  @Test func build_withSimpleRef_buildsCanonicalURL() {
    let url = ShareURL.build(authoritySlug: "croydon", ref: "23/03456/FUL")

    #expect(url?.absoluteString == "https://share.towncrierapp.uk/a/croydon/23/03456/FUL")
  }

  @Test func build_withEmptySlug_returnsNil() {
    #expect(ShareURL.build(authoritySlug: "", ref: "23/03456/FUL") == nil)
  }

  @Test func build_withEmptyRef_returnsNil() {
    #expect(ShareURL.build(authoritySlug: "croydon", ref: "") == nil)
  }
}
