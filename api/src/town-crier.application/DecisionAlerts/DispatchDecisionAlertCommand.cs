using TownCrier.Domain.PlanningApplications;

namespace TownCrier.Application.DecisionAlerts;

public sealed record DispatchDecisionAlertCommand(PlanningApplication Application);
