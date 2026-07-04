package uk.towncrierapp.domain.applications

import uk.towncrierapp.domain.auth.DomainError

/** Hand-written fake for [SavedApplicationRepository] — state-based, per testing.md conventions. */
public class FakeSavedApplicationRepository(
    public var stored: MutableList<SavedApplication> = mutableListOf(),
) : SavedApplicationRepository {
    public var savedApplicationsFailWith: DomainError? = null
    public var saveFailWith: DomainError? = null
    public var unsaveFailWith: DomainError? = null

    public val saveCalls: MutableList<PlanningApplicationId> = mutableListOf()
    public val unsaveCalls: MutableList<PlanningApplicationId> = mutableListOf()

    override suspend fun savedApplications(): List<SavedApplication> {
        savedApplicationsFailWith?.let { throw it }
        return stored.toList()
    }

    override suspend fun save(id: PlanningApplicationId) {
        saveCalls += id
        saveFailWith?.let { throw it }
    }

    override suspend fun unsave(id: PlanningApplicationId) {
        unsaveCalls += id
        unsaveFailWith?.let { throw it }
        stored.removeAll { it.applicationUid == id }
    }
}
