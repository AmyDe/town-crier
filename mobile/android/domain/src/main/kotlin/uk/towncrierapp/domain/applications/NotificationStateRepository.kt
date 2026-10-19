package uk.towncrierapp.domain.applications

/**
 * Read state (ADR 0035). [markRead] and [markAllRead] take domain
 * [PlanningApplicationId]s; wire-format conversion is a `:data` concern. The
 * legacy `/advance` endpoint is deliberately not represented here.
 */
public interface NotificationStateRepository {
    public suspend fun state(): NotificationState

    /** Marks [ids] read. Capped at 500 ids per underlying request by the `:data` implementation. */
    public suspend fun markRead(ids: List<PlanningApplicationId>)

    public suspend fun markAllRead()
}
