package uk.towncrierapp.domain.applications

import uk.towncrierapp.domain.watchzones.Coordinate
import java.time.LocalDate

/**
 * A single PlanIt planning application, as browsed from a watch zone's list,
 * a saved-applications row, or a by-id/by-slug detail fetch. [reference] is
 * the display case reference — the same value backing [id]'s
 * [PlanningApplicationId.name], kept as its own field so the presentation
 * layer never has to reach into [id] to render the fields card. [location] is
 * `null` when the row carries no coordinate; [portalUrl] is `null` when
 * PlanIt has no external council-portal link for this application. Port of
 * iOS `PlanningApplication` (GH#775).
 */
public data class PlanningApplication(
    public val id: PlanningApplicationId,
    public val reference: String,
    public val authority: LocalAuthority,
    public val status: ApplicationStatus,
    public val receivedDate: LocalDate,
    public val description: String,
    public val address: String,
    public val location: Coordinate? = null,
    public val portalUrl: String? = null,
    public val statusHistory: List<StatusEvent> = emptyList(),
    public val latestUnreadEvent: LatestUnreadEvent? = null,
)
