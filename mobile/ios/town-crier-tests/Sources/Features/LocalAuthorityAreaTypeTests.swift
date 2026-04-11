import Testing
import TownCrierDomain

@Suite("LocalAuthority areaType support")
struct LocalAuthorityAreaTypeTests {

  @Test("LocalAuthority stores areaType when provided")
  func storesAreaType() {
    let authority = LocalAuthority(code: "123", name: "Bath and NE Somerset", areaType: "Unitary")

    #expect(authority.areaType == "Unitary")
  }

  @Test("LocalAuthority defaults areaType to nil")
  func defaultsAreaTypeToNil() {
    let authority = LocalAuthority(code: "123", name: "Cambridge")

    #expect(authority.areaType == nil)
  }

  @Test("Two authorities with different areaType are not equal")
  func equalityConsidersAreaType() {
    let a = LocalAuthority(code: "123", name: "Bath", areaType: "Unitary")
    let b = LocalAuthority(code: "123", name: "Bath", areaType: "District")

    #expect(a != b)
  }

  @Test("Two authorities with same areaType are equal")
  func equalityWithSameAreaType() {
    let a = LocalAuthority(code: "123", name: "Bath", areaType: "Unitary")
    let b = LocalAuthority(code: "123", name: "Bath", areaType: "Unitary")

    #expect(a == b)
  }
}
