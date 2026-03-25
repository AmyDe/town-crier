using TownCrier.Application.Authorities;

namespace TownCrier.Application.Tests.WatchZones;

internal sealed class FakeAuthorityResolver : IAuthorityResolver
{
    private readonly Dictionary<(double Latitude, double Longitude), int> mappings = [];

    public int CallCount { get; private set; }

    public void Add(double latitude, double longitude, int authorityId)
    {
        this.mappings[(latitude, longitude)] = authorityId;
    }

    public Task<int> ResolveFromCoordinatesAsync(double latitude, double longitude, CancellationToken ct)
    {
        this.CallCount++;

        if (this.mappings.TryGetValue((latitude, longitude), out var authorityId))
        {
            return Task.FromResult(authorityId);
        }

        throw new InvalidOperationException($"No authority found for coordinates ({latitude}, {longitude})");
    }
}
