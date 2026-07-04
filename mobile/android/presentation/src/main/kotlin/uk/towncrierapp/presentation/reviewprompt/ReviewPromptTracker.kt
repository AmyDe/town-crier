package uk.towncrierapp.presentation.reviewprompt

import uk.towncrierapp.domain.reviewprompt.ReviewPromptDecision
import uk.towncrierapp.domain.reviewprompt.ReviewPromptPolicy
import uk.towncrierapp.domain.reviewprompt.ReviewPromptState
import uk.towncrierapp.domain.reviewprompt.ReviewPromptStore
import uk.towncrierapp.domain.reviewprompt.ReviewRequester
import uk.towncrierapp.domain.reviewprompt.ReviewSignal
import java.time.Clock

/**
 * The service the app talks to for the store review prompt (GH #628). Owns
 * the only mutable session flag ([suppressThisSession]), reads and writes
 * the persisted state via the injected [ReviewPromptStore], runs the pure
 * [ReviewPromptPolicy] on each signal, and — on a FIRE decision — asks the
 * injected [ReviewRequester] to present the dialog. Port of iOS
 * `ReviewPromptTracker`.
 */
public class ReviewPromptTracker(
    private val store: ReviewPromptStore,
    private val requester: ReviewRequester,
    private val clock: Clock = Clock.systemUTC(),
    private val policy: ReviewPromptPolicy = ReviewPromptPolicy(clock),
) {
    private var isSessionSuppressed = false

    /**
     * Suppresses any review prompt for the rest of this session. Call during
     * onboarding, when a post-purchase prompt fires, and on friction.
     */
    public fun suppressThisSession() {
        isSessionSuppressed = true
    }

    /**
     * Records an engagement signal: updates the persisted state via the
     * policy and, on a FIRE decision, requests the native review dialog.
     */
    public suspend fun recordSignal(
        signal: ReviewSignal,
        isReactivation: Boolean = false,
    ) {
        val state = establishFirstLaunchDateIfNeeded(store.load())
        val outcome = policy.evaluate(signal, state, isSessionSuppressed, isReactivation)
        store.save(outcome.state)
        if (outcome.decision == ReviewPromptDecision.FIRE) {
            requester.requestReview()
        }
    }

    /**
     * Records a loyalty active-day signal on app foreground. [isReactivation]
     * is `true` only for a background-to-active re-entry, never the
     * cold-launch render.
     */
    public suspend fun recordAppForegrounded(isReactivation: Boolean) {
        recordSignal(ReviewSignal.ActiveDay, isReactivation)
    }

    /** Anchors the account-age guard the first time the tracker runs on a device. */
    private suspend fun establishFirstLaunchDateIfNeeded(state: ReviewPromptState): ReviewPromptState {
        if (state.firstLaunchDate != null) return state
        val updated = state.copy(firstLaunchDate = clock.instant())
        store.save(updated)
        return updated
    }
}
