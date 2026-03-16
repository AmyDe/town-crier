import Combine
import TownCrierDomain

/// ViewModel for the home screen placeholder.
@MainActor
public final class HomeViewModel: ObservableObject {
    @Published private(set) var title: String
    @Published private(set) var subtitle: String

    public init() {
        title = "Town Crier"
        subtitle = "Planning applications near you"
    }
}
