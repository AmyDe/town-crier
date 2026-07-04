package uk.towncrierapp.domain.applications

import java.time.OffsetDateTime

/**
 * A row from `GET /v1/me/saved-applications`. [applicationUid] is always the
 * RECONSTRUCTED [PlanningApplicationId] (tc-jjl4: saved-state comparison must
 * use this, never a raw uid string) — the field name is kept as `Uid` for
 * parity with the wire/iOS naming, even though its type is structured, not a
 * bare string. A `null` [application] means a legacy save whose payload
 * predates this field being populated server-side; those rows are dropped
 * from display entirely, never shown as an empty/placeholder row. Port of iOS
 * `SavedApplication` (GH#775).
 */
public data class SavedApplication(
    public val applicationUid: PlanningApplicationId,
    public val savedAt: OffsetDateTime,
    public val application: PlanningApplication? = null,
)
