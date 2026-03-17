import Combine
import TownCrierDomain

/// ViewModel managing subscription product display, purchasing, and restoration.
@MainActor
public final class SubscriptionViewModel: ObservableObject {
    @Published public private(set) var products: [SubscriptionProduct] = []
    @Published public private(set) var isLoading = false
    @Published public private(set) var isPurchasing = false
    @Published public private(set) var isRestoring = false
    @Published public private(set) var error: DomainError?
    @Published public private(set) var currentEntitlement: SubscriptionEntitlement?

    private let subscriptionService: SubscriptionService

    public var isSubscribed: Bool {
        currentEntitlement != nil
    }

    public init(subscriptionService: SubscriptionService) {
        self.subscriptionService = subscriptionService
    }

    /// Loads available subscription products and current entitlement.
    public func loadProducts() async {
        isLoading = true
        error = nil
        do {
            products = try await subscriptionService.availableProducts()
            currentEntitlement = await subscriptionService.currentEntitlement()
        } catch let domainError as DomainError {
            error = domainError
        } catch {
            self.error = .unexpected(error.localizedDescription)
        }
        isLoading = false
    }

    /// Initiates purchase of the given product.
    public func purchase(productId: String) async {
        isPurchasing = true
        error = nil
        do {
            currentEntitlement = try await subscriptionService.purchase(productId)
        } catch DomainError.purchaseCancelled {
            // User cancelled — not an error
        } catch let domainError as DomainError {
            error = domainError
        } catch {
            self.error = .purchaseFailed(error.localizedDescription)
        }
        isPurchasing = false
    }

    /// Restores previously purchased subscriptions (App Store requirement).
    public func restorePurchases() async {
        isRestoring = true
        error = nil
        do {
            currentEntitlement = try await subscriptionService.restorePurchases()
        } catch let domainError as DomainError {
            error = domainError
        } catch {
            self.error = .restoreFailed(error.localizedDescription)
        }
        isRestoring = false
    }

    /// Returns subscription disclosure text for App Store compliance.
    public func subscriptionDisclosure(for product: SubscriptionProduct) -> String {
        var disclosure =
            "Your subscription will automatically renew at \(product.displayPrice)/month unless cancelled at least 24 hours before the end of the current period. You can manage or cancel your subscription in your App Store settings."

        if product.hasFreeTrial {
            disclosure =
                "Start with a \(product.trialDays)-day free trial. After the trial, your subscription will automatically renew at \(product.displayPrice)/month unless cancelled at least 24 hours before the end of the current period. You can manage or cancel your subscription in your App Store settings."
        }

        return disclosure
    }
}
