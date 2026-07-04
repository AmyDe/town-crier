package uk.towncrierapp.presentation.reviewprompt

import kotlinx.coroutines.test.runTest
import org.junit.jupiter.api.Test
import uk.towncrierapp.domain.reviewprompt.FakeReviewPromptStore
import uk.towncrierapp.domain.reviewprompt.FakeReviewRequester
import uk.towncrierapp.domain.reviewprompt.ReviewPromptPolicy
import uk.towncrierapp.domain.reviewprompt.ReviewSignal
import uk.towncrierapp.domain.reviewprompt.aReviewPromptState
import java.time.Clock
import java.time.Duration
import java.time.Instant
import java.time.ZoneOffset
import kotlin.test.assertEquals
import kotlin.test.assertNotNull
import kotlin.test.assertNull

/**
 * Unit tests for the `:presentation` glue over the pure [ReviewPromptPolicy]
 * (GH #628) — port of iOS `ReviewPromptTrackerTests`. The policy's own
 * weights/guards are exhaustively covered by `ReviewPromptPolicyTest`; these
 * tests cover only the tracker's own responsibilities: load/save
 * orchestration, firing the requester, first-launch establishment, and
 * session suppression.
 */
class ReviewPromptTrackerTest {
    private val reference: Instant = Instant.ofEpochSecond(1_700_000_000)
    private val clock: Clock = Clock.fixed(reference, ZoneOffset.UTC)

    private fun makeSut(
        store: FakeReviewPromptStore = FakeReviewPromptStore(),
        requester: FakeReviewRequester = FakeReviewRequester(),
    ) = ReviewPromptTracker(store, requester, clock, ReviewPromptPolicy(clock))

    @Test
    fun `recordSignal persists the updated state to the store`() =
        runTest {
            val store = FakeReviewPromptStore(aReviewPromptState(firstLaunchDate = reference))
            val sut = makeSut(store = store)

            sut.recordSignal(ReviewSignal.TappedPortal)

            assertEquals(3, store.state.engagementScore)
        }

    @Test
    fun `recordSignal requests a review when the policy decides to fire`() =
        runTest {
            val store =
                FakeReviewPromptStore(
                    aReviewPromptState(firstLaunchDate = reference.minus(Duration.ofDays(30)), engagementScore = 3),
                )
            val requester = FakeReviewRequester()
            val sut = makeSut(store = store, requester = requester)

            sut.recordSignal(ReviewSignal.TappedPortal)

            assertEquals(1, requester.requestReviewCalls.size)
            assertEquals(0, store.state.engagementScore)
        }

    @Test
    fun `recordSignal does not request a review when the policy holds`() =
        runTest {
            val store = FakeReviewPromptStore(aReviewPromptState(firstLaunchDate = reference))
            val requester = FakeReviewRequester()
            val sut = makeSut(store = store, requester = requester)

            sut.recordSignal(ReviewSignal.TappedPortal)

            assertEquals(0, requester.requestReviewCalls.size)
        }

    @Test
    fun `the first-launch date is established on the very first call when absent`() =
        runTest {
            val store = FakeReviewPromptStore(aReviewPromptState(firstLaunchDate = null))
            val sut = makeSut(store = store)

            sut.recordSignal(ReviewSignal.TappedPortal)

            assertEquals(reference, store.state.firstLaunchDate)
        }

    @Test
    fun `the first-launch date is left unchanged once already set`() =
        runTest {
            val original = reference.minus(Duration.ofDays(100))
            val store = FakeReviewPromptStore(aReviewPromptState(firstLaunchDate = original))
            val sut = makeSut(store = store)

            sut.recordSignal(ReviewSignal.TappedPortal)

            assertEquals(original, store.state.firstLaunchDate)
        }

    @Test
    fun `suppressThisSession prevents a fire even once the score reaches threshold`() =
        runTest {
            val store =
                FakeReviewPromptStore(
                    aReviewPromptState(firstLaunchDate = reference.minus(Duration.ofDays(30)), engagementScore = 3),
                )
            val requester = FakeReviewRequester()
            val sut = makeSut(store = store, requester = requester)

            sut.suppressThisSession()
            sut.recordSignal(ReviewSignal.TappedPortal)

            assertEquals(0, requester.requestReviewCalls.size)
            assertEquals(6, store.state.engagementScore) // suppression does not stop accrual
        }

    @Test
    fun `recordAppForegrounded forwards to the ActiveDay signal with the given reactivation flag`() =
        runTest {
            val store =
                FakeReviewPromptStore(
                    aReviewPromptState(firstLaunchDate = reference, distinctActiveDays = 0, lastActiveDayKey = null),
                )
            val sut = makeSut(store = store)

            sut.recordAppForegrounded(isReactivation = true)

            assertEquals(1, store.state.distinctActiveDays)
            assertNotNull(store.state.lastActiveDayKey)
        }

    @Test
    fun `a fresh store with no prior state still resolves a first-launch date`() =
        runTest {
            val store = FakeReviewPromptStore()
            assertNull(store.state.firstLaunchDate)
            val sut = makeSut(store = store)

            sut.recordSignal(ReviewSignal.OpenedAlert)

            assertEquals(reference, store.state.firstLaunchDate)
        }
}
