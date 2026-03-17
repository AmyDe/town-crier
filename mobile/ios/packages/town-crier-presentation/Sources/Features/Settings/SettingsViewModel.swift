import Combine
import Foundation
import TownCrierDomain

/// ViewModel managing the settings and account screen.
@MainActor
public final class SettingsViewModel: ObservableObject {
    @Published public private(set) var userEmail: String?
    @Published public private(set) var userName: String?
    @Published public private(set) var authMethod: AuthMethod?
    @Published public private(set) var subscriptionTier: SubscriptionTier = .free
    @Published public private(set) var isTrialPeriod = false
    @Published public private(set) var isLoading = false
    @Published public private(set) var error: DomainError?
    @Published public var isShowingDeleteConfirmation = false

    var onLogout: (() -> Void)?

    private let authService: AuthenticationService
    private let subscriptionService: SubscriptionService
    private let appVersionProvider: AppVersionProvider

    public init(
        authService: AuthenticationService,
        subscriptionService: SubscriptionService,
        appVersionProvider: AppVersionProvider
    ) {
        self.authService = authService
        self.subscriptionService = subscriptionService
        self.appVersionProvider = appVersionProvider
    }

    public var appVersion: String {
        "\(appVersionProvider.version) (\(appVersionProvider.buildNumber))"
    }

    public var attributionItems: [AttributionItem] {
        [
            AttributionItem(
                name: "PlanIt",
                detail: "Planning application data",
                url: URL(string: "https://www.planit.org.uk")
            ),
            AttributionItem(
                name: "Crown Copyright",
                detail: "Contains public sector information"
            ),
            AttributionItem(
                name: "Ordnance Survey",
                detail: "Mapping data"
            ),
            AttributionItem(
                name: "OpenStreetMap",
                detail: "Map tiles and geodata",
                url: URL(string: "https://www.openstreetmap.org")
            ),
        ]
    }

    public func load() async {
        isLoading = true
        error = nil

        if let session = await authService.currentSession() {
            userEmail = session.userProfile.email
            userName = session.userProfile.name
            authMethod = session.userProfile.authMethod
        }

        if let entitlement = await subscriptionService.currentEntitlement() {
            subscriptionTier = entitlement.tier
            isTrialPeriod = entitlement.isTrialPeriod
        } else {
            subscriptionTier = .free
            isTrialPeriod = false
        }

        isLoading = false
    }

    public func logout() async {
        error = nil
        do {
            try await authService.logout()
            clearSession()
            onLogout?()
        } catch let domainError as DomainError {
            error = domainError
        } catch {
            self.error = .logoutFailed(error.localizedDescription)
        }
    }

    public func requestAccountDeletion() {
        isShowingDeleteConfirmation = true
    }

    public func cancelDeletion() {
        isShowingDeleteConfirmation = false
    }

    public func confirmDeleteAccount() async {
        isShowingDeleteConfirmation = false
        error = nil
        do {
            try await authService.deleteAccount()
            clearSession()
            onLogout?()
        } catch let domainError as DomainError {
            error = domainError
        } catch {
            self.error = .unexpected(error.localizedDescription)
        }
    }

    private func clearSession() {
        userEmail = nil
        userName = nil
        authMethod = nil
        subscriptionTier = .free
        isTrialPeriod = false
    }
}
