package uk.towncrierapp.data.api

import kotlinx.coroutines.suspendCancellableCoroutine
import okhttp3.Call
import okhttp3.Callback
import okhttp3.Request
import okhttp3.Response
import java.io.IOException
import kotlin.coroutines.resume
import kotlin.coroutines.resumeWithException

/**
 * Production [HttpTransport]: bridges OkHttp's [Call.Factory] to a suspend
 * call. A hand-rolled `okhttp-coroutines` (5.x-only; OkHttp is pinned to 4.x
 * because 5.x needs compileSdk 36): it closes the response if the coroutine
 * is cancelled mid-flight, and needs no dispatcher hop because enqueue is
 * non-blocking.
 */
public class OkHttpTransport(
    private val callFactory: Call.Factory,
) : HttpTransport {
    override suspend fun execute(request: Request): Response = callFactory.newCall(request).await()
}

@Suppress("RedundantSuspendModifier")
// False positive: this is the textbook suspendCancellableCoroutine bridge —
// detekt 1.23.8's type-resolution against Kotlin 2.4.0 doesn't recognise it
// as a suspending call.
private suspend fun Call.await(): Response =
    suspendCancellableCoroutine { continuation ->
        enqueue(
            object : Callback {
                override fun onResponse(
                    call: Call,
                    response: Response,
                ) {
                    continuation.resume(response) { _, value, _ -> value.close() }
                }

                override fun onFailure(
                    call: Call,
                    e: IOException,
                ) {
                    continuation.resumeWithException(e)
                }
            },
        )
        continuation.invokeOnCancellation { cancel() }
    }
