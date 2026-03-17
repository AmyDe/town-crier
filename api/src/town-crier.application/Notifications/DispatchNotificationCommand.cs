using TownCrier.Domain.PlanningApplications;
using TownCrier.Domain.WatchZones;

namespace TownCrier.Application.Notifications;

public sealed record DispatchNotificationCommand(PlanningApplication Application, WatchZone MatchedZone);
