package uk.towncrierapp.app

import com.google.android.gms.tasks.Task
import com.google.android.play.core.review.ReviewInfo
import com.google.android.play.core.review.ReviewManagerFactory
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.suspendCancellableCoroutine
import uk.towncrierapp.data.auth.CurrentActivityProvider
import uk.towncrierapp.domain.reviewprompt.ReviewRequester
import kotlin.coroutines.resume
import kotlin.coroutines.resumeWithException

/**
 * Best-effort Play In-App Review requester (GH #628 / #778): requests a
 * review flow and launches it against the current foreground `Activity`.
 * Silently no-ops on ANY failure — no foreground Activity, Play Services
 * unavailable, no real Play Console listing yet, or the OS declining to
 * show the dialog (Play's own API design never reports whether it actually
 * appeared) — matching [ReviewRequester]'s best-effort contract, which the
 * automatic review-prompt engine ([uk.towncrierapp.presentation.reviewprompt.ReviewPromptTracker])
 * relies on. Constructed here (not `:presentation`) because it needs a real
 * `Activity`, matching the `CurrentActivityProvider` precedent
 * `Auth0AuthenticationService` already uses.
 */
internal class PlayReviewRequester(
    private val activityProvider: CurrentActivityProvider,
) : ReviewRequester {
    @Suppress("TooGenericExceptionCaught", "SwallowedException")
    // Deliberately broad: ANY failure in this best-effort flow degrades to a
    // silent no-op (see the class doc above) — that IS the policy.
    override suspend fun requestReview() {
        try {
            val activity = activityProvider.currentActivity() ?: return
            val manager = ReviewManagerFactory.create(activity)
            val reviewInfo: ReviewInfo = manager.requestReviewFlow().await()
            manager.launchReviewFlow(activity, reviewInfo).await()
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            // Best-effort — see the class doc above.
        }
    }
}

/** Minimal Task-to-suspend bridge — avoids depending on the exact review-ktx extension names. */
private suspend fun <T> Task<T>.await(): T =
    suspendCancellableCoroutine { continuation ->
        addOnSuccessListener { result -> continuation.resume(result) }
        addOnFailureListener { error -> continuation.resumeWithException(error) }
    }
