import Testing

@testable import TownCrierDomain

@Suite("Postcode validation")
struct PostcodeTests {
    @Test func validPostcode_succeeds() throws {
        let postcode = try Postcode("CB1 2AD")
        #expect(postcode.value == "CB1 2AD")
    }

    @Test func lowercasePostcode_normalisesToUppercase() throws {
        let postcode = try Postcode("cb1 2ad")
        #expect(postcode.value == "CB1 2AD")
    }

    @Test func postcodeWithExtraWhitespace_trims() throws {
        let postcode = try Postcode("  CB1 2AD  ")
        #expect(postcode.value == "CB1 2AD")
    }

    @Test func invalidPostcode_throwsError() {
        #expect(throws: DomainError.invalidPostcode("INVALID")) {
            try Postcode("INVALID")
        }
    }

    @Test func emptyString_throwsError() {
        #expect(throws: (any Error).self) {
            try Postcode("")
        }
    }
}
