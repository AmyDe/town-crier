import Foundation
import TownCrierDomain

extension SubscriptionProduct {
    static let personal = SubscriptionProduct(
        id: "uk.co.towncrier.personal.monthly",
        displayName: "Personal",
        displayPrice: "£1.99",
        tier: .personal,
        hasFreeTrial: true,
        trialDays: 7
    )

    static let pro = SubscriptionProduct(
        id: "uk.co.towncrier.pro.monthly",
        displayName: "Pro",
        displayPrice: "£5.99",
        tier: .pro
    )
}

extension SubscriptionEntitlement {
    static let personalActive = SubscriptionEntitlement(
        tier: .personal,
        expiryDate: Date.distantFuture
    )

    static let proActive = SubscriptionEntitlement(
        tier: .pro,
        expiryDate: Date.distantFuture
    )

    static let personalTrial = SubscriptionEntitlement(
        tier: .personal,
        expiryDate: Date.distantFuture,
        isTrialPeriod: true
    )

    static let expired = SubscriptionEntitlement(
        tier: .personal,
        expiryDate: Date.distantPast
    )
}
