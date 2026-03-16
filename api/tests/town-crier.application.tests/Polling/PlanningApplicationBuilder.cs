using TownCrier.Domain.PlanningApplications;

namespace TownCrier.Application.Tests.Polling;

internal sealed class PlanningApplicationBuilder
{
    private readonly string areaName = "Test Council";
    private readonly int areaId = 1;
    private readonly string address = "123 Test Street";
    private readonly string? postcode = "SW1A 1AA";
    private readonly string description = "Test planning application";
    private readonly string appType = "Full";
    private readonly DateTimeOffset lastDifferent = DateTimeOffset.UtcNow;
    private string name = "Test Application";
    private string uid = "test-uid-001";
    private string appState = "Undecided";

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

    public PlanningApplicationBuilder WithAppState(string appState)
    {
        this.appState = appState;
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
            longitude: null,
            latitude: null,
            url: null,
            link: null,
            lastDifferent: this.lastDifferent);
    }
}
