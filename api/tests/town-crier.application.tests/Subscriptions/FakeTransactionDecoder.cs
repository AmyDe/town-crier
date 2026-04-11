using TownCrier.Application.Subscriptions;

namespace TownCrier.Application.Tests.Subscriptions;

internal sealed class FakeTransactionDecoder : ITransactionDecoder
{
    private readonly Dictionary<string, DecodedTransaction> transactions = [];

    public void Register(string json, DecodedTransaction transaction)
    {
        this.transactions[json] = transaction;
    }

    public DecodedTransaction Decode(string json)
    {
        if (this.transactions.TryGetValue(json, out var transaction))
        {
            return transaction;
        }

        throw new InvalidOperationException($"No registered transaction for JSON: '{json}'");
    }
}
