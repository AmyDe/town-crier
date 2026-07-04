package uk.towncrierapp.data.api

import okhttp3.Call
import okhttp3.Request
import okhttp3.Response
import okhttp3.coroutines.executeAsync

/**
 * Production [HttpTransport]: bridges OkHttp's [Call.Factory] to a suspend
 * call via the official `okhttp-coroutines` artifact's `executeAsync()` —
 * already cancellation-aware (closes the response if the coroutine is
 * cancelled mid-flight) and already non-blocking, so no `withContext`
 * dispatcher hop is needed here (android-coding-standards: coroutines-and-
 * flow.md).
 */
public class OkHttpTransport(
    private val callFactory: Call.Factory,
) : HttpTransport {
    override suspend fun execute(request: Request): Response = callFactory.newCall(request).executeAsync()
}
