import Combine
import Foundation
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
  @Published public var selectedProPeriod: SubscriptionPeriod = .annual
  @Published public var isCancelSubscriptionPromptPresented = false
  @Published public var isManageSubscriptionsPresented = false
  @Published public var isTransactionClaimedAlertPresented = false

  /// The legal document currently presented over the paywall, if any.
  /// Drives a `.sheet(item:)` local to `SubscriptionView` so the Privacy Policy
  /// and Terms of Use links required by App Store Guideline 3.1.2(c) open without
  /// routing through the root coordinator (which already owns the paywall sheet).
  @Published public var presentedLegalDocument: LegalDocumentType?

  /// Fired when a purchase fails (not when cancelled or successful). A failed
  /// purchase is a friction moment that suppresses the review prompt (GH #628).
  public var onPurchaseFailed: (() -> Void)?

  private let subscriptionService: SubscriptionService
  private let authenticationService: AuthenticationService
  private let locale: Locale

  public var isSubscribed: Bool {
    currentEntitlement != nil
  }

  public var personalProducts: [SubscriptionProduct] {
    products.filter { $0.tier == .personal }
  }

  /// Pro products ordered monthly, annual, lifetime.
  public var proOptions: [SubscriptionProduct] {
    products.filter { $0.tier == .pro }.sorted { $0.period < $1.period }
  }

  public var selectedProProduct: SubscriptionProduct? {
    proOptions.first { $0.period == selectedProPeriod } ?? proOptions.first
  }

  /// False once the user holds a lifetime entitlement: there is nothing left to buy.
  public var showsPurchaseButtons: Bool {
    currentEntitlement?.isLifetime != true
  }

  /// Whole-number percentage the annual plan saves against twelve monthly payments.
  /// Nil when either plan is unavailable or the saving rounds to less than 1%.
  public var annualSavingsPercent: Int? {
    guard let monthly = proProduct(for: .monthly), let annual = proProduct(for: .annual),
      monthly.price > 0
    else { return nil }
    let yearOfMonthly = monthly.price * 12
    let fraction = (yearOfMonthly - annual.price) / yearOfMonthly * 100
    let percent = Int((Double(fraction.description) ?? 0).rounded())
    return percent >= 1 ? percent : nil
  }

  public var annualMonthlyEquivalent: String? {
    guard let annual = proProduct(for: .annual) else { return nil }
    return (annual.price / 12).formatted(
      .currency(code: annual.currencyCode).locale(locale)
    )
  }

  public var annualSubLine: String? {
    annualMonthlyEquivalent.map { "Works out at \($0) a month" }
  }

  public var annualSavingsBadge: String? {
    annualSavingsPercent.map { "Save \($0)%" }
  }

  public static let cancelPromptTitle = "Cancel your old subscription"
  public static let cancelPromptMessage =
    "You now have lifetime Pro, but your subscription will still renew. "
    + "Apple doesn't cancel it for you, so cancel it now or you'll pay twice."
  public static let cancelPromptManageButtonTitle = "Manage subscription"
  public static let cancelPromptDismissButtonTitle = "Not now"

  /// Shown when a purchase or restore's Apple ID transaction is already
  /// linked to a different Town Crier account. Never names or hints at the
  /// other account -- the app holds no information about it that it is
  /// allowed to show (GH#1165).
  public static let transactionClaimedAlertTitle = "This subscription is on another account"
  public static let transactionClaimedAlertMessage =
    "The subscription on this Apple ID is already linked to a different Town Crier "
    + "account. Sign in with that account to use it, or get in touch and we'll sort it out."
  public static let transactionClaimedAlertButtonTitle = "OK"

  public init(
    subscriptionService: SubscriptionService,
    authenticationService: AuthenticationService,
    locale: Locale = .current
  ) {
    self.subscriptionService = subscriptionService
    self.authenticationService = authenticationService
    self.locale = locale
  }

  /// Loads available subscription products and current entitlement.
  public func loadProducts() async {
    isLoading = true
    error = nil
    do {
      products = try await subscriptionService.availableProducts()
      currentEntitlement = await subscriptionService.currentEntitlement()
      selectedProPeriod = initialProPeriod()
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
      let entitlement = try await subscriptionService.purchase(productId)
      currentEntitlement = entitlement
      await refreshAuthSession()
      if entitlement.isLifetime, await subscriptionService.hasActiveAutoRenewingSubscription() {
        isCancelSubscriptionPromptPresented = true
      }
    } catch DomainError.purchaseCancelled {
      // User cancelled — not an error
    } catch DomainError.transactionAlreadyClaimed {
      isTransactionClaimedAlertPresented = true
      onPurchaseFailed?()
    } catch {
      handleError(error) { .purchaseFailed($0) }
      onPurchaseFailed?()
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
    } catch DomainError.transactionAlreadyClaimed {
      isTransactionClaimedAlertPresented = true
    } catch {
      handleError(error) { .restoreFailed($0) }
    }
    isRestoring = false
  }

  /// Opens Apple's Manage Subscriptions sheet.
  public func showManageSubscriptions() {
    isManageSubscriptionsPresented = true
  }

  /// Presents the given legal document (Privacy Policy or Terms of Use) over the paywall.
  public func showLegalDocument(_ documentType: LegalDocumentType) {
    presentedLegalDocument = documentType
  }

  // MARK: - Token refresh

  /// Refreshes the auth session to pick up an updated `subscription_tier` claim.
  /// Best-effort: failure is silently absorbed so the purchase/restore is not affected.
  private func refreshAuthSession() async {
    _ = try? await authenticationService.refreshSession()
  }

  public func isCurrentProduct(_ product: SubscriptionProduct) -> Bool {
    currentEntitlement?.productId == product.id
  }

  public func priceLine(for product: SubscriptionProduct) -> String {
    switch product.period {
    case .monthly: "\(product.displayPrice)/month"
    case .annual: "\(product.displayPrice)/year"
    case .lifetime: "\(product.displayPrice) once"
    }
  }

  public func purchaseButtonTitle(for product: SubscriptionProduct) -> String {
    switch product.period {
    case .monthly: product.hasFreeTrial ? "Start Free Trial" : "Subscribe"
    case .annual: "Subscribe yearly"
    case .lifetime: "Buy lifetime"
    }
  }

  /// Returns subscription disclosure text for App Store compliance.
  public func subscriptionDisclosure(for product: SubscriptionProduct) -> String {
    switch product.period {
    case .monthly:
      monthlyDisclosure(for: product)
    case .annual:
      """
      Your subscription renews automatically at \(product.displayPrice)/year unless you \
      cancel at least 24 hours before the end of the current period. You can manage or \
      cancel it in your App Store settings.
      """
    case .lifetime:
      """
      One payment of \(product.displayPrice). This is not a subscription and nothing \
      renews. Pro stays on your account for as long as Town Crier runs.
      """
    }
  }

  private func monthlyDisclosure(for product: SubscriptionProduct) -> String {
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

  private func proProduct(for period: SubscriptionPeriod) -> SubscriptionProduct? {
    proOptions.first { $0.period == period }
  }

  private func initialProPeriod() -> SubscriptionPeriod {
    let preferred: SubscriptionPeriod
    if let entitlement = currentEntitlement, entitlement.tier == .pro {
      if entitlement.isLifetime {
        preferred = .lifetime
      } else {
        preferred = proOptions.first { $0.id == entitlement.productId }?.period ?? .annual
      }
    } else {
      preferred = .annual
    }
    guard let first = proOptions.first, !proOptions.contains(where: { $0.period == preferred })
    else { return preferred }
    return first.period
  }
}
