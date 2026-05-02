using Microsoft.Extensions.Logging;
using Microsoft.Extensions.Logging.Abstractions;
using TownCrier.Application.PlanIt;
using TownCrier.Application.SavedApplications;
using TownCrier.Domain.PlanningApplications;
using TownCrier.Domain.SavedApplications;

namespace TownCrier.Application.PlanningApplications;

public sealed class GetApplicationByUidQueryHandler
{
    private static readonly Action<ILogger, string, string, Exception?> LogRefreshFailed =
        LoggerMessage.Define<string, string>(
            LogLevel.Warning,
            new EventId(1, nameof(LogRefreshFailed)),
            "Refresh-on-tap failed for user {UserId}, uid {Uid}.");

    private readonly IPlanningApplicationRepository repository;
    private readonly IPlanItClient planItClient;
    private readonly ISavedApplicationRepository savedApplicationRepository;
    private readonly ILogger<GetApplicationByUidQueryHandler> logger;

    public GetApplicationByUidQueryHandler(
        IPlanningApplicationRepository repository,
        IPlanItClient planItClient,
        ISavedApplicationRepository savedApplicationRepository)
        : this(repository, planItClient, savedApplicationRepository, NullLogger<GetApplicationByUidQueryHandler>.Instance)
    {
    }

    public GetApplicationByUidQueryHandler(
        IPlanningApplicationRepository repository,
        IPlanItClient planItClient,
        ISavedApplicationRepository savedApplicationRepository,
        ILogger<GetApplicationByUidQueryHandler> logger)
    {
        this.repository = repository;
        this.planItClient = planItClient;
        this.savedApplicationRepository = savedApplicationRepository;
        this.logger = logger;
    }

    public async Task<PlanningApplicationResult?> HandleAsync(GetApplicationByUidQuery query, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(query);

        var application = await this.repository.GetByUidAsync(query.Uid, ct).ConfigureAwait(false);
        if (application is null)
        {
            // Cosmos miss: fall back to PlanIt's per-application endpoint. This
            // closes the gap when Cosmos has not yet ingested an application that
            // a saved-list snapshot or a deep link references.
            var fetched = await this.planItClient.GetByUidAsync(query.Uid, ct).ConfigureAwait(false);
            if (fetched is null)
            {
                return null;
            }

            await this.repository.UpsertAsync(fetched, ct).ConfigureAwait(false);
            application = fetched;
        }

        // Refresh-on-tap: if the requesting user has this application saved,
        // silently upsert the fresh snapshot back into their saved row so the
        // saved-list self-heals on the items they actually engage with. Failure
        // to refresh must NOT fail the read. See bd tc-udby.
        if (!string.IsNullOrEmpty(query.UserId))
        {
            await this.TryRefreshSavedSnapshotAsync(query.UserId, application, ct).ConfigureAwait(false);
        }

        return ToResult(application);
    }

    internal static PlanningApplicationResult ToResult(PlanningApplication application)
    {
        return new PlanningApplicationResult(
            application.Name,
            application.Uid,
            application.AreaName,
            application.AreaId,
            application.Address,
            application.Postcode,
            application.Description,
            application.AppType,
            application.AppState,
            application.AppSize,
            application.StartDate,
            application.DecidedDate,
            application.ConsultedDate,
            application.Longitude,
            application.Latitude,
            application.Url,
            application.Link,
            application.LastDifferent);
    }

    private async Task TryRefreshSavedSnapshotAsync(string userId, PlanningApplication application, CancellationToken ct)
    {
        try
        {
            var hasSaved = await this.savedApplicationRepository.ExistsAsync(userId, application.Uid, ct).ConfigureAwait(false);
            if (!hasSaved)
            {
                return;
            }

            // Preserve the original SavedAt by reading the existing row. If the
            // read fails or the row has vanished mid-flight, fall back to a
            // fresh save with the current time — better stale than to bubble.
            var rows = await this.savedApplicationRepository.GetByUserIdAsync(userId, ct).ConfigureAwait(false);
            var existing = rows.FirstOrDefault(r =>
                string.Equals(r.ApplicationUid, application.Uid, StringComparison.Ordinal));
            var savedAt = existing?.SavedAt ?? DateTimeOffset.UtcNow;

            var refreshed = SavedApplication.Create(userId, application, savedAt);
            await this.savedApplicationRepository.SaveAsync(refreshed, ct).ConfigureAwait(false);
        }
        catch (OperationCanceledException)
        {
            throw;
        }
#pragma warning disable CA1031 // Do not catch general exception types
        catch (Exception ex)
#pragma warning restore CA1031
        {
            // Refresh-on-tap is a side effect; it must never fail the user's read.
            LogRefreshFailed(this.logger, userId, application.Uid, ex);
        }
    }
}
