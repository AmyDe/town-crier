package uk.towncrierapp.domain.auth

/**
 * Errors thrown from `:data` and caught in `:presentation` (only `:domain`
 * is visible to both, per the module dependency rule). Port of iOS
 * `DomainError`'s catalogue, trimmed to what this issue's seams need — later
 * issues (watch zones, applications, purchases) extend this sealed hierarchy
 * feature-by-feature, the same way [uk.towncrierapp.domain.DomainModule]'s
 * placeholder comment describes for other domain types.
 *
 * User-facing `userTitle`/`userMessage` strings are deliberately NOT here:
 * they live as `:presentation` string resources, keyed off these cases (see
 * the login screen's error caption mapping).
 */
public sealed class DomainError : Exception() {
    /** No session (or an unrecoverable refresh failure) — the user must sign in again. */
    public object SessionExpired : DomainError()

    /** The transport itself failed (no connectivity, DNS, timeout, ...). */
    public object NetworkUnavailable : DomainError()

    /** The resource genuinely does not exist (HTTP 404). */
    public object NotFound : DomainError()

    /** HTTP 403 with an `{"error":"insufficient_entitlement","required":"..."}` body. */
    public data class InsufficientEntitlement(
        public val required: String,
    ) : DomainError()

    /** Any other HTTP error (>= 400 and not one of the cases above). [body] is the raw response text. */
    public data class ServerError(
        public val status: Int,
        public val body: String?,
    ) : DomainError()

    /**
     * Sign-in itself failed (cancelled, provider error, ...). The optional
     * [cause] overload calls [initCause] rather than becoming a stored
     * property — a data class's primary constructor can only declare
     * properties, and threading `cause` through as one would fold it into
     * `equals`/`copy`, breaking `AuthenticationFailed(reason)` comparisons in
     * tests. This is purely so the original SDK exception isn't lost from
     * the stack trace / crash report.
     */
    public data class AuthenticationFailed(
        public val reason: String,
    ) : DomainError() {
        public constructor(reason: String, cause: Throwable) : this(reason) {
            initCause(cause)
        }
    }

    /** Sign-out failed to fully clear the remote session. See [AuthenticationFailed] on the [cause] overload. */
    public data class LogoutFailed(
        public val reason: String,
    ) : DomainError() {
        public constructor(reason: String, cause: Throwable) : this(reason) {
            initCause(cause)
        }
    }

    /** A failure that doesn't fit the cases above (e.g. a malformed response body). See [AuthenticationFailed] on the [cause] overload. */
    public data class Unexpected(
        public val reason: String,
    ) : DomainError() {
        public constructor(reason: String, cause: Throwable) : this(reason) {
            initCause(cause)
        }
    }
}

/** Whether the error is transient and retrying may succeed. */
public val DomainError.isRetryable: Boolean
    get() =
        when (this) {
            DomainError.NetworkUnavailable,
            DomainError.SessionExpired,
            is DomainError.ServerError,
            is DomainError.Unexpected,
            is DomainError.LogoutFailed,
            -> true

            DomainError.NotFound,
            is DomainError.InsufficientEntitlement,
            is DomainError.AuthenticationFailed,
            -> false
        }
