package uk.towncrierapp.domain.subscriptions

/**
 * Subscription level determining feature access. Declaration order is
 * significant: enum ordinal gives `Comparable` for free, so `maxOf(a, b)`
 * always picks the richer tier when merging multiple sources (server
 * profile, cached value, JWT claim — see [resolveTier]).
 */
public enum class SubscriptionTier(
    public val wireValue: String,
) {
    FREE("Free"),
    PERSONAL("Personal"),
    PRO("Pro"),
    ;

    public companion object {
        /**
         * Parses a tier from its wire string. Case-insensitive so it accepts
         * both the server's PascalCase vocabulary ("Personal") and the JWT
         * `subscription_tier` claim's lowercase spelling ("personal").
         * Returns `null` for anything unrecognised — callers decide the
         * fallback (never silently default to FREE here, see tc-exg6).
         */
        public fun fromWireValue(value: String): SubscriptionTier? = entries.firstOrNull { it.wireValue.equals(value, ignoreCase = true) }
    }
}
