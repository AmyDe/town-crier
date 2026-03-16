/// The local authority's reference number for a planning application.
public struct ApplicationReference: Equatable, Hashable, Sendable {
    public let value: String

    public init(_ value: String) {
        self.value = value
    }
}
