namespace TownCrier.Domain.Tests.PlanningApplications;

public sealed class PlanningApplicationTests
{
    [Test]
    public async Task HasSameBusinessFieldsAs_Should_ReturnTrue_When_AllBusinessFieldsMatch()
    {
        var left = new PlanningApplicationBuilder().Build();
        var right = new PlanningApplicationBuilder().Build();

        await Assert.That(left.HasSameBusinessFieldsAs(right)).IsTrue();
    }

    [Test]
    public async Task HasSameBusinessFieldsAs_Should_ReturnTrue_When_OnlyLastDifferentChanges()
    {
        var left = new PlanningApplicationBuilder()
            .WithLastDifferent(new DateTimeOffset(2026, 3, 1, 0, 0, 0, TimeSpan.Zero))
            .Build();
        var right = new PlanningApplicationBuilder()
            .WithLastDifferent(new DateTimeOffset(2026, 4, 10, 0, 0, 0, TimeSpan.Zero))
            .Build();

        await Assert.That(left.HasSameBusinessFieldsAs(right)).IsTrue();
    }

    [Test]
    public async Task HasSameBusinessFieldsAs_Should_ReturnFalse_When_AddressChanges()
    {
        var left = new PlanningApplicationBuilder().WithAddress("123 Test Street").Build();
        var right = new PlanningApplicationBuilder().WithAddress("456 Other Street").Build();

        await Assert.That(left.HasSameBusinessFieldsAs(right)).IsFalse();
    }

    [Test]
    public async Task HasSameBusinessFieldsAs_Should_ReturnFalse_When_DescriptionChanges()
    {
        var left = new PlanningApplicationBuilder().WithDescription("Original").Build();
        var right = new PlanningApplicationBuilder().WithDescription("Amended").Build();

        await Assert.That(left.HasSameBusinessFieldsAs(right)).IsFalse();
    }

    [Test]
    public async Task HasSameBusinessFieldsAs_Should_ReturnFalse_When_AppStateChanges()
    {
        var left = new PlanningApplicationBuilder().WithAppState("Undecided").Build();
        var right = new PlanningApplicationBuilder().WithAppState("Decided").Build();

        await Assert.That(left.HasSameBusinessFieldsAs(right)).IsFalse();
    }

    [Test]
    public async Task HasSameBusinessFieldsAs_Should_ReturnFalse_When_AppTypeChanges()
    {
        var left = new PlanningApplicationBuilder().WithAppType("Full").Build();
        var right = new PlanningApplicationBuilder().WithAppType("Outline").Build();

        await Assert.That(left.HasSameBusinessFieldsAs(right)).IsFalse();
    }

    [Test]
    public async Task HasSameBusinessFieldsAs_Should_ReturnFalse_When_DecidedDateChanges()
    {
        var left = new PlanningApplicationBuilder().WithDecidedDate(null).Build();
        var right = new PlanningApplicationBuilder().WithDecidedDate(new DateOnly(2026, 4, 1)).Build();

        await Assert.That(left.HasSameBusinessFieldsAs(right)).IsFalse();
    }

    [Test]
    public async Task HasSameBusinessFieldsAs_Should_ReturnFalse_When_PostcodeChanges()
    {
        var left = new PlanningApplicationBuilder().WithPostcode("SW1A 1AA").Build();
        var right = new PlanningApplicationBuilder().WithPostcode("SW1A 2AA").Build();

        await Assert.That(left.HasSameBusinessFieldsAs(right)).IsFalse();
    }

    [Test]
    public async Task HasSameBusinessFieldsAs_Should_ReturnFalse_When_LatitudeChanges()
    {
        var left = new PlanningApplicationBuilder().WithCoordinates(51.5074, -0.1278).Build();
        var right = new PlanningApplicationBuilder().WithCoordinates(52.0, -0.1278).Build();

        await Assert.That(left.HasSameBusinessFieldsAs(right)).IsFalse();
    }

    [Test]
    public async Task HasSameBusinessFieldsAs_Should_ReturnFalse_When_LongitudeChanges()
    {
        var left = new PlanningApplicationBuilder().WithCoordinates(51.5074, -0.1278).Build();
        var right = new PlanningApplicationBuilder().WithCoordinates(51.5074, 0.0).Build();

        await Assert.That(left.HasSameBusinessFieldsAs(right)).IsFalse();
    }

    [Test]
    public async Task HasSameBusinessFieldsAs_Should_ReturnFalse_When_CoordinatesGoFromNullToPopulated()
    {
        var left = new PlanningApplicationBuilder().WithCoordinates(null, null).Build();
        var right = new PlanningApplicationBuilder().WithCoordinates(51.5074, -0.1278).Build();

        await Assert.That(left.HasSameBusinessFieldsAs(right)).IsFalse();
    }

    [Test]
    public async Task HasSameBusinessFieldsAs_Should_ReturnFalse_When_StartDateChanges()
    {
        var left = new PlanningApplicationBuilder().WithStartDate(new DateOnly(2026, 1, 1)).Build();
        var right = new PlanningApplicationBuilder().WithStartDate(new DateOnly(2026, 2, 1)).Build();

        await Assert.That(left.HasSameBusinessFieldsAs(right)).IsFalse();
    }

    [Test]
    public async Task HasSameBusinessFieldsAs_Should_ReturnFalse_When_ConsultedDateChanges()
    {
        var left = new PlanningApplicationBuilder().WithConsultedDate(new DateOnly(2026, 2, 1)).Build();
        var right = new PlanningApplicationBuilder().WithConsultedDate(new DateOnly(2026, 3, 1)).Build();

        await Assert.That(left.HasSameBusinessFieldsAs(right)).IsFalse();
    }

    [Test]
    public async Task HasSameBusinessFieldsAs_Should_ReturnFalse_When_AppSizeChanges()
    {
        var left = new PlanningApplicationBuilder().WithAppSize("Small").Build();
        var right = new PlanningApplicationBuilder().WithAppSize("Large").Build();

        await Assert.That(left.HasSameBusinessFieldsAs(right)).IsFalse();
    }

    [Test]
    public async Task HasSameBusinessFieldsAs_Should_ReturnFalse_When_UrlChanges()
    {
        var left = new PlanningApplicationBuilder().WithUrl("url-a").Build();
        var right = new PlanningApplicationBuilder().WithUrl("url-b").Build();

        await Assert.That(left.HasSameBusinessFieldsAs(right)).IsFalse();
    }

    [Test]
    public async Task HasSameBusinessFieldsAs_Should_Throw_When_OtherIsNull()
    {
        var app = new PlanningApplicationBuilder().Build();

        await Assert.ThrowsAsync<ArgumentNullException>(
            () => Task.FromResult(app.HasSameBusinessFieldsAs(null!)));
    }
}
