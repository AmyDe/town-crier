using TownCrier.Domain.PlanningApplications;

namespace TownCrier.Application.DemoAccount;

public static class DemoSeedData
{
    public const string AuthorityName = "Westminster City Council";
    public const int AuthorityId = 441;

    public static IReadOnlyList<PlanningApplication> CreateApplications(DateTimeOffset lastDifferent)
    {
        return
        [
            new PlanningApplication(
                name: "24/05678/FULL",
                uid: "demo-app-001",
                areaName: AuthorityName,
                areaId: AuthorityId,
                address: "10 Downing Street, London SW1A 2AA",
                postcode: "SW1A 2AA",
                description: "Replacement of entrance door and associated security improvements",
                appType: "Full",
                appState: "Under consideration",
                appSize: null,
                startDate: new DateOnly(2026, 1, 15),
                decidedDate: null,
                consultedDate: new DateOnly(2026, 2, 1),
                longitude: -0.1276,
                latitude: 51.5034,
                url: null,
                link: null,
                lastDifferent: lastDifferent),
            new PlanningApplication(
                name: "24/05679/FULL",
                uid: "demo-app-002",
                areaName: AuthorityName,
                areaId: AuthorityId,
                address: "Westminster Abbey, 20 Deans Yd, London SW1P 3PA",
                postcode: "SW1P 3PA",
                description: "Installation of new lighting system in the nave and restoration of stonework",
                appType: "Listed Building",
                appState: "Approved",
                appSize: null,
                startDate: new DateOnly(2025, 11, 1),
                decidedDate: new DateOnly(2026, 2, 20),
                consultedDate: new DateOnly(2025, 12, 1),
                longitude: -0.1273,
                latitude: 51.4993,
                url: null,
                link: null,
                lastDifferent: lastDifferent),
            new PlanningApplication(
                name: "24/05680/FULL",
                uid: "demo-app-003",
                areaName: AuthorityName,
                areaId: AuthorityId,
                address: "Buckingham Palace, London SW1A 1AA",
                postcode: "SW1A 1AA",
                description: "Erection of temporary scaffolding for facade cleaning and minor repairs to roof drainage",
                appType: "Full",
                appState: "Under consideration",
                appSize: null,
                startDate: new DateOnly(2026, 2, 10),
                decidedDate: null,
                consultedDate: null,
                longitude: -0.1419,
                latitude: 51.5014,
                url: null,
                link: null,
                lastDifferent: lastDifferent),
            new PlanningApplication(
                name: "24/05681/FULL",
                uid: "demo-app-004",
                areaName: AuthorityName,
                areaId: AuthorityId,
                address: "St James's Park, London SW1A 2BJ",
                postcode: "SW1A 2BJ",
                description: "Construction of new accessible footpath and installation of park benches",
                appType: "Full",
                appState: "Refused",
                appSize: null,
                startDate: new DateOnly(2025, 10, 5),
                decidedDate: new DateOnly(2026, 1, 30),
                consultedDate: new DateOnly(2025, 11, 1),
                longitude: -0.1340,
                latitude: 51.5025,
                url: null,
                link: null,
                lastDifferent: lastDifferent),
        ];
    }
}
