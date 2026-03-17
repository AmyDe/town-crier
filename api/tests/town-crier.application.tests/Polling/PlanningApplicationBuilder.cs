using TownCrier.Domain.PlanningApplications;

namespace TownCrier.Application.Tests.Polling;

internal sealed class PlanningApplicationBuilder
{
    private readonly string address = "123 Test Street";
    private readonly string? postcode = "SW1A 1AA";
    private readonly string description = "Test planning application";
    private readonly string appType = "Full";
    private readonly DateTimeOffset lastDifferent = DateTimeOffset.UtcNow;
    private string areaName = "Test Council";
    private int areaId = 1;
    private string name = "Test Application";
    private string uid = "test-uid-001";
    private string appState = "Undecided";
    private double? latitude;
    private double? longitude;

    public PlanningApplicationBuilder WithUid(string uid)
    {
        this.uid = uid;
        return this;
    }

    public PlanningApplicationBuilder WithName(string name)
    {
        this.name = name;
        return this;
    }

    public PlanningApplicationBuilder WithAreaId(int areaId)
    {
        this.areaId = areaId;
        return this;
    }

    public PlanningApplicationBuilder WithAreaName(string areaName)
    {
        this.areaName = areaName;
        return this;
    }

    public PlanningApplicationBuilder WithAppState(string appState)
    {
        this.appState = appState;
        return this;
    }

    public PlanningApplicationBuilder WithCoordinates(double latitude, double longitude)
    {
        this.latitude = latitude;
        this.longitude = longitude;
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
            appSize: null,
            startDate: null,
            decidedDate: null,
            consultedDate: null,
            longitude: this.longitude,
            latitude: this.latitude,
            url: null,
            link: null,
            lastDifferent: this.lastDifferent);
    }
}
