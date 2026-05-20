import Testing

@testable import TownCrierDomain

@Suite("PlanningApplicationId")
struct PlanningApplicationIdTests {
  // MARK: - Struct shape

  @Test("has authority and name properties")
  func hasAuthorityAndNameProperties() {
    let id = PlanningApplicationId(authority: "42", name: "22/1234/FUL")

    #expect(id.authority == "42")
    #expect(id.name == "22/1234/FUL")
  }

  @Test("two ids with same authority and name are equal")
  func equality_sameAuthorityAndName_isEqual() {
    let id1 = PlanningApplicationId(authority: "42", name: "22/1234/FUL")
    let id2 = PlanningApplicationId(authority: "42", name: "22/1234/FUL")

    #expect(id1 == id2)
  }

  @Test("two ids with different authority are not equal")
  func equality_differentAuthority_isNotEqual() {
    let id1 = PlanningApplicationId(authority: "42", name: "22/1234/FUL")
    let id2 = PlanningApplicationId(authority: "99", name: "22/1234/FUL")

    #expect(id1 != id2)
  }

  @Test("two ids with different name are not equal")
  func equality_differentName_isNotEqual() {
    let id1 = PlanningApplicationId(authority: "42", name: "22/1234/FUL")
    let id2 = PlanningApplicationId(authority: "42", name: "22/9999/FUL")

    #expect(id1 != id2)
  }

  @Test("is usable as a dictionary key (Hashable)")
  func hashable_usableAsDictionaryKey() {
    let id = PlanningApplicationId(authority: "42", name: "22/1234/FUL")
    var dict: [PlanningApplicationId: String] = [:]
    dict[id] = "present"

    #expect(dict[id] == "present")
  }
}
