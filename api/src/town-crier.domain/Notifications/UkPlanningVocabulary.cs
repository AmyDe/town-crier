namespace TownCrier.Domain.Notifications;

/// <summary>
/// Maps PlanIt's wire vocabulary (<c>Permitted</c>, <c>Conditions</c>,
/// <c>Rejected</c>, <c>Appealed</c>) to the user-facing UK planning terms
/// residents recognise (<c>Approved</c>, <c>Approved with conditions</c>,
/// <c>Refused</c>, <c>Refusal appealed</c>). Centralised so push payloads,
/// email digests, and any future UI surface stay in sync.
/// See <c>docs/specs/decision-state-vocabulary.md</c>.
/// </summary>
public static class UkPlanningVocabulary
{
    /// <summary>
    /// Returns the UK display string for a PlanIt <c>app_state</c> value, or
    /// <see langword="null"/> if the input is not one of the four decision
    /// states this helper covers. Matching is case-insensitive to tolerate
    /// upstream casing drift.
    /// </summary>
    /// <param name="planItAppState">The raw PlanIt <c>app_state</c> string.</param>
    /// <returns>The UK display label, or <see langword="null"/> when the input is not a recognised decision state.</returns>
    public static string? GetDisplayString(string? planItAppState)
    {
        if (string.IsNullOrWhiteSpace(planItAppState))
        {
            return null;
        }

        if (string.Equals(planItAppState, "Permitted", StringComparison.OrdinalIgnoreCase))
        {
            return "Approved";
        }

        if (string.Equals(planItAppState, "Conditions", StringComparison.OrdinalIgnoreCase))
        {
            return "Approved with conditions";
        }

        if (string.Equals(planItAppState, "Rejected", StringComparison.OrdinalIgnoreCase))
        {
            return "Refused";
        }

        if (string.Equals(planItAppState, "Appealed", StringComparison.OrdinalIgnoreCase))
        {
            return "Refusal appealed";
        }

        return null;
    }
}
