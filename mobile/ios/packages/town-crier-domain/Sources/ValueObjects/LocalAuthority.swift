/// A UK local authority responsible for planning decisions.
public struct LocalAuthority: Equatable, Hashable, Sendable {
  public let code: String
  public let name: String
  public let areaType: String?

  public init(code: String, name: String, areaType: String? = nil) {
    self.code = code
    self.name = name
    self.areaType = areaType
  }
}
