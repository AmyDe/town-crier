package uk.towncrierapp.domain.applications

import uk.towncrierapp.domain.auth.DomainError
import java.time.OffsetDateTime

/** Hand-written fake for [NotificationStateRepository] — state-based, per testing.md conventions. */
public class FakeNotificationStateRepository(
    public var stateResult: NotificationState = NotificationState(lastReadAt = OffsetDateTime.now(), version = 1, totalUnreadCount = 0),
) : NotificationStateRepository {
    public var stateFailWith: DomainError? = null
    public var markReadFailWith: DomainError? = null
    public var markAllReadFailWith: DomainError? = null

    public val markReadCalls: MutableList<List<PlanningApplicationId>> = mutableListOf()
    public var markAllReadCallCount: Int = 0

    override suspend fun state(): NotificationState {
        stateFailWith?.let { throw it }
        return stateResult
    }

    override suspend fun markRead(ids: List<PlanningApplicationId>) {
        markReadCalls += ids
        markReadFailWith?.let { throw it }
    }

    override suspend fun markAllRead() {
        markAllReadCallCount++
        markAllReadFailWith?.let { throw it }
    }
}
