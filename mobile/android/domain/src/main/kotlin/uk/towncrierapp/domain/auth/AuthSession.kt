package uk.towncrierapp.domain.auth

import uk.towncrierapp.domain.subscriptions.SubscriptionTier
import java.time.Clock
import java.time.Instant

/** An authenticated user session: tokens, identity, and the tier read off the access token's JWT claim. */
public data class AuthSession(
    val accessToken: String,
    val idToken: String,
    val expiresAt: Instant,
    val userProfile: UserProfile,
    val subscriptionTier: SubscriptionTier = SubscriptionTier.FREE,
) {
    /** Whether the access token has expired as of [clock] (injected — domain code never reads the wall clock itself). */
    public fun isExpired(clock: Clock): Boolean = !expiresAt.isAfter(clock.instant())
}
