package uk.towncrierapp.data.api

import okhttp3.Request
import okhttp3.Response

/**
 * The transport seam `ApiClient` is built over: a thin suspend wrapper around
 * OkHttp's `Call.Factory`. Production wires [OkHttpTransport]; unit tests
 * substitute a hand-written `FakeHttpTransport` that returns canned [Response]s.
 */
public interface HttpTransport {
    public suspend fun execute(request: Request): Response
}
