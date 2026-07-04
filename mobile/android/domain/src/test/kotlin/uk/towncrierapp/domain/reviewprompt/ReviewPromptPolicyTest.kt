package uk.towncrierapp.domain.reviewprompt

import java.time.Clock
import java.time.Duration
import java.time.Instant
import java.time.ZoneOffset
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

/**
 * Exhaustive unit tests for the pure [ReviewPromptPolicy] decision engine
 * (GH #628) — a verbatim port of iOS `ReviewPromptPolicyTests`. Every
 * weight, the threshold, and every guard is covered with a deterministic
 * fixed [Clock] so distinct-day counting is reproducible.
 */
class ReviewPromptPolicyTest {
    // 2023-11-14 22:13:20 UTC — a fixed anchor for all time-dependent assertions.
    private val reference: Instant = Instant.ofEpochSecond(1_700_000_000)
    private val day: Duration = Duration.ofDays(1)

    private fun makePolicy(now: Instant = reference): ReviewPromptPolicy = ReviewPromptPolicy(Clock.fixed(now, ZoneOffset.UTC))

    /** A state whose account is comfortably older than the 7-day age guard and never prompted. */
    private fun eligibleState(
        engagementScore: Int = 0,
        saveCount: Int = 0,
        lastActiveDayKey: String? = null,
        distinctActiveDays: Int = 0,
        lastPromptDate: Instant? = null,
        promptTimestamps: List<Instant> = emptyList(),
        hasRecordedUpgrade: Boolean = false,
    ) = ReviewPromptState(
        firstLaunchDate = reference.minus(Duration.ofDays(30)),
        engagementScore = engagementScore,
        saveCount = saveCount,
        lastActiveDayKey = lastActiveDayKey,
        distinctActiveDays = distinctActiveDays,
        lastPromptDate = lastPromptDate,
        promptTimestamps = promptTimestamps,
        hasRecordedUpgrade = hasRecordedUpgrade,
    )

    private fun evaluate(
        signal: ReviewSignal,
        state: ReviewPromptState,
        now: Instant = reference,
        sessionSuppressed: Boolean = false,
        isReactivation: Boolean = false,
    ): ReviewPromptOutcome = makePolicy(now).evaluate(signal, state, sessionSuppressed, isReactivation)

    // region Threshold

    @Test
    fun `holds when the accumulated score stays below the threshold`() {
        val outcome = evaluate(ReviewSignal.TappedPortal, eligibleState(engagementScore = 0))

        assertEquals(ReviewPromptDecision.HOLD, outcome.decision)
        assertEquals(3, outcome.state.engagementScore)
    }

    @Test
    fun `fires when a fire-eligible signal lifts the score to exactly the threshold`() {
        val outcome = evaluate(ReviewSignal.TappedPortal, eligibleState(engagementScore = 3))

        assertEquals(ReviewPromptDecision.FIRE, outcome.decision)
    }

    // endregion

    // region Weights

    @Test
    fun `tapping through to the portal contributes +3`() {
        val outcome = evaluate(ReviewSignal.TappedPortal, eligibleState(engagementScore = 0))
        assertEquals(ReviewPromptPolicy.PORTAL_WEIGHT, outcome.state.engagementScore)
        assertEquals(3, ReviewPromptPolicy.PORTAL_WEIGHT)
    }

    @Test
    fun `opening an instant alert contributes +2`() {
        val outcome = evaluate(ReviewSignal.OpenedAlert, eligibleState(engagementScore = 0))
        assertEquals(ReviewPromptPolicy.OPENED_ALERT_WEIGHT, outcome.state.engagementScore)
        assertEquals(2, ReviewPromptPolicy.OPENED_ALERT_WEIGHT)
    }

    @Test
    fun `a loyalty active day contributes +1`() {
        val outcome =
            evaluate(
                ReviewSignal.ActiveDay,
                eligibleState(engagementScore = 0, lastActiveDayKey = null),
                isReactivation = true,
            )
        assertEquals(ReviewPromptPolicy.ACTIVE_DAY_WEIGHT, outcome.state.engagementScore)
        assertEquals(1, ReviewPromptPolicy.ACTIVE_DAY_WEIGHT)
    }

    @Test
    fun `portal (+3), a 2nd save (+2) and a loyalty day (+1) reach 6 and fire`() {
        val state =
            eligibleState(
                engagementScore = 5,
                saveCount = 2,
                lastActiveDayKey = "2023-11-14",
                distinctActiveDays = 2,
            )

        val outcome =
            evaluate(ReviewSignal.ActiveDay, state, now = reference.plus(day), isReactivation = true)

        assertEquals(ReviewPromptDecision.FIRE, outcome.decision)
    }

    // endregion

    // region Upgrade — score contributor only, latched

    @Test
    fun `upgrading contributes +2 but never fires (weight less than threshold invariant)`() {
        assertTrue(ReviewPromptPolicy.UPGRADE_WEIGHT < ReviewPromptPolicy.ENGAGEMENT_THRESHOLD)

        val outcome = evaluate(ReviewSignal.Upgraded, eligibleState(engagementScore = 0))

        assertEquals(ReviewPromptDecision.HOLD, outcome.decision)
        assertEquals(2, outcome.state.engagementScore)
    }

    @Test
    fun `upgrade is latched — recording it twice still only adds +2 once`() {
        val first = evaluate(ReviewSignal.Upgraded, eligibleState(engagementScore = 0))
        assertEquals(2, first.state.engagementScore)
        assertTrue(first.state.hasRecordedUpgrade)

        val second = evaluate(ReviewSignal.Upgraded, first.state)
        assertEquals(ReviewPromptDecision.HOLD, second.decision)
        assertEquals(2, second.state.engagementScore)
    }

    // endregion

    // region Saves — first save not counted

    @Test
    fun `the first save increments the count but contributes no score and never fires`() {
        val outcome = evaluate(ReviewSignal.SavedApplication, eligibleState(engagementScore = 5, saveCount = 0))

        assertEquals(ReviewPromptDecision.HOLD, outcome.decision)
        assertEquals(1, outcome.state.saveCount)
        assertEquals(5, outcome.state.engagementScore)
    }

    @Test
    fun `the second save contributes +2 and is fire-eligible`() {
        val outcome = evaluate(ReviewSignal.SavedApplication, eligibleState(engagementScore = 4, saveCount = 1))

        assertEquals(ReviewPromptDecision.FIRE, outcome.decision)
        assertEquals(2, outcome.state.saveCount)
    }

    // endregion

    // region Account-age guard

    @Test
    fun `never fires before the account is 7 days old, regardless of score`() {
        val state = ReviewPromptState(firstLaunchDate = reference.minus(Duration.ofDays(6)), engagementScore = 3)

        val outcome = evaluate(ReviewSignal.TappedPortal, state)

        assertEquals(ReviewPromptDecision.HOLD, outcome.decision)
        assertEquals(6, outcome.state.engagementScore) // score still accrues
    }

    @Test
    fun `fires once the account reaches 7 days old`() {
        val state = ReviewPromptState(firstLaunchDate = reference.minus(Duration.ofDays(7)), engagementScore = 3)

        val outcome = evaluate(ReviewSignal.TappedPortal, state)

        assertEquals(ReviewPromptDecision.FIRE, outcome.decision)
    }

    @Test
    fun `never fires when the first-launch date is unknown`() {
        val state = ReviewPromptState(firstLaunchDate = null, engagementScore = 3)

        val outcome = evaluate(ReviewSignal.TappedPortal, state)

        assertEquals(ReviewPromptDecision.HOLD, outcome.decision)
    }

    // endregion

    // region Cooldown guard

    @Test
    fun `never fires within 120 days of the last prompt`() {
        val state = eligibleState(engagementScore = 3, lastPromptDate = reference.minus(Duration.ofDays(119)))

        val outcome = evaluate(ReviewSignal.TappedPortal, state)

        assertEquals(ReviewPromptDecision.HOLD, outcome.decision)
    }

    @Test
    fun `fires once 120 days have elapsed since the last prompt`() {
        val state = eligibleState(engagementScore = 3, lastPromptDate = reference.minus(Duration.ofDays(120)))

        val outcome = evaluate(ReviewSignal.TappedPortal, state)

        assertEquals(ReviewPromptDecision.FIRE, outcome.decision)
    }

    @Test
    fun `holds when the clock has moved backwards past the last prompt date`() {
        val state = eligibleState(engagementScore = 3, lastPromptDate = reference.plus(Duration.ofDays(10)))

        val outcome = evaluate(ReviewSignal.TappedPortal, state)

        assertEquals(ReviewPromptDecision.HOLD, outcome.decision)
    }

    // endregion

    // region Annual cap guard

    @Test
    fun `never fires when 3 prompts already fall within the trailing 365 days`() {
        val state =
            eligibleState(
                engagementScore = 3,
                lastPromptDate = reference.minus(Duration.ofDays(200)),
                promptTimestamps =
                    listOf(
                        reference.minus(Duration.ofDays(300)),
                        reference.minus(Duration.ofDays(200)),
                        reference.minus(Duration.ofDays(130)),
                    ),
            )

        val outcome = evaluate(ReviewSignal.TappedPortal, state)

        assertEquals(ReviewPromptDecision.HOLD, outcome.decision)
    }

    @Test
    fun `fires once the oldest of three prompts ages out past 365 days`() {
        val state =
            eligibleState(
                engagementScore = 3,
                lastPromptDate = reference.minus(Duration.ofDays(200)),
                promptTimestamps =
                    listOf(
                        reference.minus(Duration.ofDays(366)), // aged out
                        reference.minus(Duration.ofDays(300)),
                        reference.minus(Duration.ofDays(200)),
                    ),
            )

        val outcome = evaluate(ReviewSignal.TappedPortal, state)

        assertEquals(ReviewPromptDecision.FIRE, outcome.decision)
    }

    // endregion

    // region Session suppression guard

    @Test
    fun `never fires while the session is suppressed`() {
        val outcome =
            evaluate(ReviewSignal.TappedPortal, eligibleState(engagementScore = 3), sessionSuppressed = true)

        assertEquals(ReviewPromptDecision.HOLD, outcome.decision)
        assertEquals(6, outcome.state.engagementScore) // suppression does not stop accrual
    }

    // endregion

    // region Reset after fire

    @Test
    fun `after a fire the score resets and the prompt is timestamped`() {
        val state = eligibleState(engagementScore = 3, promptTimestamps = emptyList())

        val outcome = evaluate(ReviewSignal.TappedPortal, state)

        assertEquals(ReviewPromptDecision.FIRE, outcome.decision)
        assertEquals(0, outcome.state.engagementScore)
        assertEquals(reference, outcome.state.lastPromptDate)
        assertEquals(listOf(reference), outcome.state.promptTimestamps)
    }

    // endregion

    // region Loyalty active-day eligibility

    @Test
    fun `loyalty is not fire-eligible before 3 distinct active days`() {
        val state =
            eligibleState(engagementScore = 8, lastActiveDayKey = "2023-11-14", distinctActiveDays = 1)

        val outcome =
            evaluate(ReviewSignal.ActiveDay, state, now = reference.plus(day), isReactivation = true)

        assertEquals(ReviewPromptDecision.HOLD, outcome.decision)
        assertEquals(2, outcome.state.distinctActiveDays)
    }

    @Test
    fun `loyalty is fire-eligible on the 3rd distinct day during a re-entry`() {
        val state =
            eligibleState(engagementScore = 5, lastActiveDayKey = "2023-11-14", distinctActiveDays = 2)

        val outcome =
            evaluate(ReviewSignal.ActiveDay, state, now = reference.plus(day), isReactivation = true)

        assertEquals(ReviewPromptDecision.FIRE, outcome.decision)
        assertEquals(3, outcome.state.distinctActiveDays)
    }

    @Test
    fun `loyalty never fires on a cold launch even at 3 distinct days`() {
        val state =
            eligibleState(engagementScore = 5, lastActiveDayKey = "2023-11-14", distinctActiveDays = 2)

        val outcome =
            evaluate(ReviewSignal.ActiveDay, state, now = reference.plus(day), isReactivation = false)

        assertEquals(ReviewPromptDecision.HOLD, outcome.decision)
        assertEquals(3, outcome.state.distinctActiveDays)
        assertEquals(6, outcome.state.engagementScore)
    }

    @Test
    fun `the same calendar day is never counted twice`() {
        val state = eligibleState(engagementScore = 4, lastActiveDayKey = "2023-11-14", distinctActiveDays = 2)

        val outcome = evaluate(ReviewSignal.ActiveDay, state, now = reference, isReactivation = true)

        assertEquals(ReviewPromptDecision.HOLD, outcome.decision)
        assertEquals(2, outcome.state.distinctActiveDays)
        assertEquals(4, outcome.state.engagementScore)
    }

    @Test
    fun `a backward clock change counts a new day without corrupting state`() {
        val state = eligibleState(engagementScore = 0, lastActiveDayKey = "2023-11-15", distinctActiveDays = 3)

        // now is a day earlier than the last recorded key.
        val outcome = evaluate(ReviewSignal.ActiveDay, state, now = reference, isReactivation = true)

        assertEquals(4, outcome.state.distinctActiveDays)
        assertEquals(1, outcome.state.engagementScore)
        assertEquals("2023-11-14", outcome.state.lastActiveDayKey)
        assertFalse(outcome.decision == ReviewPromptDecision.FIRE && outcome.state.distinctActiveDays < 3)
        assertNull(state.lastPromptDate)
    }

    // endregion
}
