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
        string appType,
        string appState,
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

    public string AppType { get; }

    public string AppState { get; }

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
}
