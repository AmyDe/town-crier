using TownCrier.Application.PlanningApplications;
using TownCrier.Domain.PlanningApplications;

namespace TownCrier.Application.Tests.SavedApplications;

/// <summary>
/// Spy fake that fails the test if any read or write reaches the planning
/// application repository. Used by the saved-list query handler tests to prove
/// the new design renders entirely from the embedded snapshot — zero hydration
/// fan-out — for the 429 storm fix in bd tc-udby.
/// </summary>
internal sealed class FailIfCalledPlanningApplicationRepository : IPlanningApplicationRepository
{
    public Task UpsertAsync(PlanningApplication application, CancellationToken ct) =>
        throw new InvalidOperationException(
            "Planning repository must not be touched on the saved-list happy path. See bd tc-udby.");

    public Task<PlanningApplication?> GetByUidAsync(string uid, CancellationToken ct) =>
        throw new InvalidOperationException(
            "Planning repository must not be touched on the saved-list happy path. See bd tc-udby.");

    public Task<PlanningApplication?> GetByUidAsync(string uid, string authorityCode, CancellationToken ct) =>
        throw new InvalidOperationException(
            "Planning repository must not be touched on the saved-list happy path. See bd tc-udby.");

    public Task<IReadOnlyCollection<PlanningApplication>> GetByAuthorityIdAsync(int authorityId, CancellationToken ct) =>
        throw new InvalidOperationException(
            "Planning repository must not be touched on the saved-list happy path. See bd tc-udby.");

    public Task<IReadOnlyCollection<PlanningApplication>> FindNearbyAsync(
        string authorityCode, double latitude, double longitude, double radiusMetres, CancellationToken ct) =>
        throw new InvalidOperationException(
            "Planning repository must not be touched on the saved-list happy path. See bd tc-udby.");
}
