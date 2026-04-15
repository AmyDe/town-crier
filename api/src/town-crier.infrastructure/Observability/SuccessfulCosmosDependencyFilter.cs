using System.Diagnostics;
using OpenTelemetry;

namespace TownCrier.Infrastructure.Observability;

/// <summary>
/// Drops successful (2xx) HTTP dependency spans targeting Cosmos DB.
/// These generate high telemetry volume (~1.5 GB/week) with little diagnostic
/// value — failures and the custom Cosmos activity spans (which carry RU metrics)
/// are preserved.
/// </summary>
public sealed class SuccessfulCosmosDependencyFilter : BaseProcessor<Activity>
{
    public override void OnEnd(Activity activity)
    {
        ArgumentNullException.ThrowIfNull(activity);

        if (activity.Kind != ActivityKind.Client)
        {
            return;
        }

        var peerName = activity.GetTagItem("server.address") as string;
        if (peerName is null || !peerName.Contains(".documents.azure.com", StringComparison.Ordinal))
        {
            return;
        }

        var statusCode = activity.GetTagItem("http.response.status_code") as int?;
        if (statusCode is >= 200 and < 300)
        {
            activity.ActivityTraceFlags &= ~ActivityTraceFlags.Recorded;
        }
    }
}
