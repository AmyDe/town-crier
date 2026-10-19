package uk.towncrierapp.domain.applications

import java.time.OffsetDateTime

/**
 * `GET /v1/me/notification-state`'s payload (ADR 0035). [lastReadAt] is
 * vestigial (kept only for wire parity). [totalUnreadCount] drives only the OS
 * app-icon badge; the list screen derives its unread count from loaded rows.
 */
public data class NotificationState(
    public val lastReadAt: OffsetDateTime?,
    public val version: Int,
    public val totalUnreadCount: Int,
)
