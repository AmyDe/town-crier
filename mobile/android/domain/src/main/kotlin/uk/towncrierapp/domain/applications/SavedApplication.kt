package uk.towncrierapp.domain.applications

import java.time.OffsetDateTime

/**
 * A row from `GET /v1/me/saved-applications`. [applicationUid] is always the
 * reconstructed [PlanningApplicationId]; saved-state comparison must use it,
 * never a raw uid string. A `null` [application] is a legacy save with no
 * payload; such rows are not displayed.
 */
public data class SavedApplication(
    public val applicationUid: PlanningApplicationId,
    public val savedAt: OffsetDateTime,
    public val application: PlanningApplication? = null,
)
