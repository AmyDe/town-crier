/// A UK local authority responsible for planning decisions.
public struct LocalAuthority: Equatable, Hashable, Sendable {
    public let code: String
    public let name: String

    public init(code: String, name: String) {
        self.code = code
        self.name = name
    }
}
