package uk.towncrierapp.data.api

import okhttp3.Request
import okhttp3.Response

/**
 * The transport seam `ApiClient` is built over. Production wires
 * [OkHttpTransport]; unit tests substitute a hand-written `FakeHttpTransport`
 * that returns canned [Response]s — no MockWebServer (epic #770 "API
 * contract essentials" test strategy). This is a deliberately thin,
 * coroutine-friendly wrapper around OkHttp's own `Call.Factory` seam (see
 * [OkHttpTransport]), not a reinvention of OkHttp's request/response types.
 */
public interface HttpTransport {
    public suspend fun execute(request: Request): Response
}
