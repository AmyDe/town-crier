package uk.towncrierapp.domain.reviewprompt

import java.time.Instant

/**
 * The locally-persisted state the review-prompt policy reads and updates.
 * Every field is device-local: there is no server telemetry, analytics, or
 * PII involved in deciding when to ask for a rating. Port of iOS
 * `ReviewPromptState`.
 */
public data class ReviewPromptState(
    /**
     * When this device first ran the app with the review feature — the
     * anchor for the account-age guard. `null` until the tracker establishes
     * it on first run.
     */
    val firstLaunchDate: Instant? = null,
    /** The accumulated engagement score, reset to 0 after a fire. */
    val engagementScore: Int = 0,
    /** How many applications the user has saved (gates the first-save exclusion). */
    val saveCount: Int = 0,
    /** The calendar-day key of the most recent active day, so the same day is never double-counted. */
    val lastActiveDayKey: String? = null,
    /** The number of distinct calendar days the app has been foregrounded on. */
    val distinctActiveDays: Int = 0,
    /** When the last review prompt was attempted, anchoring the cooldown guard. */
    val lastPromptDate: Instant? = null,
    /** Timestamps of past prompt attempts, used for the rolling annual cap. */
    val promptTimestamps: List<Instant> = emptyList(),
    /** Whether the upgrade score contribution has already been latched. */
    val hasRecordedUpgrade: Boolean = false,
)
