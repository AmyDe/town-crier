package uk.towncrierapp.domain.auth

import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

/**
 * Port of iOS `DomainError`'s `isRetryable` catalogue (user-facing
 * `userTitle`/`userMessage` strings stay in `:presentation` resources per the
 * bead — see LoginScreen's error captions).
 */
class DomainErrorTest {
    @Test
    fun `transient failures are retryable`() {
        assertTrue(DomainError.NetworkUnavailable.isRetryable)
        assertTrue(DomainError.ServerError(500, null).isRetryable)
        assertTrue(DomainError.Unexpected("boom").isRetryable)
        assertTrue(DomainError.SessionExpired.isRetryable)
    }

    @Test
    fun `permanent or user-input failures are not retryable`() {
        assertFalse(DomainError.NotFound.isRetryable)
        assertFalse(DomainError.InsufficientEntitlement("personal").isRetryable)
        assertFalse(DomainError.AuthenticationFailed("cancelled").isRetryable)
    }

    @Test
    fun `InsufficientEntitlement carries the required tier string`() {
        val error = DomainError.InsufficientEntitlement("personal")

        assertEquals("personal", error.required)
    }

    @Test
    fun `ServerError carries the status and body`() {
        val error = DomainError.ServerError(status = 503, body = "maintenance")

        assertEquals(503, error.status)
        assertEquals("maintenance", error.body)
    }

    @Test
    fun `same-case DomainError instances are equal`() {
        assertEquals(DomainError.InsufficientEntitlement("personal"), DomainError.InsufficientEntitlement("personal"))
        assertEquals(DomainError.ServerError(404, "x"), DomainError.ServerError(404, "x"))
        assertEquals(DomainError.SessionExpired, DomainError.SessionExpired)
    }
}
