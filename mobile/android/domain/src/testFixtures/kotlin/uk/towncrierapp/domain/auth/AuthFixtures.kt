package uk.towncrierapp.domain.auth

import uk.towncrierapp.domain.subscriptions.SubscriptionTier
import java.time.Instant

/** Fixture factory for [AuthSession] — override only what a test cares about. */
public fun anAuthSession(
    accessToken: String = "access-token",
    idToken: String = "id-token",
    expiresAt: Instant = Instant.parse("2026-07-20T15:00:00Z"),
    userProfile: UserProfile = aUserProfile(),
    subscriptionTier: SubscriptionTier = SubscriptionTier.FREE,
): AuthSession =
    AuthSession(
        accessToken = accessToken,
        idToken = idToken,
        expiresAt = expiresAt,
        userProfile = userProfile,
        subscriptionTier = subscriptionTier,
    )

/** Fixture factory for [UserProfile]. */
public fun aUserProfile(
    userId: String = "auth0|user-1",
    email: String = "resident@example.test",
    name: String? = "Resident",
): UserProfile = UserProfile(userId = userId, email = email, name = name)
