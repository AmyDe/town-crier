import Combine
import TownCrierDomain

/// ViewModel driving the filterable list of planning applications.
@MainActor
public final class ApplicationListViewModel: ObservableObject {
    @Published private(set) var applications: [PlanningApplication] = []
    @Published var selectedStatusFilter: ApplicationStatus?
    @Published private(set) var isLoading = false
    @Published private(set) var error: DomainError?

    private let repository: PlanningApplicationRepository?
    private let offlineRepository: OfflineAwareRepository?
    private let authority: LocalAuthority
    private let tier: SubscriptionTier

    var onApplicationSelected: ((PlanningApplicationId) -> Void)?

    public var canFilter: Bool {
        tier != .free
    }

    public var filteredApplications: [PlanningApplication] {
        guard canFilter, let filter = selectedStatusFilter else {
            return applications
        }
        return applications.filter { $0.status == filter }
    }

    public var isEmpty: Bool {
        filteredApplications.isEmpty && error == nil && !isLoading
    }

    public var isNetworkError: Bool {
        error == .networkUnavailable
    }

    public var isSessionExpired: Bool {
        error == .sessionExpired
    }

    public init(
        repository: PlanningApplicationRepository,
        authority: LocalAuthority,
        tier: SubscriptionTier = .free
    ) {
        self.repository = repository
        self.offlineRepository = nil
        self.authority = authority
        self.tier = tier
    }

    public init(
        offlineRepository: OfflineAwareRepository,
        authority: LocalAuthority,
        tier: SubscriptionTier = .free
    ) {
        self.repository = nil
        self.offlineRepository = offlineRepository
        self.authority = authority
        self.tier = tier
    }

    public func loadApplications() async {
        isLoading = true
        error = nil
        do {
            let fetched: [PlanningApplication]
            if let offlineRepository {
                let entry = try await offlineRepository.fetchApplications(for: authority)
                fetched = entry.data
            } else if let repository {
                fetched = try await repository.fetchApplications(for: authority)
            } else {
                fetched = []
            }
            applications = fetched.sorted { $0.receivedDate > $1.receivedDate }
        } catch let domainError as DomainError {
            error = domainError
        } catch {
            self.error = .unexpected(error.localizedDescription)
        }
        isLoading = false
    }

    public func selectApplication(_ id: PlanningApplicationId) {
        onApplicationSelected?(id)
    }
}
