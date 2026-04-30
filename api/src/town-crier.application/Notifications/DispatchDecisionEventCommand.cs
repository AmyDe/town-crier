using TownCrier.Domain.PlanningApplications;

namespace TownCrier.Application.Notifications;

/// <summary>
/// Fan-out trigger raised when a tracked planning application transitions
/// into a final decision state (Permitted, Conditions, Rejected, Appealed).
/// The handler computes the union of users who match the application via a
/// watch zone OR a saved bookmark, OR-merges per-channel toggles across
/// those sources, and writes one <see cref="Notification"/> per user.
/// </summary>
/// <param name="Application">The application whose state transitioned.</param>
public sealed record DispatchDecisionEventCommand(PlanningApplication Application);
