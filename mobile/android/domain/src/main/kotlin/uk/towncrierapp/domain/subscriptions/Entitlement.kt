package uk.towncrierapp.domain.subscriptions

/**
 * A feature entitlement a subscription tier may grant. Used to parameterise
 * UI gating decisions and the subscription upsell surfaces. Must remain in
 * sync with the API's `EntitlementMap`. Port of iOS `Entitlement`.
 */
public enum class Entitlement(
    public val displayName: String,
    public val featureDescription: String,
) {
    STATUS_CHANGE_ALERTS(
        displayName = "Status Change Alerts",
        featureDescription = "Get notified when a planning application in your watch zone changes status.",
    ),
    DECISION_UPDATE_ALERTS(
        displayName = "Decision Update Alerts",
        featureDescription = "Get notified when a decision is made on a planning application near you.",
    ),
    HOURLY_DIGEST_EMAILS(
        displayName = "Hourly Digest Emails",
        featureDescription = "Receive an hourly email digest summarising new planning activity in your watch zones.",
    ),
}
