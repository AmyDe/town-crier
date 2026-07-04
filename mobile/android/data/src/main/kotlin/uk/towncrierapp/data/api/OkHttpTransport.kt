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
 * call (android-coding-standards: data-access.md). OkHttp is pinned to the
 * 4.x line here (the 5.x "okhttp-android" variant declares a compileSdk-36
 * floor this project doesn't meet), so the official `okhttp-coroutines`
 * artifact (5.x-only) isn't available — this is that same bridge, hand-
 * rolled: cancellation-aware (closes the response if the coroutine is
 * cancelled mid-flight) and already non-blocking, so no `withContext`
 * dispatcher hop is needed here.
 */
public class OkHttpTransport(
    private val callFactory: Call.Factory,
) : HttpTransport {
    override suspend fun execute(request: Request): Response = callFactory.newCall(request).await()
}

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
