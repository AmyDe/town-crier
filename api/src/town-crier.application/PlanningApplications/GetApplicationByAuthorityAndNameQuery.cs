namespace TownCrier.Application.PlanningApplications;

/// <summary>
/// Looks up a planning application by its authority code (AreaId as string) and name
/// (the PlanIt case reference, which is the Cosmos document id). Uses a single-partition
/// point read — ~1 RU — instead of the cross-partition uid scan.
/// </summary>
/// <param name="AuthorityCode">The AreaId as a string; doubles as the Cosmos partition key.</param>
/// <param name="Name">The PlanIt case reference (document id in the applications container).</param>
/// <param name="UserId">Optional — when set, triggers refresh-on-tap for saved applications.</param>
public sealed record GetApplicationByAuthorityAndNameQuery(
    string AuthorityCode,
    string Name,
    string? UserId);
