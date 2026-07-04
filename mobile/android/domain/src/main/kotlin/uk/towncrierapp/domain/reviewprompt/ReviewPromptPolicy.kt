package uk.towncrierapp.domain.reviewprompt

import java.time.Clock
import java.time.Duration
import java.time.Instant
import java.time.LocalDate

/** The outcome of a single [ReviewPromptPolicy.evaluate] call. */
public enum class ReviewPromptDecision {
    /** Present the platform's native review dialog now. */
    FIRE,

    /** Do not present the dialog. */
    HOLD,
}

/** [ReviewPromptPolicy.evaluate]'s result: the decision and the state the caller should persist. */
public data class ReviewPromptOutcome(
    val decision: ReviewPromptDecision,
    val state: ReviewPromptState,
)

/**
 * Pure, deterministic decision engine for the store review prompt (GH #628).
 * Verbatim port of iOS `ReviewPromptPolicy`'s weights/threshold/guards.
 *
 * Given an incoming [ReviewSignal], the persisted [ReviewPromptState], a
 * transient session-suppression flag, and whether the current foreground is
 * a background-to-active re-entry, it applies the signal's weight, enforces
 * every guard, and returns a [ReviewPromptOutcome]. It performs no I/O and
 * never touches a platform review API, so it is exhaustively unit-testable.
 *
 * The scarce resource is the store's prompts-per-year quota, so the policy
 * only fires at a genuine engagement peak (the score has reached the
 * threshold *at* a fire-eligible signal) and never after friction.
 */
public class ReviewPromptPolicy(
    private val clock: Clock = Clock.systemUTC(),
) {
    /**
     * Applies [signal] to [state], enforces the guards, and returns the
     * decision plus the state to persist. The signal's weight is always
     * applied (so score accrues even when a guard holds the prompt); the
     * score is reset only on a successful fire.
     */
    public fun evaluate(
        signal: ReviewSignal,
        state: ReviewPromptState,
        sessionSuppressed: Boolean,
        isReactivation: Boolean,
    ): ReviewPromptOutcome {
        val (updated, fireEligible) = apply(signal, state, isReactivation)

        if (!fireEligible || !passesGuards(updated, sessionSuppressed)) {
            return ReviewPromptOutcome(ReviewPromptDecision.HOLD, updated)
        }

        val firedAt = clock.instant()
        val firedState =
            updated.copy(
                engagementScore = 0,
                lastPromptDate = firedAt,
                promptTimestamps = updated.promptTimestamps + firedAt,
            )
        return ReviewPromptOutcome(ReviewPromptDecision.FIRE, firedState)
    }

    /** Mutates [state] for [signal] and returns the updated state plus whether it is a fire-eligible peak. */
    private fun apply(
        signal: ReviewSignal,
        state: ReviewPromptState,
        isReactivation: Boolean,
    ): Pair<ReviewPromptState, Boolean> =
        when (signal) {
            ReviewSignal.TappedPortal ->
                state.copy(engagementScore = state.engagementScore + PORTAL_WEIGHT) to true

            ReviewSignal.OpenedAlert ->
                state.copy(engagementScore = state.engagementScore + OPENED_ALERT_WEIGHT) to true

            ReviewSignal.SavedApplication -> applySavedApplication(state)

            ReviewSignal.ActiveDay -> applyActiveDay(state, isReactivation)

            ReviewSignal.Upgraded -> applyUpgraded(state)
        }

    /** The first save is not counted; only the 2nd and later saves contribute. */
    private fun applySavedApplication(state: ReviewPromptState): Pair<ReviewPromptState, Boolean> {
        val nextSaveCount = state.saveCount + 1
        return if (nextSaveCount < MINIMUM_FIRE_ELIGIBLE_SAVE_COUNT) {
            state.copy(saveCount = nextSaveCount) to false
        } else {
            state.copy(
                saveCount = nextSaveCount,
                engagementScore = state.engagementScore + SAVED_APPLICATION_WEIGHT,
            ) to true
        }
    }

    /**
     * Never double-counts the same calendar day. Loyalty only becomes a fire
     * moment once enough distinct days have accrued, and only on a
     * background-to-active re-entry (never a cold launch).
     */
    private fun applyActiveDay(
        state: ReviewPromptState,
        isReactivation: Boolean,
    ): Pair<ReviewPromptState, Boolean> {
        val key = dayKey(clock.instant())
        if (key == state.lastActiveDayKey) return state to false

        val distinctDays = state.distinctActiveDays + 1
        val updated =
            state.copy(
                lastActiveDayKey = key,
                distinctActiveDays = distinctDays,
                engagementScore = state.engagementScore + ACTIVE_DAY_WEIGHT,
            )
        return updated to (isReactivation && distinctDays >= LOYALTY_DISTINCT_DAY_THRESHOLD)
    }

    /** Latched: contributes at most once across the app's lifetime, and is never itself a fire moment. */
    private fun applyUpgraded(state: ReviewPromptState): Pair<ReviewPromptState, Boolean> =
        if (state.hasRecordedUpgrade) {
            state to false
        } else {
            state.copy(engagementScore = state.engagementScore + UPGRADE_WEIGHT, hasRecordedUpgrade = true) to false
        }

    private fun passesGuards(
        state: ReviewPromptState,
        sessionSuppressed: Boolean,
    ): Boolean {
        if (sessionSuppressed) return false
        if (state.engagementScore < ENGAGEMENT_THRESHOLD) return false

        val current = clock.instant()

        val firstLaunch = state.firstLaunchDate ?: return false
        if (Duration.between(firstLaunch, current) < MINIMUM_ACCOUNT_AGE) return false

        // A backward clock (current before lastPrompt) yields a negative elapsed
        // duration, which is < the cooldown, so it correctly holds.
        state.lastPromptDate?.let { lastPrompt ->
            if (Duration.between(lastPrompt, current) < PROMPT_COOLDOWN) return false
        }

        val windowStart = current.minus(ANNUAL_CAP_WINDOW)
        val recentPromptCount = state.promptTimestamps.count { !it.isBefore(windowStart) }
        if (recentPromptCount >= ANNUAL_CAP_LIMIT) return false

        return true
    }

    /**
     * Derives a calendar-day key in the policy clock's zone so distinct days
     * are counted on day boundaries, never on time deltas — robust to
     * timezone and backward-clock changes.
     */
    private fun dayKey(instant: Instant): String = LocalDate.ofInstant(instant, clock.zone).toString()

    public companion object {
        /** The score at or above which a fire-eligible signal may fire. */
        public const val ENGAGEMENT_THRESHOLD: Int = 6
        public const val PORTAL_WEIGHT: Int = 3
        public const val SAVED_APPLICATION_WEIGHT: Int = 2
        public const val OPENED_ALERT_WEIGHT: Int = 2
        public const val ACTIVE_DAY_WEIGHT: Int = 1
        public const val UPGRADE_WEIGHT: Int = 2

        /** Distinct active days required before a loyalty day becomes fire-eligible. */
        public const val LOYALTY_DISTINCT_DAY_THRESHOLD: Int = 3

        /** Maximum prompt attempts allowed within [ANNUAL_CAP_WINDOW]. */
        public const val ANNUAL_CAP_LIMIT: Int = 3

        private const val MINIMUM_FIRE_ELIGIBLE_SAVE_COUNT: Int = 2

        /** Minimum account age before the first prompt. */
        public val MINIMUM_ACCOUNT_AGE: Duration = Duration.ofDays(7)

        /** Minimum gap between two prompts. */
        public val PROMPT_COOLDOWN: Duration = Duration.ofDays(120)

        /** Rolling window for the belt-and-braces annual cap. */
        public val ANNUAL_CAP_WINDOW: Duration = Duration.ofDays(365)
    }
}
