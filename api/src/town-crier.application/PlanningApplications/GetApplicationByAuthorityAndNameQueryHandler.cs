using Microsoft.Extensions.Logging;
using Microsoft.Extensions.Logging.Abstractions;
using TownCrier.Application.SavedApplications;
using TownCrier.Domain.PlanningApplications;
using TownCrier.Domain.SavedApplications;

namespace TownCrier.Application.PlanningApplications;

public sealed class GetApplicationByAuthorityAndNameQueryHandler
{
    private static readonly Action<ILogger, string, string, Exception?> LogRefreshFailed =
        LoggerMessage.Define<string, string>(
            LogLevel.Warning,
            new EventId(1, nameof(LogRefreshFailed)),
            "Refresh-on-tap failed for user {UserId}, application {Name}.");

    private readonly IPlanningApplicationRepository repository;
    private readonly ISavedApplicationRepository savedApplicationRepository;
    private readonly ILogger<GetApplicationByAuthorityAndNameQueryHandler> logger;

    public GetApplicationByAuthorityAndNameQueryHandler(
        IPlanningApplicationRepository repository,
        ISavedApplicationRepository savedApplicationRepository)
        : this(repository, savedApplicationRepository, NullLogger<GetApplicationByAuthorityAndNameQueryHandler>.Instance)
    {
    }

    public GetApplicationByAuthorityAndNameQueryHandler(
        IPlanningApplicationRepository repository,
        ISavedApplicationRepository savedApplicationRepository,
        ILogger<GetApplicationByAuthorityAndNameQueryHandler> logger)
    {
        this.repository = repository;
        this.savedApplicationRepository = savedApplicationRepository;
        this.logger = logger;
    }

    public async Task<PlanningApplicationResult?> HandleAsync(
        GetApplicationByAuthorityAndNameQuery query, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(query);

        // Partitioned point read — ~1 RU. No PlanIt fallback: if the application
        // is not in Cosmos the polling worker hasn't ingested it yet; return 404
        // and let the user retry. Fallbacks to PlanIt are removed from all user
        // paths (GH#395 Invariant 1).
        var application = await this.repository
            .GetByAuthorityAndNameAsync(query.AuthorityCode, query.Name, ct)
            .ConfigureAwait(false);

        if (application is null)
        {
            return null;
        }

        // Refresh-on-tap: same side-effect as the uid endpoint (bd tc-udby).
        if (!string.IsNullOrEmpty(query.UserId))
        {
            await this.TryRefreshSavedSnapshotAsync(query.UserId, application, ct).ConfigureAwait(false);
        }

        return GetApplicationByUidQueryHandler.ToResult(application);
    }

    private async Task TryRefreshSavedSnapshotAsync(
        string userId, PlanningApplication application, CancellationToken ct)
    {
        try
        {
            // Saved rows are keyed on the canonical {areaId}/{name} uid, not the
            // master record's raw Uid field — align the lookup on CanonicalUid so
            // snapshot healing fires for stale-format saves (bd tc-o88i).
            var canonicalUid = application.CanonicalUid;
            var hasSaved = await this.savedApplicationRepository
                .ExistsAsync(userId, canonicalUid, ct).ConfigureAwait(false);

            if (!hasSaved)
            {
                return;
            }

            var rows = await this.savedApplicationRepository
                .GetByUserIdAsync(userId, ct).ConfigureAwait(false);

            var existing = rows.FirstOrDefault(r =>
                string.Equals(r.ApplicationUid, canonicalUid, StringComparison.Ordinal));
            var savedAt = existing?.SavedAt ?? DateTimeOffset.UtcNow;

            var refreshed = SavedApplication.Create(userId, application, savedAt);
            await this.savedApplicationRepository.SaveAsync(refreshed, ct).ConfigureAwait(false);
        }
        catch (OperationCanceledException)
        {
            throw;
        }
#pragma warning disable CA1031 // Refresh-on-tap is a side effect; must never fail the read.
        catch (Exception ex)
#pragma warning restore CA1031
        {
            LogRefreshFailed(this.logger, userId, application.Name, ex);
        }
    }
}
