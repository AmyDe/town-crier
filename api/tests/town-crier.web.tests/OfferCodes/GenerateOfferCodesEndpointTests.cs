using System.Net;
using System.Net.Http.Json;
using System.Text.RegularExpressions;
using Microsoft.AspNetCore.Hosting;
using Microsoft.AspNetCore.TestHost;
using Microsoft.Extensions.DependencyInjection;
using TownCrier.Application.OfferCodes;
using TownCrier.Domain.UserProfiles;
using TownCrier.Infrastructure.OfferCodes;
using TownCrier.Web.Endpoints;

namespace TownCrier.Web.Tests.OfferCodes;

public sealed class GenerateOfferCodesEndpointTests
{
    private const string AdminKey = "test-admin-key-12345";
    private const string AdminKeyHeaderName = "X-Admin-Key";

    [Test]
    public async Task Should_Return401_When_AdminKeyMissing()
    {
        await using var baseFactory = new TestWebApplicationFactory();
        await using var factory = baseFactory.WithWebHostBuilder(ConfigureOfferCodeHost);
        using var client = factory.CreateClient();

        var response = await client.PostAsJsonAsync(
            new Uri("/v1/admin/offer-codes", UriKind.Relative),
            new GenerateOfferCodesRequest(1, SubscriptionTier.Pro, 30),
            AppJsonSerializerContext.Default.GenerateOfferCodesRequest).ConfigureAwait(false);

        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.Unauthorized);
    }

    [Test]
    public async Task Should_Return200_WithPlainTextCodes_When_Valid()
    {
        await using var baseFactory = new TestWebApplicationFactory();
        await using var factory = baseFactory.WithWebHostBuilder(ConfigureOfferCodeHost);
        using var client = factory.CreateClient();
        client.DefaultRequestHeaders.Add(AdminKeyHeaderName, AdminKey);

        var response = await client.PostAsJsonAsync(
            new Uri("/v1/admin/offer-codes", UriKind.Relative),
            new GenerateOfferCodesRequest(3, SubscriptionTier.Pro, 30),
            AppJsonSerializerContext.Default.GenerateOfferCodesRequest).ConfigureAwait(false);

        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.OK);
        await Assert.That(response.Content.Headers.ContentType!.MediaType).IsEqualTo("text/plain");

        var body = await response.Content.ReadAsStringAsync().ConfigureAwait(false);
        var lines = body.Split('\n', StringSplitOptions.RemoveEmptyEntries);
        await Assert.That(lines.Length).IsEqualTo(3);

        foreach (var line in lines)
        {
            await Assert.That(Regex.IsMatch(line, "^[0-9A-Z]{4}-[0-9A-Z]{4}-[0-9A-Z]{4}$")).IsTrue();
        }
    }

    [Test]
    public async Task Should_Return400_When_CountOutOfRange()
    {
        await using var baseFactory = new TestWebApplicationFactory();
        await using var factory = baseFactory.WithWebHostBuilder(ConfigureOfferCodeHost);
        using var client = factory.CreateClient();
        client.DefaultRequestHeaders.Add(AdminKeyHeaderName, AdminKey);

        var response = await client.PostAsJsonAsync(
            new Uri("/v1/admin/offer-codes", UriKind.Relative),
            new GenerateOfferCodesRequest(0, SubscriptionTier.Pro, 30),
            AppJsonSerializerContext.Default.GenerateOfferCodesRequest).ConfigureAwait(false);

        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.BadRequest);
    }

    private static void ConfigureOfferCodeHost(IWebHostBuilder builder)
    {
        builder.UseSetting("Admin:ApiKey", AdminKey);

        builder.ConfigureTestServices(services =>
        {
            services.AddSingleton<IOfferCodeRepository, InMemoryOfferCodeRepository>();
        });
    }
}
