package uk.towncrierapp.domain.profile

import uk.towncrierapp.domain.subscriptions.SubscriptionTier

/** Fixture factory for [ServerProfile]. */
public fun aServerProfile(
    userId: String = "auth0|user-1",
    pushEnabled: Boolean = false,
    tier: SubscriptionTier = SubscriptionTier.FREE,
): ServerProfile = ServerProfile(userId = userId, pushEnabled = pushEnabled, tier = tier)

/** Hand-written fake for [UserProfileRepository]. */
public class FakeUserProfileRepository(
    public var ensureProfileResult: Result<ServerProfile> = Result.success(aServerProfile()),
) : UserProfileRepository {
    public val ensureProfileCalls: MutableList<Unit> = mutableListOf()

    override suspend fun ensureProfile(): ServerProfile {
        ensureProfileCalls += Unit
        return ensureProfileResult.getOrThrow()
    }
}
