using TownCrier.Application.Geocoding;
using TownCrier.Domain.Geocoding;

namespace TownCrier.Application.Tests.Geocoding;

public sealed class GeocodePostcodeQueryHandlerTests
{
    [Test]
    public async Task Should_ReturnCoordinates_When_PostcodeIsValid()
    {
        // Arrange
        var geocoder = new FakePostcodeGeocoder();
        var postcode = Postcode.Create("SW1A 1AA");
        geocoder.Add(postcode, new Coordinates(51.501009, -0.141588));
        var handler = new GeocodePostcodeQueryHandler(geocoder);
        var query = new GeocodePostcodeQuery("SW1A 1AA");

        // Act
        var result = await handler.HandleAsync(query, CancellationToken.None);

        // Assert
        await Assert.That(result.Coordinates.Latitude).IsEqualTo(51.501009);
        await Assert.That(result.Coordinates.Longitude).IsEqualTo(-0.141588);
    }

    [Test]
    public async Task Should_ThrowInvalidOperation_When_PostcodeNotFound()
    {
        // Arrange
        var geocoder = new FakePostcodeGeocoder();
        var handler = new GeocodePostcodeQueryHandler(geocoder);
        var query = new GeocodePostcodeQuery("SW1A 1AA");

        // Act & Assert
        var exception = await Assert.ThrowsAsync<InvalidOperationException>(
            () => handler.HandleAsync(query, CancellationToken.None));
        await Assert.That(exception.Message).Contains("could not be geocoded");
    }

    [Test]
    public async Task Should_ThrowArgumentException_When_PostcodeIsInvalid()
    {
        // Arrange
        var geocoder = new FakePostcodeGeocoder();
        var handler = new GeocodePostcodeQueryHandler(geocoder);
        var query = new GeocodePostcodeQuery("NOT-A-POSTCODE");

        // Act & Assert
        var exception = await Assert.ThrowsAsync<ArgumentException>(
            () => handler.HandleAsync(query, CancellationToken.None));
        await Assert.That(exception.Message).Contains("not a valid UK postcode");
    }
}
