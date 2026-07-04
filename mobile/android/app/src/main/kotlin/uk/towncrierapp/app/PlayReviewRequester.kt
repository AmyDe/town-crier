package uk.towncrierapp.app

import android.content.Context
import android.content.Intent
import android.net.Uri
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
 * The real Play Store listing — the "Rate the App" fallback
 * ([requestReviewOrOpenStoreListing]) when the in-app review flow fails for
 * any reason, pinned to the real package id (tc-4jjw / #778).
 */
internal const val PLAY_STORE_LISTING_URL = "https://play.google.com/store/apps/details?id=uk.towncrierapp.mobile"

/**
 * Best-effort Play In-App Review requester (GH #628 / #778) for the
 * AUTOMATIC review-prompt engine
 * ([uk.towncrierapp.presentation.reviewprompt.ReviewPromptTracker]): requests
 * a review flow and launches it against the current foreground `Activity`.
 * Silently no-ops on ANY failure — that's [ReviewRequester]'s own contract,
 * since a background engagement-driven ask must never surface an error.
 * Constructed here (not `:presentation`) because it needs a real `Activity`,
 * matching the `CurrentActivityProvider` precedent `Auth0AuthenticationService`
 * already uses. For the manual "Rate the App" settings row — which DOES need
 * to observe failure to fall back to the store listing — see
 * [requestReviewOrOpenStoreListing] instead; the two have deliberately
 * different failure contracts, so they are not the same code path.
 */
internal class PlayReviewRequester(
    private val activityProvider: CurrentActivityProvider,
) : ReviewRequester {
    @Suppress("TooGenericExceptionCaught", "SwallowedException")
    // Deliberately broad: ANY failure in this best-effort flow degrades to a
    // silent no-op (see the class doc above) — that IS the policy.
    override suspend fun requestReview() {
        try {
            requestReviewFlow(activityProvider)
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            // Best-effort — see the class doc above.
        }
    }
}

/**
 * The manual "Rate the App" settings row (tc-4jjw / #778): requests the Play
 * In-App Review flow and, on ANY failure (no foreground Activity, Play
 * Services unavailable, no real Play Console listing yet, ...), falls back
 * to opening the Play Store listing directly. Distinct from
 * [PlayReviewRequester], which must never surface failure.
 */
@Suppress("TooGenericExceptionCaught")
internal suspend fun requestReviewOrOpenStoreListing(
    context: Context,
    activityProvider: CurrentActivityProvider,
) {
    try {
        requestReviewFlow(activityProvider)
    } catch (e: CancellationException) {
        throw e
    } catch (e: Exception) {
        context.startActivity(Intent(Intent.ACTION_VIEW, Uri.parse(PLAY_STORE_LISTING_URL)))
    }
}

private suspend fun requestReviewFlow(activityProvider: CurrentActivityProvider) {
    val activity = activityProvider.currentActivity() ?: error("no foreground activity to present the review flow")
    val manager = ReviewManagerFactory.create(activity)
    val reviewInfo: ReviewInfo = manager.requestReviewFlow().await()
    manager.launchReviewFlow(activity, reviewInfo).await()
}

/** Minimal Task-to-suspend bridge — avoids depending on the exact review-ktx extension names. */
private suspend fun <T> Task<T>.await(): T =
    suspendCancellableCoroutine { continuation ->
        addOnSuccessListener { result -> continuation.resume(result) }
        addOnFailureListener { error -> continuation.resumeWithException(error) }
    }
