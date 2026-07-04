package uk.towncrierapp.domain.applications

/**
 * Read state (ADR 0035). [markRead] and [markAllRead] take domain
 * [PlanningApplicationId]s — the wire's `applicationUid`-key-holds-a-name
 * misnomer and the int `authorityId` conversion are entirely a `:data`
 * concern (see `ApiNotificationStateRepository`). Port of iOS
 * `NotificationStateRepository`. The legacy `/advance` endpoint is
 * deliberately NOT represented here.
 */
public interface NotificationStateRepository {
    public suspend fun state(): NotificationState

    /** Marks [ids] read. Capped at 500 ids per underlying request by the `:data` implementation. */
    public suspend fun markRead(ids: List<PlanningApplicationId>)

    public suspend fun markAllRead()
}
