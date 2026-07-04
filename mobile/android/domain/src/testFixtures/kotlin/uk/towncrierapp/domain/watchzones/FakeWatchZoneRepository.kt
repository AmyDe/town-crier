package uk.towncrierapp.domain.watchzones

import uk.towncrierapp.domain.auth.DomainError

/** Hand-written fake for [WatchZoneRepository] — state-based, per testing.md conventions. */
public class FakeWatchZoneRepository(
    public var stored: MutableList<WatchZone> = mutableListOf(),
) : WatchZoneRepository {
    public var zonesFailWith: DomainError? = null
    public var createFailWith: DomainError? = null
    public var updateFailWith: DomainError? = null
    public var deleteFailWith: DomainError? = null

    public val createCalls: MutableList<WatchZone> = mutableListOf()
    public val updateCalls: MutableList<WatchZone> = mutableListOf()
    public val deleteCalls: MutableList<WatchZoneId> = mutableListOf()

    override suspend fun zones(): List<WatchZone> {
        zonesFailWith?.let { throw it }
        return stored.toList()
    }

    override suspend fun create(zone: WatchZone) {
        createCalls += zone
        createFailWith?.let { throw it }
        stored += zone
    }

    override suspend fun update(zone: WatchZone) {
        updateCalls += zone
        updateFailWith?.let { throw it }
        stored = stored.map { if (it.id == zone.id) zone else it }.toMutableList()
    }

    override suspend fun delete(id: WatchZoneId) {
        deleteCalls += id
        deleteFailWith?.let { throw it }
        stored.removeAll { it.id == id }
    }
}
