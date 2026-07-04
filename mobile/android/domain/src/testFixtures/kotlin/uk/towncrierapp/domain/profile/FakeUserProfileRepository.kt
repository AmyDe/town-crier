package uk.towncrierapp.domain.profile

import uk.towncrierapp.domain.subscriptions.SubscriptionTier

/** Fixture factory for [ServerProfile]. */
public fun aServerProfile(
    userId: String = "auth0|user-1",
    pushEnabled: Boolean = false,
    tier: SubscriptionTier = SubscriptionTier.FREE,
    digestDay: DigestDay = DigestDay.MONDAY,
    emailDigestEnabled: Boolean = true,
    savedDecisionPush: Boolean = true,
    savedDecisionEmail: Boolean = true,
): ServerProfile =
    ServerProfile(
        userId = userId,
        pushEnabled = pushEnabled,
        tier = tier,
        digestDay = digestDay,
        emailDigestEnabled = emailDigestEnabled,
        savedDecisionPush = savedDecisionPush,
        savedDecisionEmail = savedDecisionEmail,
    )

/** Fixture factory for [UserPreferences]. */
public fun aUserPreferences(
    pushEnabled: Boolean = true,
    digestDay: DigestDay = DigestDay.MONDAY,
    emailDigestEnabled: Boolean = true,
    savedDecisionPush: Boolean = true,
    savedDecisionEmail: Boolean = true,
): UserPreferences =
    UserPreferences(
        pushEnabled = pushEnabled,
        digestDay = digestDay,
        emailDigestEnabled = emailDigestEnabled,
        savedDecisionPush = savedDecisionPush,
        savedDecisionEmail = savedDecisionEmail,
    )

/** Hand-written fake for [UserProfileRepository]. */
public class FakeUserProfileRepository(
    public var ensureProfileResult: Result<ServerProfile> = Result.success(aServerProfile()),
    public var fetchProfileResult: Result<ServerProfile?> = Result.success(aServerProfile()),
    public var updatePreferencesResult: Result<ServerProfile> = Result.success(aServerProfile()),
    public var deleteAccountResult: Result<Unit> = Result.success(Unit),
    public var exportDataResult: Result<ByteArray> = Result.success(ByteArray(0)),
) : UserProfileRepository {
    public val ensureProfileCalls: MutableList<Unit> = mutableListOf()
    public val fetchProfileCalls: MutableList<Unit> = mutableListOf()
    public val updatePreferencesCalls: MutableList<UserPreferences> = mutableListOf()
    public val deleteAccountCalls: MutableList<Unit> = mutableListOf()
    public val exportDataCalls: MutableList<Unit> = mutableListOf()

    override suspend fun ensureProfile(): ServerProfile {
        ensureProfileCalls += Unit
        return ensureProfileResult.getOrThrow()
    }

    override suspend fun fetchProfile(): ServerProfile? {
        fetchProfileCalls += Unit
        return fetchProfileResult.getOrThrow()
    }

    override suspend fun updatePreferences(preferences: UserPreferences): ServerProfile {
        updatePreferencesCalls += preferences
        return updatePreferencesResult.getOrThrow()
    }

    override suspend fun deleteAccount() {
        deleteAccountCalls += Unit
        deleteAccountResult.getOrThrow()
    }

    override suspend fun exportData(): ByteArray {
        exportDataCalls += Unit
        return exportDataResult.getOrThrow()
    }
}
