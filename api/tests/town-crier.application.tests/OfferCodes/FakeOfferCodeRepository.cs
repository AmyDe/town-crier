using System.Collections.Concurrent;
using TownCrier.Application.OfferCodes;
using TownCrier.Domain.OfferCodes;

namespace TownCrier.Application.Tests.OfferCodes;

internal sealed class FakeOfferCodeRepository : IOfferCodeRepository
{
    private readonly ConcurrentDictionary<string, OfferCode> store = new(StringComparer.Ordinal);

    public int Count => this.store.Count;

    public IReadOnlyCollection<OfferCode> Snapshot() => this.store.Values.ToArray();

    public Task<OfferCode?> GetAsync(string canonicalCode, CancellationToken ct)
    {
        this.store.TryGetValue(canonicalCode, out var code);
        return Task.FromResult(code);
    }

    public Task CreateAsync(OfferCode code, CancellationToken ct)
    {
        if (!this.store.TryAdd(code.Code, code))
        {
            throw new InvalidOperationException($"Offer code '{code.Code}' already exists.");
        }

        return Task.CompletedTask;
    }

    public Task SaveAsync(OfferCode code, CancellationToken ct)
    {
        this.store[code.Code] = code;
        return Task.CompletedTask;
    }
}
