using TownCrier.Application.Designations;
using TownCrier.Domain.Designations;

namespace TownCrier.Application.Tests.Designations;

internal sealed class FakeDesignationDataProvider : IDesignationDataProvider
{
    private readonly Dictionary<(double Latitude, double Longitude), DesignationContext> store = new();

    public void Add(double latitude, double longitude, DesignationContext context)
    {
        this.store[(latitude, longitude)] = context;
    }

    public Task<DesignationContext> GetDesignationsAsync(
        double latitude,
        double longitude,
        CancellationToken ct)
    {
        if (this.store.TryGetValue((latitude, longitude), out var context))
        {
            return Task.FromResult(context);
        }

        return Task.FromResult(DesignationContext.None);
    }
}
