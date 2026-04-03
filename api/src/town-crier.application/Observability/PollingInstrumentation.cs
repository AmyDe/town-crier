using System.Diagnostics;

namespace TownCrier.Application.Observability;

public static class PollingInstrumentation
{
    public const string ActivitySourceName = "TownCrier.Polling";

    public static readonly ActivitySource Source = new(ActivitySourceName);
}
