package uk.towncrierapp.data.api

/**
 * Describes a request `ApiClient` should make: method, path, an optional
 * already-JSON-encoded [body] (`null` means no request body at all — e.g.
 * `POST /v1/me` — not an empty string), and optional query parameters.
 * `internal`: only `:data`'s own repository implementations construct these
 * (port of iOS `APIEndpoint`).
 */
internal data class ApiEndpoint(
    val method: String,
    val path: String,
    val body: String? = null,
    val query: List<Pair<String, String>> = emptyList(),
) {
    companion object {
        fun get(
            path: String,
            query: List<Pair<String, String>> = emptyList(),
        ) = ApiEndpoint(method = "GET", path = path, query = query)

        fun post(
            path: String,
            body: String? = null,
        ) = ApiEndpoint(method = "POST", path = path, body = body)

        fun put(
            path: String,
            body: String? = null,
        ) = ApiEndpoint(method = "PUT", path = path, body = body)

        fun patch(
            path: String,
            body: String,
        ) = ApiEndpoint(method = "PATCH", path = path, body = body)

        fun delete(path: String) = ApiEndpoint(method = "DELETE", path = path)
    }
}
