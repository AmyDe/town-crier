package uk.towncrierapp.domain.profile

import uk.towncrierapp.domain.subscriptions.SubscriptionTier
import kotlin.test.Test
import kotlin.test.assertEquals

/**
 * [UserPreferences] is the exact `PATCH /v1/me` body shape — always all five
 * fields (epic #770 pre-resolved decision: never a partial update).
 * [ServerProfile.preferences] is the starting point a setter copies from
 * when only one field is changing.
 */
class UserPreferencesTest {
    @Test
    fun `ServerProfile defaults match the server's DefaultPreferences (push-on, Monday, all channels on)`() {
        val profile = ServerProfile(userId = "auth0|1", pushEnabled = true, tier = SubscriptionTier.FREE)

        assertEquals(DigestDay.MONDAY, profile.digestDay)
        assertEquals(true, profile.emailDigestEnabled)
        assertEquals(true, profile.savedDecisionPush)
        assertEquals(true, profile.savedDecisionEmail)
    }

    @Test
    fun `preferences extracts the full five-field set currently in effect for the profile`() {
        val profile =
            ServerProfile(
                userId = "auth0|1",
                pushEnabled = false,
                tier = SubscriptionTier.PERSONAL,
                digestDay = DigestDay.FRIDAY,
                emailDigestEnabled = false,
                savedDecisionPush = true,
                savedDecisionEmail = false,
            )

        assertEquals(
            UserPreferences(
                pushEnabled = false,
                digestDay = DigestDay.FRIDAY,
                emailDigestEnabled = false,
                savedDecisionPush = true,
                savedDecisionEmail = false,
            ),
            profile.preferences,
        )
    }
}
