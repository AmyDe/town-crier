import Combine
import TownCrierDomain

/// ViewModel driving the map view with planning application pins.
@MainActor
public final class MapViewModel: ObservableObject {
    @Published private(set) var annotations: [MapAnnotationItem] = []
    @Published private(set) var isLoading = false
    @Published private(set) var error: DomainError?
    @Published private(set) var selectedApplication: PlanningApplication?

    let centreLat: Double
    let centreLon: Double
    let radiusMetres: Double

    private let repository: PlanningApplicationRepository
    private let watchZone: WatchZone
    private var applications: [PlanningApplication] = []

    var onApplicationSelected: ((PlanningApplicationId) -> Void)?

    public init(repository: PlanningApplicationRepository, watchZone: WatchZone) {
        self.repository = repository
        self.watchZone = watchZone
        self.centreLat = watchZone.centre.latitude
        self.centreLon = watchZone.centre.longitude
        self.radiusMetres = watchZone.radiusMetres
    }

    public func loadApplications() async {
        isLoading = true
        error = nil
        do {
            let fetched = try await repository.fetchApplications(for: LocalAuthority(code: "", name: ""))
            applications = fetched
            annotations = fetched.compactMap { app in
                guard let location = app.location else { return nil }
                return MapAnnotationItem(application: app, coordinate: location)
            }
        } catch let domainError as DomainError {
            error = domainError
        } catch {
            self.error = .unexpected(error.localizedDescription)
        }
        isLoading = false
    }

    public func selectApplication(_ id: PlanningApplicationId) {
        selectedApplication = applications.first { $0.id == id }
        onApplicationSelected?(id)
    }

    public func clearSelection() {
        selectedApplication = nil
    }
}
