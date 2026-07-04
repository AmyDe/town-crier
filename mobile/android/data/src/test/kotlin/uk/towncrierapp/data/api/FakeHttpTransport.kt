package uk.towncrierapp.data.api

import okhttp3.MediaType.Companion.toMediaType
import okhttp3.Protocol
import okhttp3.Request
import okhttp3.Response
import okhttp3.ResponseBody.Companion.toResponseBody
import java.io.IOException

/**
 * Hand-written [HttpTransport] fake: a queue of scripted per-request
 * responses (or failures), no MockWebServer (epic #770 "API contract
 * essentials" test strategy). Each [execute] call records the [Request] it
 * received so tests can assert headers/method/URL, and pops the next
 * queued response/failure.
 */
internal class FakeHttpTransport : HttpTransport {
    val requests: MutableList<Request> = mutableListOf()
    private val queue: ArrayDeque<(Request) -> Response> = ArrayDeque()

    fun enqueueResponse(
        code: Int,
        body: String = "",
        headers: Map<String, String> = emptyMap(),
    ) {
        queue.addLast { request -> fakeResponse(request, code, body, headers) }
    }

    fun enqueueFailure(exception: IOException) {
        queue.addLast { throw exception }
    }

    override suspend fun execute(request: Request): Response {
        requests += request
        val next = queue.removeFirstOrNull() ?: throw IOException("FakeHttpTransport: no response queued for $request")
        return next(request)
    }
}

private fun fakeResponse(
    request: Request,
    code: Int,
    body: String,
    headers: Map<String, String>,
): Response {
    val builder =
        Response
            .Builder()
            .request(request)
            .protocol(Protocol.HTTP_1_1)
            .code(code)
            .message("fake")
            .body(body.toResponseBody("application/json".toMediaType()))
    headers.forEach { (name, value) -> builder.addHeader(name, value) }
    return builder.build()
}
