import Combine
import TownCrierDomain

/// ViewModel driving the map view with planning application pins.
@MainActor
public final class MapViewModel: ObservableObject, ErrorHandlingViewModel {
    @Published private(set) var annotations: [MapAnnotationItem] = []
    @Published private(set) var isLoading = false
    @Published var error: DomainError?
    @Published private(set) var selectedApplication: PlanningApplication?
    @Published private(set) var hasLoaded = false

    let centreLat: Double
    let centreLon: Double
    let radiusMetres: Double

    private let repository: PlanningApplicationRepository?
    private let offlineRepository: OfflineAwareRepository?
    private let watchZone: WatchZone
    private var applications: [PlanningApplication] = []

    public var isEmpty: Bool {
        hasLoaded && annotations.isEmpty && error == nil && !isLoading
    }

    public var isNetworkError: Bool {
        error == .networkUnavailable
    }

    public var isSessionExpired: Bool {
        error == .sessionExpired
    }

    var onApplicationSelected: ((PlanningApplicationId) -> Void)?

    public init(repository: PlanningApplicationRepository, watchZone: WatchZone) {
        self.repository = repository
        self.offlineRepository = nil
        self.watchZone = watchZone
        self.centreLat = watchZone.centre.latitude
        self.centreLon = watchZone.centre.longitude
        self.radiusMetres = watchZone.radiusMetres
    }

    public init(offlineRepository: OfflineAwareRepository, watchZone: WatchZone) {
        self.repository = nil
        self.offlineRepository = offlineRepository
        self.watchZone = watchZone
        self.centreLat = watchZone.centre.latitude
        self.centreLon = watchZone.centre.longitude
        self.radiusMetres = watchZone.radiusMetres
    }

    public func loadApplications() async {
        isLoading = true
        error = nil
        do {
            let fetched: [PlanningApplication]
            if let offlineRepository {
                let entry = try await offlineRepository.fetchApplications(for: LocalAuthority(code: "", name: ""))
                fetched = entry.data
            } else if let repository {
                fetched = try await repository.fetchApplications(for: LocalAuthority(code: "", name: ""))
            } else {
                fetched = []
            }
            applications = fetched
            annotations = fetched.compactMap { app in
                guard let location = app.location else { return nil }
                return MapAnnotationItem(application: app, coordinate: location)
            }
        } catch {
            handleError(error)
        }
        isLoading = false
        hasLoaded = true
    }

    public func selectApplication(_ id: PlanningApplicationId) {
        selectedApplication = applications.first { $0.id == id }
        onApplicationSelected?(id)
    }

    public func clearSelection() {
        selectedApplication = nil
    }
}
