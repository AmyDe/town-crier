using System.Diagnostics.CodeAnalysis;
using TownCrier.Domain.PlanningApplications;
using TownCrier.Infrastructure.PlanningApplications;

namespace TownCrier.Infrastructure.SavedApplications;

/// <summary>
/// Embedded planning-application snapshot stored on a saved-application document.
/// Mirrors <see cref="PlanningApplicationDocument"/> shape but lives inline so the
/// saved-list endpoint can render with one partitioned query — no cross-partition
/// fan-out hydration. See bd tc-udby.
/// </summary>
internal sealed class SavedApplicationSnapshotDocument
{
    public required string Name { get; init; }

    public required string Uid { get; init; }

    public required string AreaName { get; init; }

    public required int AreaId { get; init; }

    public required string Address { get; init; }

    public string? Postcode { get; init; }

    public required string Description { get; init; }

    public string? AppType { get; init; }

    public string? AppState { get; init; }

    public string? AppSize { get; init; }

    public DateOnly? StartDate { get; init; }

    public DateOnly? DecidedDate { get; init; }

    public DateOnly? ConsultedDate { get; init; }

    public GeoJsonPoint? Location { get; init; }

    [SuppressMessage("Design", "CA1056:URI properties should not be strings", Justification = "PlanIt API returns URLs as strings")]
    public string? Url { get; init; }

    public string? Link { get; init; }

    public required DateTimeOffset LastDifferent { get; init; }

    public static SavedApplicationSnapshotDocument FromDomain(PlanningApplication application)
    {
        ArgumentNullException.ThrowIfNull(application);

        return new SavedApplicationSnapshotDocument
        {
            Name = application.Name,
            Uid = application.Uid,
            AreaName = application.AreaName,
            AreaId = application.AreaId,
            Address = application.Address,
            Postcode = application.Postcode,
            Description = application.Description,
            AppType = application.AppType,
            AppState = application.AppState,
            AppSize = application.AppSize,
            StartDate = application.StartDate,
            DecidedDate = application.DecidedDate,
            ConsultedDate = application.ConsultedDate,
            Location = application.Latitude.HasValue && application.Longitude.HasValue
                ? new GeoJsonPoint { Coordinates = [application.Longitude.Value, application.Latitude.Value] }
                : null,
            Url = application.Url,
            Link = application.Link,
            LastDifferent = application.LastDifferent,
        };
    }

    public PlanningApplication ToDomain()
    {
        return new PlanningApplication(
            name: this.Name,
            uid: this.Uid,
            areaName: this.AreaName,
            areaId: this.AreaId,
            address: this.Address,
            postcode: this.Postcode,
            description: this.Description,
            appType: this.AppType,
            appState: this.AppState,
            appSize: this.AppSize,
            startDate: this.StartDate,
            decidedDate: this.DecidedDate,
            consultedDate: this.ConsultedDate,
            longitude: this.Location?.Coordinates.Length >= 2 ? this.Location.Coordinates[0] : null,
            latitude: this.Location?.Coordinates.Length >= 2 ? this.Location.Coordinates[1] : null,
            url: this.Url,
            link: this.Link,
            lastDifferent: this.LastDifferent);
    }
}
