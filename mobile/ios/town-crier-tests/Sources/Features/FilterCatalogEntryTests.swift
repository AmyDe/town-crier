import Testing

@testable import TownCrierDomain

/// `FilterCatalogEntry` (GH#1104, bead tc-m8j90.2): the plain data-carrying
/// struct that replaces the closed `WatchZoneFilterKey` enum (GH#1098/
/// tc-hogkn) as the wire shape for one entry in the fetched pre-canned
/// watch-zone filter catalog.
@Suite("FilterCatalogEntry")
struct FilterCatalogEntryTests {
  @Test func init_storesAllFields() {
    let entry = FilterCatalogEntry(
      key: "loft_extension",
      displayName: "Loft extension",
      description: "Loft conversions: loft, dormer, hip-to-gable and roof space mentions."
    )

    #expect(entry.key == "loft_extension")
    #expect(entry.displayName == "Loft extension")
    #expect(
      entry.description == "Loft conversions: loft, dormer, hip-to-gable and roof space mentions.")
  }

  @Test func equality_matchingFields_areEqual() {
    let a = FilterCatalogEntry(key: "new_homes", displayName: "New homes", description: "x")
    let b = FilterCatalogEntry(key: "new_homes", displayName: "New homes", description: "x")

    #expect(a == b)
  }

  @Test func equality_differentKey_areNotEqual() {
    let a = FilterCatalogEntry(key: "new_homes", displayName: "New homes", description: "x")
    let b = FilterCatalogEntry(key: "house_builder", displayName: "New homes", description: "x")

    #expect(a != b)
  }
}
