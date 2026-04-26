import Foundation
import Testing
import TownCrierDomain

@Suite("SearchResult")
struct SearchResultTests {

  @Test("hasMore returns true when more pages exist")
  func hasMore_morePagesExist_returnsTrue() {
    let result = SearchResult(
      applications: [.pendingReview, .permitted],
      total: 10,
      page: 1
    )

    #expect(result.hasMore)
  }

  @Test("hasMore returns false when on last page")
  func hasMore_lastPage_returnsFalse() {
    let result = SearchResult(
      applications: [.pendingReview, .permitted],
      total: 2,
      page: 1
    )

    #expect(!result.hasMore)
  }

  @Test("hasMore returns false when applications is empty")
  func hasMore_emptyApplications_returnsFalse() {
    let result = SearchResult(
      applications: [],
      total: 0,
      page: 1
    )

    #expect(!result.hasMore)
  }

  @Test("hasMore returns false when total equals page times pageSize")
  func hasMore_exactlyFull_returnsFalse() {
    let result = SearchResult(
      applications: [.pendingReview, .permitted, .rejected],
      total: 6,
      page: 2
    )

    #expect(!result.hasMore)
  }

  @Test("hasMore returns true when total exceeds page times pageSize")
  func hasMore_moreRemaining_returnsTrue() {
    let result = SearchResult(
      applications: [.pendingReview, .permitted, .rejected],
      total: 7,
      page: 2
    )

    #expect(result.hasMore)
  }

  @Test("equality compares all fields")
  func equality_allFieldsMatch_areEqual() {
    let a = SearchResult(applications: [.pendingReview], total: 1, page: 1)
    let b = SearchResult(applications: [.pendingReview], total: 1, page: 1)

    #expect(a == b)
  }

  @Test("inequality when page differs")
  func inequality_pageDiffers_notEqual() {
    let a = SearchResult(applications: [.pendingReview], total: 1, page: 1)
    let b = SearchResult(applications: [.pendingReview], total: 1, page: 2)

    #expect(a != b)
  }
}
