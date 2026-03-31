using System.Text.Json;
using TownCrier.Application.Authorities;
using TownCrier.Domain.Authorities;

namespace TownCrier.Infrastructure.PlanIt;

public sealed class CachedPlanItAuthorityProvider : IAuthorityProvider, IDisposable
{
    private static readonly TimeSpan DefaultRefreshInterval = TimeSpan.FromHours(24);

    private readonly HttpClient httpClient;
    private readonly SemaphoreSlim refreshLock = new(1, 1);
    private readonly TimeSpan refreshInterval;
    private readonly TimeProvider timeProvider;

    private IReadOnlyList<Authority>? cachedAuthorities;
    private DateTimeOffset lastRefresh = DateTimeOffset.MinValue;

    public CachedPlanItAuthorityProvider(
        HttpClient httpClient,
        TimeProvider timeProvider,
        TimeSpan? refreshInterval = null)
    {
        this.httpClient = httpClient;
        this.timeProvider = timeProvider;
        this.refreshInterval = refreshInterval ?? DefaultRefreshInterval;
    }

    public async Task<IReadOnlyList<Authority>> GetAllAsync(CancellationToken ct)
    {
        await this.EnsureCacheAsync(ct).ConfigureAwait(false);
        return this.cachedAuthorities!;
    }

    public async Task<Authority?> GetByIdAsync(int id, CancellationToken ct)
    {
        await this.EnsureCacheAsync(ct).ConfigureAwait(false);
        return this.cachedAuthorities!.FirstOrDefault(a => a.Id == id);
    }

    public void Dispose()
    {
        this.refreshLock.Dispose();
    }

    private async Task EnsureCacheAsync(CancellationToken ct)
    {
        var now = this.timeProvider.GetUtcNow();
        if (this.cachedAuthorities is not null && now - this.lastRefresh < this.refreshInterval)
        {
            return;
        }

        await this.refreshLock.WaitAsync(ct).ConfigureAwait(false);
        try
        {
            // Double-check after acquiring lock
            now = this.timeProvider.GetUtcNow();
            if (this.cachedAuthorities is not null && now - this.lastRefresh < this.refreshInterval)
            {
                return;
            }

            var url = new Uri("/api/areas/json?pg_sz=500&select=area_id,area_name,area_type,url,planning_url", UriKind.Relative);
            using var response = await this.httpClient.GetAsync(url, ct).ConfigureAwait(false);
            response.EnsureSuccessStatusCode();

            var areasResponse = await JsonSerializer.DeserializeAsync(
                await response.Content.ReadAsStreamAsync(ct).ConfigureAwait(false),
                PlanItJsonSerializerContext.Default.PlanItAreasResponse,
                ct).ConfigureAwait(false);

            if (areasResponse is null)
            {
                this.cachedAuthorities ??= [];
                return;
            }

            this.cachedAuthorities = areasResponse.Records
                .Select(r => new Authority(r.Id, r.Name, r.AreaType, r.CouncilUrl, r.PlanningUrl))
                .OrderBy(a => a.Name, StringComparer.OrdinalIgnoreCase)
                .ToList()
                .AsReadOnly();

            this.lastRefresh = now;
        }
        finally
        {
            this.refreshLock.Release();
        }
    }
}
