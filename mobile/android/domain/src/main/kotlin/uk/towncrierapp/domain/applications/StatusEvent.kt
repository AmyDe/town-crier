package uk.towncrierapp.domain.applications

import java.time.LocalDate

/**
 * One point in a [PlanningApplication.statusHistory] timeline, CLIENT-
 * synthesized from the list/detail row (there is no server-side history
 * endpoint): a `startDate` row always produces an [ApplicationStatus.Undecided]
 * event, and a `decidedDate` row produces a second event carrying the
 * application's actual (decided) status — but only when
 * [ApplicationStatus.isDecided] is true, otherwise it is folded away. Max 2
 * points. Port of iOS `StatusEvent` (GH#775).
 */
public data class StatusEvent(
    public val status: ApplicationStatus,
    public val date: LocalDate,
)
