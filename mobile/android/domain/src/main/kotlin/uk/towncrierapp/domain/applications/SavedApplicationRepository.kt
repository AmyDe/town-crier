package uk.towncrierapp.domain.applications

/** The user's flat, cross-zone saved-applications list. Port of iOS `SavedApplicationRepository`. */
public interface SavedApplicationRepository {
    /** Every saved row, in wire order — callers sort by `savedAt` DESC themselves (list is small, cross-zone, unpaged). */
    public suspend fun savedApplications(): List<SavedApplication>

    public suspend fun save(id: PlanningApplicationId)

    public suspend fun unsave(id: PlanningApplicationId)
}
