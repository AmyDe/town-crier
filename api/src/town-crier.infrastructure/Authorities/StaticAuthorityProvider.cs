using System.Text.Json;
using TownCrier.Application.Authorities;
using TownCrier.Domain.Authorities;

namespace TownCrier.Infrastructure.Authorities;

public sealed class StaticAuthorityProvider : IAuthorityProvider
{
    private readonly IReadOnlyList<Authority> authorities;
    private readonly Dictionary<int, Authority> authoritiesById;

    public StaticAuthorityProvider()
    {
        var records = LoadEmbeddedAuthorities();

        this.authorities = records
            .Select(r => new Authority(r.Id, r.Name, r.AreaType, councilUrl: null, planningUrl: null))
            .OrderBy(a => a.Name, StringComparer.OrdinalIgnoreCase)
            .ToList()
            .AsReadOnly();

        this.authoritiesById = this.authorities.ToDictionary(a => a.Id);
    }

    public Task<IReadOnlyList<Authority>> GetAllAsync(CancellationToken ct)
    {
        return Task.FromResult(this.authorities);
    }

    public Task<Authority?> GetByIdAsync(int id, CancellationToken ct)
    {
        this.authoritiesById.TryGetValue(id, out var authority);
        return Task.FromResult(authority);
    }

    private static List<AuthorityRecord> LoadEmbeddedAuthorities()
    {
        var assembly = typeof(StaticAuthorityProvider).Assembly;
        const string resourceName = "TownCrier.Infrastructure.Authorities.authorities.json";

        using var stream = assembly.GetManifestResourceStream(resourceName)
            ?? throw new InvalidOperationException($"Embedded resource '{resourceName}' not found.");

        return JsonSerializer.Deserialize(stream, AuthorityJsonSerializerContext.Default.ListAuthorityRecord)
            ?? throw new InvalidOperationException("Failed to deserialize authorities.");
    }
}
