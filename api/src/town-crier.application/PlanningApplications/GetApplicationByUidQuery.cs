namespace TownCrier.Application.PlanningApplications;

// UserId is optional so unauthenticated callers (or background callers from
// other handlers) can skip the refresh-on-tap side effect. The /v1/applications
// endpoint always supplies it; see bd tc-udby.
public sealed record GetApplicationByUidQuery(string Uid, string? UserId);
