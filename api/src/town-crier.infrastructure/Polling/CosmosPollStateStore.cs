using System.Globalization;
using TownCrier.Application.Polling;
using TownCrier.Infrastructure.Cosmos;

namespace TownCrier.Infrastructure.Polling;

public sealed class CosmosPollStateStore : IPollStateStore
{
    private const string DocumentId = "poll-state";

    private readonly ICosmosRestClient client;

    public CosmosPollStateStore(ICosmosRestClient client)
    {
        ArgumentNullException.ThrowIfNull(client);
        this.client = client;
    }

    public async Task<DateTimeOffset?> GetLastPollTimeAsync(CancellationToken ct)
    {
        var doc = await this.client.ReadDocumentAsync(
            CosmosContainerNames.PollState,
            DocumentId,
            DocumentId,
            CosmosJsonSerializerContext.Default.PollStateDocument,
            ct).ConfigureAwait(false);

        if (doc is null)
        {
            return null;
        }

        return DateTimeOffset.Parse(doc.LastPollTime, CultureInfo.InvariantCulture);
    }

    public async Task SaveLastPollTimeAsync(DateTimeOffset pollTime, CancellationToken ct)
    {
        var doc = new PollStateDocument
        {
            Id = DocumentId,
            LastPollTime = pollTime.ToString("O", CultureInfo.InvariantCulture),
        };

        await this.client.UpsertDocumentAsync(
            CosmosContainerNames.PollState,
            doc,
            DocumentId,
            CosmosJsonSerializerContext.Default.PollStateDocument,
            ct).ConfigureAwait(false);
    }
}
