package uk.towncrierapp.presentation.features.notificationprefs

import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import uk.towncrierapp.domain.auth.DomainError
import uk.towncrierapp.domain.profile.DigestDay
import uk.towncrierapp.domain.profile.FakeUserProfileRepository
import uk.towncrierapp.domain.profile.UserPreferences
import uk.towncrierapp.domain.profile.aServerProfile
import uk.towncrierapp.presentation.MainDispatcherExtension
import kotlin.test.assertEquals

/**
 * Port of iOS `NotificationPreferencesViewModelTests`: global toggles for
 * `savedDecisionPush`/`savedDecisionEmail`/`emailDigestEnabled`/`digestDay`,
 * optimistic-update-with-rollback, every setter PATCHing the full five-field
 * body. Loads via `GET /v1/me` ([uk.towncrierapp.domain.profile.UserProfileRepository.fetchProfile])
 * rather than the `POST /v1/me` ensure-profile call, since only GET/PATCH
 * return the user's actual saved preferences on the wire (see KEY DECISIONS,
 * tc-4jjw) — this is what makes a real round-trip (`toggle → reload →
 * see the persisted value`) actually work.
 */
@ExtendWith(MainDispatcherExtension::class)
class NotificationPreferencesViewModelTest {
    private fun aProfile(
        pushEnabled: Boolean = true,
        digestDay: DigestDay = DigestDay.MONDAY,
        emailDigestEnabled: Boolean = true,
        savedDecisionPush: Boolean = true,
        savedDecisionEmail: Boolean = true,
    ) = aServerProfile(
        pushEnabled = pushEnabled,
        digestDay = digestDay,
        emailDigestEnabled = emailDigestEnabled,
        savedDecisionPush = savedDecisionPush,
        savedDecisionEmail = savedDecisionEmail,
    )

    // region Load

    @Test
    fun `load populates fields from the fetched profile`() {
        val repository =
            FakeUserProfileRepository(
                fetchProfileResult =
                    Result.success(
                        aProfile(
                            digestDay = DigestDay.FRIDAY,
                            emailDigestEnabled = false,
                            savedDecisionPush = false,
                            savedDecisionEmail = true,
                        ),
                    ),
            )
        val viewModel = NotificationPreferencesViewModel(repository)

        viewModel.load()

        val state = viewModel.uiState.value
        assertEquals(false, state.savedDecisionPush)
        assertEquals(true, state.savedDecisionEmail)
        assertEquals(false, state.emailDigestEnabled)
        assertEquals(DigestDay.FRIDAY, state.digestDay)
        assertEquals(false, state.isLoading)
    }

    @Test
    fun `load falls back to defaults when fetchProfile throws`() {
        val repository = FakeUserProfileRepository(fetchProfileResult = Result.failure(DomainError.NetworkUnavailable))
        val viewModel = NotificationPreferencesViewModel(repository)

        viewModel.load()

        val state = viewModel.uiState.value
        assertEquals(true, state.savedDecisionPush)
        assertEquals(true, state.savedDecisionEmail)
        assertEquals(true, state.emailDigestEnabled)
        assertEquals(DigestDay.MONDAY, state.digestDay)
        assertEquals(false, state.isLoading)
    }

    @Test
    fun `load falls back to defaults when no profile exists yet`() {
        val repository = FakeUserProfileRepository(fetchProfileResult = Result.success(null))
        val viewModel = NotificationPreferencesViewModel(repository)

        viewModel.load()

        assertEquals(true, viewModel.uiState.value.savedDecisionPush)
    }

    // endregion

    // region Setters — full five-field round trip

    @Test
    fun `setSavedDecisionPush PATCHes the full five-field body and round-trips other fields`() {
        val initial =
            aProfile(pushEnabled = true, digestDay = DigestDay.FRIDAY, emailDigestEnabled = false, savedDecisionPush = true, savedDecisionEmail = true)
        val repository = FakeUserProfileRepository(fetchProfileResult = Result.success(initial))
        repository.updatePreferencesResult =
            Result.success(initial.copy(savedDecisionPush = false))
        val viewModel = NotificationPreferencesViewModel(repository)
        viewModel.load()

        viewModel.setSavedDecisionPush(false)

        assertEquals(1, repository.updatePreferencesCalls.size)
        assertEquals(
            UserPreferences(
                pushEnabled = true,
                digestDay = DigestDay.FRIDAY,
                emailDigestEnabled = false,
                savedDecisionPush = false,
                savedDecisionEmail = true,
            ),
            repository.updatePreferencesCalls.single(),
        )
        assertEquals(false, viewModel.uiState.value.savedDecisionPush)
    }

    @Test
    fun `setSavedDecisionEmail PATCHes the full five-field body and round-trips other fields`() {
        val initial =
            aProfile(pushEnabled = false, digestDay = DigestDay.WEDNESDAY, emailDigestEnabled = true, savedDecisionPush = true, savedDecisionEmail = true)
        val repository = FakeUserProfileRepository(fetchProfileResult = Result.success(initial))
        repository.updatePreferencesResult = Result.success(initial.copy(savedDecisionEmail = false))
        val viewModel = NotificationPreferencesViewModel(repository)
        viewModel.load()

        viewModel.setSavedDecisionEmail(false)

        assertEquals(
            UserPreferences(
                pushEnabled = false,
                digestDay = DigestDay.WEDNESDAY,
                emailDigestEnabled = true,
                savedDecisionPush = true,
                savedDecisionEmail = false,
            ),
            repository.updatePreferencesCalls.single(),
        )
        assertEquals(false, viewModel.uiState.value.savedDecisionEmail)
    }

    @Test
    fun `setEmailDigestEnabled PATCHes the full five-field body and round-trips other fields`() {
        val initial =
            aProfile(pushEnabled = true, digestDay = DigestDay.TUESDAY, emailDigestEnabled = true, savedDecisionPush = false, savedDecisionEmail = true)
        val repository = FakeUserProfileRepository(fetchProfileResult = Result.success(initial))
        repository.updatePreferencesResult = Result.success(initial.copy(emailDigestEnabled = false))
        val viewModel = NotificationPreferencesViewModel(repository)
        viewModel.load()

        viewModel.setEmailDigestEnabled(false)

        assertEquals(
            UserPreferences(
                pushEnabled = true,
                digestDay = DigestDay.TUESDAY,
                emailDigestEnabled = false,
                savedDecisionPush = false,
                savedDecisionEmail = true,
            ),
            repository.updatePreferencesCalls.single(),
        )
        assertEquals(false, viewModel.uiState.value.emailDigestEnabled)
    }

    @Test
    fun `setDigestDay PATCHes the full five-field body and round-trips other fields`() {
        val initial =
            aProfile(pushEnabled = true, digestDay = DigestDay.MONDAY, emailDigestEnabled = true, savedDecisionPush = true, savedDecisionEmail = false)
        val repository = FakeUserProfileRepository(fetchProfileResult = Result.success(initial))
        repository.updatePreferencesResult = Result.success(initial.copy(digestDay = DigestDay.SATURDAY))
        val viewModel = NotificationPreferencesViewModel(repository)
        viewModel.load()

        viewModel.setDigestDay(DigestDay.SATURDAY)

        assertEquals(
            UserPreferences(
                pushEnabled = true,
                digestDay = DigestDay.SATURDAY,
                emailDigestEnabled = true,
                savedDecisionPush = true,
                savedDecisionEmail = false,
            ),
            repository.updatePreferencesCalls.single(),
        )
        assertEquals(DigestDay.SATURDAY, viewModel.uiState.value.digestDay)
    }

    // endregion

    // region Rollback / error

    @Test
    fun `a failed PATCH rolls the toggle back to its prior value`() {
        val initial = aProfile(savedDecisionPush = true)
        val repository = FakeUserProfileRepository(fetchProfileResult = Result.success(initial))
        repository.updatePreferencesResult = Result.failure(DomainError.NetworkUnavailable)
        val viewModel = NotificationPreferencesViewModel(repository)
        viewModel.load()

        viewModel.setSavedDecisionPush(false)

        assertEquals(true, viewModel.uiState.value.savedDecisionPush)
        assertEquals(DomainError.NetworkUnavailable, viewModel.uiState.value.error)
    }

    @Test
    fun `a failed PATCH leaves the other three fields untouched`() {
        val initial =
            aProfile(digestDay = DigestDay.THURSDAY, emailDigestEnabled = false, savedDecisionEmail = false)
        val repository = FakeUserProfileRepository(fetchProfileResult = Result.success(initial))
        repository.updatePreferencesResult = Result.failure(DomainError.NetworkUnavailable)
        val viewModel = NotificationPreferencesViewModel(repository)
        viewModel.load()

        viewModel.setSavedDecisionPush(false)

        val state = viewModel.uiState.value
        assertEquals(DigestDay.THURSDAY, state.digestDay)
        assertEquals(false, state.emailDigestEnabled)
        assertEquals(false, state.savedDecisionEmail)
    }

    // endregion
}
