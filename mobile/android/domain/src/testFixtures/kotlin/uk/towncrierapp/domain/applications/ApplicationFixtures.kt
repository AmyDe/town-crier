package uk.towncrierapp.domain.applications

import java.time.LocalDate
import java.time.OffsetDateTime

/** Fixture factory for [PlanningApplicationId] — override only what a test cares about. */
public fun aPlanningApplicationId(
    authority: String = "42",
    name: String = "24/0001",
): PlanningApplicationId = PlanningApplicationId(authority, name)

/** Fixture factory for [LocalAuthority]. */
public fun aLocalAuthority(
    code: String = "42",
    name: String = "Camden",
    areaType: String? = null,
    slug: String? = null,
): LocalAuthority = LocalAuthority(code, name, areaType, slug)

/** Fixture factory for [PlanningApplication] — a pending application with no history, matching a freshly-submitted row. */
public fun aPlanningApplication(
    id: PlanningApplicationId = aPlanningApplicationId(),
    reference: String = id.name,
    authority: LocalAuthority = aLocalAuthority(code = id.authority),
    status: ApplicationStatus = ApplicationStatus.Undecided,
    receivedDate: LocalDate = LocalDate.of(2026, 1, 15),
    description: String = "Two-storey rear extension",
    address: String = "1 Example Street, Camden",
    location: uk.towncrierapp.domain.watchzones.Coordinate? = null,
    portalUrl: String? = null,
    statusHistory: List<StatusEvent> = listOf(StatusEvent(ApplicationStatus.Undecided, receivedDate)),
    latestUnreadEvent: LatestUnreadEvent? = null,
): PlanningApplication =
    PlanningApplication(
        id = id,
        reference = reference,
        authority = authority,
        status = status,
        receivedDate = receivedDate,
        description = description,
        address = address,
        location = location,
        portalUrl = portalUrl,
        statusHistory = statusHistory,
        latestUnreadEvent = latestUnreadEvent,
    )

/** Fixture factory for [LatestUnreadEvent]. */
public fun aLatestUnreadEvent(
    type: String = "NewApplication",
    decision: String? = null,
    createdAt: OffsetDateTime = OffsetDateTime.parse("2026-01-15T09:00:00Z"),
): LatestUnreadEvent = LatestUnreadEvent(type, decision, createdAt)

/** Fixture factory for [SavedApplication]. */
public fun aSavedApplication(
    applicationUid: PlanningApplicationId = aPlanningApplicationId(),
    savedAt: OffsetDateTime = OffsetDateTime.parse("2026-01-16T09:00:00Z"),
    application: PlanningApplication? = aPlanningApplication(id = applicationUid),
): SavedApplication = SavedApplication(applicationUid, savedAt, application)

/** Fixture factory for [ApplicationPage]. */
public fun anApplicationPage(
    applications: List<PlanningApplication> = listOf(aPlanningApplication()),
    nextCursor: String? = null,
): ApplicationPage = ApplicationPage(applications, nextCursor)
