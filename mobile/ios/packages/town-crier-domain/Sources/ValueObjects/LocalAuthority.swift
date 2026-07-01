/// A UK local authority responsible for planning decisions.
public struct LocalAuthority: Equatable, Hashable, Sendable {
  public let code: String
  public let name: String
  public let areaType: String?
  /// URL-safe slug for the public share surface (e.g. `"kingston"`), as emitted
  /// by the API on the detail/by-slug JSON. `nil` on list/zone payloads, which
  /// omit it. Always supplied by the server — iOS never computes it
  /// (GH #738 Slice 4).
  public let slug: String?

  public init(code: String, name: String, areaType: String? = nil, slug: String? = nil) {
    self.code = code
    self.name = name
    self.areaType = areaType
    self.slug = slug
  }
}
