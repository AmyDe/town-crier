/// ViewModel for the home screen placeholder.
/// All properties are static after initialisation, so this is a plain value type.
public struct HomeViewModel: Sendable {
  public let title: String
  public let subtitle: String

  public init() {
    title = "Town Crier"
    subtitle = "Planning applications near you"
  }
}
