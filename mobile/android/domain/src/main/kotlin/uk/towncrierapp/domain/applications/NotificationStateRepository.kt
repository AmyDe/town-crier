package uk.towncrierapp.domain.applications

/** The legacy `/advance` endpoint is deliberately not represented here. */
public interface NotificationStateRepository {
    public suspend fun state(): NotificationState

    /** Marks [ids] read. Capped at 500 ids per underlying request by the `:data` implementation. */
    public suspend fun markRead(ids: List<PlanningApplicationId>)

    public suspend fun markAllRead()
}
