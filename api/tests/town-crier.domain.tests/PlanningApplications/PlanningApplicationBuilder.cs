using TownCrier.Domain.PlanningApplications;

namespace TownCrier.Domain.Tests.PlanningApplications;

internal sealed class PlanningApplicationBuilder
{
    private string name = "Council/app-1";
    private string uid = "uid-1";
    private string areaName = "Test Council";
    private int areaId = 1;
    private string address = "123 Test Street";
    private string? postcode = "SW1A 1AA";
    private string description = "Test planning application";
    private string? appType = "Full";
    private string? appState = "Undecided";
    private string? appSize = "Small";
    private DateOnly? startDate = new DateOnly(2026, 1, 1);
    private DateOnly? decidedDate;
    private DateOnly? consultedDate = new DateOnly(2026, 2, 1);
    private double? longitude = -0.1278;
    private double? latitude = 51.5074;
    private string? url;
    private string? link;
    private DateTimeOffset lastDifferent = new(2026, 3, 1, 0, 0, 0, TimeSpan.Zero);

    public PlanningApplicationBuilder WithName(string value)
    {
        this.name = value;
        return this;
    }

    public PlanningApplicationBuilder WithUid(string value)
    {
        this.uid = value;
        return this;
    }

    public PlanningApplicationBuilder WithAreaName(string value)
    {
        this.areaName = value;
        return this;
    }

    public PlanningApplicationBuilder WithAreaId(int value)
    {
        this.areaId = value;
        return this;
    }

    public PlanningApplicationBuilder WithAddress(string value)
    {
        this.address = value;
        return this;
    }

    public PlanningApplicationBuilder WithPostcode(string? value)
    {
        this.postcode = value;
        return this;
    }

    public PlanningApplicationBuilder WithDescription(string value)
    {
        this.description = value;
        return this;
    }

    public PlanningApplicationBuilder WithAppType(string? value)
    {
        this.appType = value;
        return this;
    }

    public PlanningApplicationBuilder WithAppState(string? value)
    {
        this.appState = value;
        return this;
    }

    public PlanningApplicationBuilder WithAppSize(string? value)
    {
        this.appSize = value;
        return this;
    }

    public PlanningApplicationBuilder WithStartDate(DateOnly? value)
    {
        this.startDate = value;
        return this;
    }

    public PlanningApplicationBuilder WithDecidedDate(DateOnly? value)
    {
        this.decidedDate = value;
        return this;
    }

    public PlanningApplicationBuilder WithConsultedDate(DateOnly? value)
    {
        this.consultedDate = value;
        return this;
    }

    public PlanningApplicationBuilder WithCoordinates(double? latitude, double? longitude)
    {
        this.latitude = latitude;
        this.longitude = longitude;
        return this;
    }

    public PlanningApplicationBuilder WithUrl(string? value)
    {
        this.url = value;
        return this;
    }

    public PlanningApplicationBuilder WithLink(string? value)
    {
        this.link = value;
        return this;
    }

    public PlanningApplicationBuilder WithLastDifferent(DateTimeOffset value)
    {
        this.lastDifferent = value;
        return this;
    }

    public PlanningApplication Build()
    {
        return new PlanningApplication(
            name: this.name,
            uid: this.uid,
            areaName: this.areaName,
            areaId: this.areaId,
            address: this.address,
            postcode: this.postcode,
            description: this.description,
            appType: this.appType,
            appState: this.appState,
            appSize: this.appSize,
            startDate: this.startDate,
            decidedDate: this.decidedDate,
            consultedDate: this.consultedDate,
            longitude: this.longitude,
            latitude: this.latitude,
            url: this.url,
            link: this.link,
            lastDifferent: this.lastDifferent);
    }
}
