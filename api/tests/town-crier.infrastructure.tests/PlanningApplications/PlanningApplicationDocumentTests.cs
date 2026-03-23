using System.Diagnostics.CodeAnalysis;
using TownCrier.Domain.PlanningApplications;
using TownCrier.Infrastructure.PlanningApplications;

namespace TownCrier.Infrastructure.Tests.PlanningApplications;

[SuppressMessage("Design", "S1075:Hardcoded URIs", Justification = "Test data")]
public sealed class PlanningApplicationDocumentTests
{
    [Test]
    public async Task Should_MapAllFields_When_FromDomainCalled()
    {
        // Arrange
        var application = new PlanningApplication(
            name: "Westminster/12345",
            uid: "12345",
            areaName: "Westminster",
            areaId: 42,
            address: "10 Downing Street",
            postcode: "SW1A 2AA",
            description: "Extension to rear",
            appType: "Full",
            appState: "Undecided",
            appSize: "Small",
            startDate: new DateOnly(2026, 1, 15),
            decidedDate: new DateOnly(2026, 3, 1),
            consultedDate: new DateOnly(2026, 2, 1),
            longitude: -0.1276,
            latitude: 51.5034,
            url: "https://planit.org.uk/app/12345",
            link: "https://planit.org.uk/link/12345",
            lastDifferent: new DateTimeOffset(2026, 3, 16, 12, 0, 0, TimeSpan.Zero));

        // Act
        var document = PlanningApplicationDocument.FromDomain(application);

        // Assert
        await Assert.That(document.Id).IsEqualTo("Westminster/12345");
        await Assert.That(document.AuthorityCode).IsEqualTo("42");
        await Assert.That(document.PlanitName).IsEqualTo("Westminster/12345");
        await Assert.That(document.Uid).IsEqualTo("12345");
        await Assert.That(document.AreaName).IsEqualTo("Westminster");
        await Assert.That(document.AreaId).IsEqualTo(42);
        await Assert.That(document.Address).IsEqualTo("10 Downing Street");
        await Assert.That(document.Postcode).IsEqualTo("SW1A 2AA");
        await Assert.That(document.Description).IsEqualTo("Extension to rear");
        await Assert.That(document.AppType).IsEqualTo("Full");
        await Assert.That(document.AppState).IsEqualTo("Undecided");
        await Assert.That(document.AppSize).IsEqualTo("Small");
        await Assert.That(document.StartDate).IsEqualTo(new DateOnly(2026, 1, 15));
        await Assert.That(document.DecidedDate).IsEqualTo(new DateOnly(2026, 3, 1));
        await Assert.That(document.ConsultedDate).IsEqualTo(new DateOnly(2026, 2, 1));
        await Assert.That(document.Url).IsEqualTo("https://planit.org.uk/app/12345");
        await Assert.That(document.Link).IsEqualTo("https://planit.org.uk/link/12345");
        await Assert.That(document.LastDifferent).IsEqualTo(new DateTimeOffset(2026, 3, 16, 12, 0, 0, TimeSpan.Zero));
    }

    [Test]
    public async Task Should_CreateGeoJsonPoint_When_CoordinatesPresent()
    {
        // Arrange
        var application = new PlanningApplication(
            name: "Westminster/12345",
            uid: "12345",
            areaName: "Westminster",
            areaId: 42,
            address: "10 Downing Street",
            postcode: null,
            description: "Extension",
            appType: "Full",
            appState: "Undecided",
            appSize: null,
            startDate: null,
            decidedDate: null,
            consultedDate: null,
            longitude: -0.1276,
            latitude: 51.5034,
            url: null,
            link: null,
            lastDifferent: DateTimeOffset.UtcNow);

        // Act
        var document = PlanningApplicationDocument.FromDomain(application);

        // Assert — GeoJSON uses [longitude, latitude] order
        await Assert.That(document.Location).IsNotNull();
        await Assert.That(document.Location!.Type).IsEqualTo("Point");
        await Assert.That(document.Location.Coordinates[0]).IsEqualTo(-0.1276);
        await Assert.That(document.Location.Coordinates[1]).IsEqualTo(51.5034);
    }

    [Test]
    public async Task Should_SetLocationNull_When_CoordinatesMissing()
    {
        // Arrange
        var application = new PlanningApplication(
            name: "Westminster/12345",
            uid: "12345",
            areaName: "Westminster",
            areaId: 42,
            address: "10 Downing Street",
            postcode: null,
            description: "Extension",
            appType: "Full",
            appState: "Undecided",
            appSize: null,
            startDate: null,
            decidedDate: null,
            consultedDate: null,
            longitude: null,
            latitude: null,
            url: null,
            link: null,
            lastDifferent: DateTimeOffset.UtcNow);

        // Act
        var document = PlanningApplicationDocument.FromDomain(application);

        // Assert
        await Assert.That(document.Location).IsNull();
    }

    [Test]
    public async Task Should_MapBackToDomain_When_ToDomainCalled()
    {
        // Arrange
        var document = new PlanningApplicationDocument
        {
            Id = "Westminster/12345",
            AuthorityCode = "42",
            PlanitName = "Westminster/12345",
            Uid = "12345",
            AreaName = "Westminster",
            AreaId = 42,
            Address = "10 Downing Street",
            Postcode = "SW1A 2AA",
            Description = "Extension to rear",
            AppType = "Full",
            AppState = "Undecided",
            AppSize = "Small",
            StartDate = new DateOnly(2026, 1, 15),
            DecidedDate = new DateOnly(2026, 3, 1),
            ConsultedDate = new DateOnly(2026, 2, 1),
            Location = new GeoJsonPoint { Type = "Point", Coordinates = [-0.1276, 51.5034] },
            Url = "https://planit.org.uk/app/12345",
            Link = "https://planit.org.uk/link/12345",
            LastDifferent = new DateTimeOffset(2026, 3, 16, 12, 0, 0, TimeSpan.Zero),
        };

        // Act
        var application = document.ToDomain();

        // Assert
        await Assert.That(application.Name).IsEqualTo("Westminster/12345");
        await Assert.That(application.Uid).IsEqualTo("12345");
        await Assert.That(application.AreaName).IsEqualTo("Westminster");
        await Assert.That(application.AreaId).IsEqualTo(42);
        await Assert.That(application.Address).IsEqualTo("10 Downing Street");
        await Assert.That(application.Postcode).IsEqualTo("SW1A 2AA");
        await Assert.That(application.Description).IsEqualTo("Extension to rear");
        await Assert.That(application.AppType).IsEqualTo("Full");
        await Assert.That(application.AppState).IsEqualTo("Undecided");
        await Assert.That(application.AppSize).IsEqualTo("Small");
        await Assert.That(application.StartDate).IsEqualTo(new DateOnly(2026, 1, 15));
        await Assert.That(application.DecidedDate).IsEqualTo(new DateOnly(2026, 3, 1));
        await Assert.That(application.ConsultedDate).IsEqualTo(new DateOnly(2026, 2, 1));
        await Assert.That(application.Longitude).IsEqualTo(-0.1276);
        await Assert.That(application.Latitude).IsEqualTo(51.5034);
        await Assert.That(application.Url).IsEqualTo("https://planit.org.uk/app/12345");
        await Assert.That(application.Link).IsEqualTo("https://planit.org.uk/link/12345");
        await Assert.That(application.LastDifferent).IsEqualTo(new DateTimeOffset(2026, 3, 16, 12, 0, 0, TimeSpan.Zero));
    }

    [Test]
    public async Task Should_MapNullCoordinates_When_LocationIsNull()
    {
        // Arrange
        var document = new PlanningApplicationDocument
        {
            Id = "Westminster/12345",
            AuthorityCode = "42",
            PlanitName = "Westminster/12345",
            Uid = "12345",
            AreaName = "Westminster",
            AreaId = 42,
            Address = "10 Downing Street",
            Postcode = null,
            Description = "Extension",
            AppType = "Full",
            AppState = "Undecided",
            AppSize = null,
            StartDate = null,
            DecidedDate = null,
            ConsultedDate = null,
            Location = null,
            Url = null,
            Link = null,
            LastDifferent = DateTimeOffset.UtcNow,
        };

        // Act
        var application = document.ToDomain();

        // Assert
        await Assert.That(application.Longitude).IsNull();
        await Assert.That(application.Latitude).IsNull();
    }

    [Test]
    public async Task Should_RoundTrip_When_FromDomainThenToDomain()
    {
        // Arrange
        var original = new PlanningApplication(
            name: "Westminster/12345",
            uid: "12345",
            areaName: "Westminster",
            areaId: 42,
            address: "10 Downing Street",
            postcode: "SW1A 2AA",
            description: "Extension to rear",
            appType: "Full",
            appState: "Undecided",
            appSize: "Small",
            startDate: new DateOnly(2026, 1, 15),
            decidedDate: new DateOnly(2026, 3, 1),
            consultedDate: new DateOnly(2026, 2, 1),
            longitude: -0.1276,
            latitude: 51.5034,
            url: "https://planit.org.uk/app/12345",
            link: "https://planit.org.uk/link/12345",
            lastDifferent: new DateTimeOffset(2026, 3, 16, 12, 0, 0, TimeSpan.Zero));

        // Act
        var roundTripped = PlanningApplicationDocument.FromDomain(original).ToDomain();

        // Assert
        await Assert.That(roundTripped.Name).IsEqualTo(original.Name);
        await Assert.That(roundTripped.Uid).IsEqualTo(original.Uid);
        await Assert.That(roundTripped.AreaName).IsEqualTo(original.AreaName);
        await Assert.That(roundTripped.AreaId).IsEqualTo(original.AreaId);
        await Assert.That(roundTripped.Address).IsEqualTo(original.Address);
        await Assert.That(roundTripped.Postcode).IsEqualTo(original.Postcode);
        await Assert.That(roundTripped.Description).IsEqualTo(original.Description);
        await Assert.That(roundTripped.AppType).IsEqualTo(original.AppType);
        await Assert.That(roundTripped.AppState).IsEqualTo(original.AppState);
        await Assert.That(roundTripped.AppSize).IsEqualTo(original.AppSize);
        await Assert.That(roundTripped.Longitude).IsEqualTo(original.Longitude);
        await Assert.That(roundTripped.Latitude).IsEqualTo(original.Latitude);
        await Assert.That(roundTripped.Url).IsEqualTo(original.Url);
        await Assert.That(roundTripped.Link).IsEqualTo(original.Link);
        await Assert.That(roundTripped.LastDifferent).IsEqualTo(original.LastDifferent);
    }
}
