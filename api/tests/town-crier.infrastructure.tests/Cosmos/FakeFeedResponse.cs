using System.Net;
using Microsoft.Azure.Cosmos;

namespace TownCrier.Infrastructure.Tests.Cosmos;

/// <summary>
/// A hand-written fake for <see cref="FeedResponse{T}"/> that wraps a list of items.
/// </summary>
internal sealed class FakeFeedResponse<T> : FeedResponse<T>
{
    private readonly IReadOnlyList<T> items;

    public FakeFeedResponse(IReadOnlyList<T> items)
    {
        this.items = items;
    }

    public override Headers Headers => new();

    public override IEnumerable<T> Resource => this.items;

    public override HttpStatusCode StatusCode => HttpStatusCode.OK;

    public override CosmosDiagnostics Diagnostics => null!;

    public override int Count => this.items.Count;

    public override string? ContinuationToken => null;

    public override string? IndexMetrics => null;

    public override IEnumerator<T> GetEnumerator() => this.items.GetEnumerator();
}
