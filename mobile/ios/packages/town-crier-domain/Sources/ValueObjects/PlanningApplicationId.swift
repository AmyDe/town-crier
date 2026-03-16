/// A unique identifier for a planning application.
public struct PlanningApplicationId: Equatable, Hashable, Sendable {
    public let value: String

    public init(_ value: String) {
        self.value = value
    }
}
