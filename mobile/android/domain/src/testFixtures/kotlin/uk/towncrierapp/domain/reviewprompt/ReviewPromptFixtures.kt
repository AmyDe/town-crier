package uk.towncrierapp.domain.reviewprompt

import java.time.Instant

/** Fixture factory for [ReviewPromptState]. */
public fun aReviewPromptState(
    firstLaunchDate: Instant? = null,
    engagementScore: Int = 0,
    saveCount: Int = 0,
    lastActiveDayKey: String? = null,
    distinctActiveDays: Int = 0,
    lastPromptDate: Instant? = null,
    promptTimestamps: List<Instant> = emptyList(),
    hasRecordedUpgrade: Boolean = false,
) = ReviewPromptState(
    firstLaunchDate = firstLaunchDate,
    engagementScore = engagementScore,
    saveCount = saveCount,
    lastActiveDayKey = lastActiveDayKey,
    distinctActiveDays = distinctActiveDays,
    lastPromptDate = lastPromptDate,
    promptTimestamps = promptTimestamps,
    hasRecordedUpgrade = hasRecordedUpgrade,
)

/** Hand-written fake for [ReviewPromptStore]: an in-memory single-slot state, defaulting to a fresh [ReviewPromptState]. */
public class FakeReviewPromptStore(
    initial: ReviewPromptState = aReviewPromptState(),
) : ReviewPromptStore {
    public var state: ReviewPromptState = initial
    public val saveCalls: MutableList<ReviewPromptState> = mutableListOf()

    override suspend fun load(): ReviewPromptState = state

    override suspend fun save(state: ReviewPromptState) {
        this.state = state
        saveCalls += state
    }
}

/** Hand-written fake for [ReviewRequester]: records every call instead of touching a real platform API. */
public class FakeReviewRequester : ReviewRequester {
    public val requestReviewCalls: MutableList<Unit> = mutableListOf()

    override suspend fun requestReview() {
        requestReviewCalls += Unit
    }
}
