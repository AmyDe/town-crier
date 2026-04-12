import Combine
import TownCrierDomain

/// ViewModel managing subscription product display, purchasing, and restoration.
///
/// After a successful purchase or restore, refreshes the Auth0 token so the
/// `subscription_tier` JWT claim reflects the new tier.
/// Token refresh is best-effort — a failure does not affect the purchase outcome.
@MainActor
public final class SubscriptionViewModel: ObservableObject, ErrorHandlingViewModel {
  @Published public private(set) var products: [SubscriptionProduct] = []
  @Published public private(set) var isLoading = false
  @Published public private(set) var isPurchasing = false
  @Published public private(set) var isRestoring = false
  @Published public internal(set) var error: DomainError?
  @Published public private(set) var currentEntitlement: SubscriptionEntitlement?

  private let subscriptionService: SubscriptionService
  private let authenticationService: AuthenticationService

  public var isSubscribed: Bool {
    currentEntitlement != nil
  }

  public init(
    subscriptionService: SubscriptionService,
    authenticationService: AuthenticationService
  ) {
    self.subscriptionService = subscriptionService
    self.authenticationService = authenticationService
  }

  /// Loads available subscription products and current entitlement.
  public func loadProducts() async {
    isLoading = true
    error = nil
    do {
      products = try await subscriptionService.availableProducts()
      currentEntitlement = await subscriptionService.currentEntitlement()
    } catch {
      handleError(error)
    }
    isLoading = false
  }

  /// Initiates purchase of the given product.
  /// On success, refreshes the auth token so the JWT `subscription_tier` claim
  /// reflects the new tier. Token refresh is best-effort — failure does not
  /// affect the purchase outcome.
  public func purchase(productId: String) async {
    isPurchasing = true
    error = nil
    do {
      currentEntitlement = try await subscriptionService.purchase(productId)
      await refreshAuthSession()
    } catch DomainError.purchaseCancelled {
      // User cancelled — not an error
    } catch {
      handleError(error) { .purchaseFailed($0) }
    }
    isPurchasing = false
  }

  /// Restores previously purchased subscriptions (App Store requirement).
  /// On success with an active entitlement, refreshes the auth token so the JWT
  /// `subscription_tier` claim reflects the restored tier.
  public func restorePurchases() async {
    isRestoring = true
    error = nil
    do {
      let entitlement = try await subscriptionService.restorePurchases()
      currentEntitlement = entitlement
      if entitlement != nil {
        await refreshAuthSession()
      }
    } catch {
      handleError(error) { .restoreFailed($0) }
    }
    isRestoring = false
  }

  // MARK: - Token refresh

  /// Refreshes the auth session to pick up an updated `subscription_tier` claim.
  /// Best-effort: failure is silently absorbed so the purchase/restore is not affected.
  private func refreshAuthSession() async {
    _ = try? await authenticationService.refreshSession()
  }

  /// Returns subscription disclosure text for App Store compliance.
  public func subscriptionDisclosure(for product: SubscriptionProduct) -> String {
    var disclosure = """
      Your subscription will automatically renew at \
      \(product.displayPrice)/month unless cancelled at least \
      24 hours before the end of the current period. You can \
      manage or cancel your subscription in your App Store settings.
      """

    if product.hasFreeTrial {
      disclosure = """
        Start with a \(product.trialDays)-day free trial. \
        After the trial, your subscription will automatically \
        renew at \(product.displayPrice)/month unless cancelled \
        at least 24 hours before the end of the current period. \
        You can manage or cancel your subscription in your \
        App Store settings.
        """
    }

    return disclosure
  }
}
