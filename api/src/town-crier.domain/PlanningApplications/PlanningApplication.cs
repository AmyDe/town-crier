using System.Diagnostics.CodeAnalysis;

namespace TownCrier.Domain.PlanningApplications;

public sealed class PlanningApplication
{
    [SuppressMessage("Design", "CA1054:URI parameters should not be strings", Justification = "PlanIt API returns URLs as strings")]
    public PlanningApplication(
        string name,
        string uid,
        string areaName,
        int areaId,
        string address,
        string? postcode,
        string description,
        string? appType,
        string? appState,
        string? appSize,
        DateOnly? startDate,
        DateOnly? decidedDate,
        DateOnly? consultedDate,
        double? longitude,
        double? latitude,
        string? url,
        string? link,
        DateTimeOffset lastDifferent)
    {
        this.Name = name;
        this.Uid = uid;
        this.AreaName = areaName;
        this.AreaId = areaId;
        this.Address = address;
        this.Postcode = postcode;
        this.Description = description;
        this.AppType = appType;
        this.AppState = appState;
        this.AppSize = appSize;
        this.StartDate = startDate;
        this.DecidedDate = decidedDate;
        this.ConsultedDate = consultedDate;
        this.Longitude = longitude;
        this.Latitude = latitude;
        this.Url = url;
        this.Link = link;
        this.LastDifferent = lastDifferent;
    }

    public string Name { get; }

    public string Uid { get; }

    public string AreaName { get; }

    public int AreaId { get; }

    public string Address { get; }

    public string? Postcode { get; }

    public string Description { get; }

    public string? AppType { get; }

    public string? AppState { get; }

    public string? AppSize { get; }

    public DateOnly? StartDate { get; }

    public DateOnly? DecidedDate { get; }

    public DateOnly? ConsultedDate { get; }

    public double? Longitude { get; }

    public double? Latitude { get; }

    [SuppressMessage("Design", "CA1056:URI properties should not be strings", Justification = "PlanIt API returns URLs as strings")]
    public string? Url { get; }

    public string? Link { get; }

    public DateTimeOffset LastDifferent { get; }

    /// <summary>
    /// Compares business-material fields against another instance. Excludes PlanIt bookkeeping
    /// (LastDifferent) which changes on every rescrape even when content is identical.
    /// Used by the poll cycle to skip redundant upserts and zone lookups when PlanIt returns
    /// the same application with only a bumped LastDifferent timestamp.
    /// </summary>
    public bool HasSameBusinessFieldsAs(PlanningApplication other)
    {
        ArgumentNullException.ThrowIfNull(other);

        return this.Name == other.Name
            && this.Uid == other.Uid
            && this.AreaName == other.AreaName
            && this.AreaId == other.AreaId
            && this.Address == other.Address
            && this.Postcode == other.Postcode
            && this.Description == other.Description
            && this.AppType == other.AppType
            && this.AppState == other.AppState
            && this.AppSize == other.AppSize
            && this.StartDate == other.StartDate
            && this.DecidedDate == other.DecidedDate
            && this.ConsultedDate == other.ConsultedDate
            && Nullable.Equals(this.Longitude, other.Longitude)
            && Nullable.Equals(this.Latitude, other.Latitude)
            && this.Url == other.Url
            && this.Link == other.Link;
    }
}
