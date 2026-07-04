package uk.towncrierapp.domain.applications

import java.time.OffsetDateTime

/**
 * `GET /v1/me/notification-state`'s payload (ADR 0035). [lastReadAt] is
 * vestigial (kept only for wire parity). [totalUnreadCount] drives the OS
 * app-icon badge ONLY (#777 D7) — the list screen's displayed unread count is
 * always client-derived from loaded rows instead, never this field.
 */
public data class NotificationState(
    public val lastReadAt: OffsetDateTime?,
    public val version: Int,
    public val totalUnreadCount: Int,
)
