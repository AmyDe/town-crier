using Tc.Commands;

namespace Tc.Tests;

public sealed class GenerateOfferCodesCommandTests
{
    [Test]
    public async Task Should_ReturnExitCode1_When_CountMissing()
    {
        using var client = CreateDummyClient();
        var args = ArgParser.Parse(["generate-offer-codes", "--tier", "Pro", "--duration-days", "30"]);

        var exitCode = await GenerateOfferCodesCommand.RunAsync(client, args, CancellationToken.None);

        await Assert.That(exitCode).IsEqualTo(1);
    }

    [Test]
    public async Task Should_ReturnExitCode1_When_TierMissing()
    {
        using var client = CreateDummyClient();
        var args = ArgParser.Parse(["generate-offer-codes", "--count", "10", "--duration-days", "30"]);

        var exitCode = await GenerateOfferCodesCommand.RunAsync(client, args, CancellationToken.None);

        await Assert.That(exitCode).IsEqualTo(1);
    }

    [Test]
    public async Task Should_ReturnExitCode1_When_DurationDaysMissing()
    {
        using var client = CreateDummyClient();
        var args = ArgParser.Parse(["generate-offer-codes", "--count", "10", "--tier", "Pro"]);

        var exitCode = await GenerateOfferCodesCommand.RunAsync(client, args, CancellationToken.None);

        await Assert.That(exitCode).IsEqualTo(1);
    }

    [Test]
    public async Task Should_ReturnExitCode1_When_CountNotInteger()
    {
        using var client = CreateDummyClient();
        var args = ArgParser.Parse(["generate-offer-codes", "--count", "abc", "--tier", "Pro", "--duration-days", "30"]);

        var exitCode = await GenerateOfferCodesCommand.RunAsync(client, args, CancellationToken.None);

        await Assert.That(exitCode).IsEqualTo(1);
    }

    [Test]
    public async Task Should_ReturnExitCode1_When_CountBelowRange()
    {
        using var client = CreateDummyClient();
        var args = ArgParser.Parse(["generate-offer-codes", "--count", "0", "--tier", "Pro", "--duration-days", "30"]);

        var exitCode = await GenerateOfferCodesCommand.RunAsync(client, args, CancellationToken.None);

        await Assert.That(exitCode).IsEqualTo(1);
    }

    [Test]
    public async Task Should_ReturnExitCode1_When_CountAboveRange()
    {
        using var client = CreateDummyClient();
        var args = ArgParser.Parse(["generate-offer-codes", "--count", "1001", "--tier", "Pro", "--duration-days", "30"]);

        var exitCode = await GenerateOfferCodesCommand.RunAsync(client, args, CancellationToken.None);

        await Assert.That(exitCode).IsEqualTo(1);
    }

    [Test]
    public async Task Should_ReturnExitCode1_When_TierInvalid()
    {
        using var client = CreateDummyClient();
        var args = ArgParser.Parse(["generate-offer-codes", "--count", "10", "--tier", "Free", "--duration-days", "30"]);

        var exitCode = await GenerateOfferCodesCommand.RunAsync(client, args, CancellationToken.None);

        await Assert.That(exitCode).IsEqualTo(1);
    }

    [Test]
    public async Task Should_ReturnExitCode1_When_TierUnknown()
    {
        using var client = CreateDummyClient();
        var args = ArgParser.Parse(["generate-offer-codes", "--count", "10", "--tier", "Enterprise", "--duration-days", "30"]);

        var exitCode = await GenerateOfferCodesCommand.RunAsync(client, args, CancellationToken.None);

        await Assert.That(exitCode).IsEqualTo(1);
    }

    [Test]
    public async Task Should_ReturnExitCode1_When_DurationDaysNotInteger()
    {
        using var client = CreateDummyClient();
        var args = ArgParser.Parse(["generate-offer-codes", "--count", "10", "--tier", "Pro", "--duration-days", "abc"]);

        var exitCode = await GenerateOfferCodesCommand.RunAsync(client, args, CancellationToken.None);

        await Assert.That(exitCode).IsEqualTo(1);
    }

    [Test]
    public async Task Should_ReturnExitCode1_When_DurationDaysBelowRange()
    {
        using var client = CreateDummyClient();
        var args = ArgParser.Parse(["generate-offer-codes", "--count", "10", "--tier", "Pro", "--duration-days", "0"]);

        var exitCode = await GenerateOfferCodesCommand.RunAsync(client, args, CancellationToken.None);

        await Assert.That(exitCode).IsEqualTo(1);
    }

    [Test]
    public async Task Should_ReturnExitCode1_When_DurationDaysAboveRange()
    {
        using var client = CreateDummyClient();
        var args = ArgParser.Parse(["generate-offer-codes", "--count", "10", "--tier", "Pro", "--duration-days", "366"]);

        var exitCode = await GenerateOfferCodesCommand.RunAsync(client, args, CancellationToken.None);

        await Assert.That(exitCode).IsEqualTo(1);
    }

    private static ApiClient CreateDummyClient()
    {
        return new ApiClient(new TcConfig
        {
            Url = "http://127.0.0.1:1",
            ApiKey = "sk-test",
        });
    }
}
