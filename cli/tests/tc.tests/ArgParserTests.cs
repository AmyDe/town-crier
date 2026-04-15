namespace Tc.Tests;

public sealed class ArgParserTests
{
    [Test]
    public async Task Should_ParseCommandName_When_FirstArg()
    {
        var result = ArgParser.Parse(["grant-subscription", "--email", "a@b.com"]);

        await Assert.That(result.Command).IsEqualTo("grant-subscription");
    }

    [Test]
    public async Task Should_ParseKeyValuePairs_When_DashDashArgs()
    {
        var result = ArgParser.Parse(["grant-subscription", "--email", "a@b.com", "--tier", "Pro"]);

        await Assert.That(result.GetRequired("email")).IsEqualTo("a@b.com");
        await Assert.That(result.GetRequired("tier")).IsEqualTo("Pro");
    }

    [Test]
    public async Task Should_ReturnHelp_When_NoArgs()
    {
        var result = ArgParser.Parse([]);

        await Assert.That(result.Command).IsEqualTo("help");
    }

    [Test]
    public async Task Should_ReturnHelp_When_DashH()
    {
        var result = ArgParser.Parse(["-h"]);

        await Assert.That(result.Command).IsEqualTo("help");
    }

    [Test]
    public async Task Should_ReturnHelp_When_DashDashHelp()
    {
        var result = ArgParser.Parse(["--help"]);

        await Assert.That(result.Command).IsEqualTo("help");
    }

    [Test]
    public async Task Should_ExtractGlobalArgs_When_Mixed()
    {
        var result = ArgParser.Parse(["grant-subscription", "--url", "http://localhost:8080", "--email", "a@b.com"]);

        await Assert.That(result.GetOptional("url")).IsEqualTo("http://localhost:8080");
        await Assert.That(result.GetRequired("email")).IsEqualTo("a@b.com");
    }

    [Test]
    public async Task Should_ThrowArgumentException_When_RequiredArgMissing()
    {
        var result = ArgParser.Parse(["grant-subscription"]);

        Assert.Throws<ArgumentException>(() => result.GetRequired("email"));
    }

    [Test]
    public async Task Should_ReturnNull_When_OptionalArgMissing()
    {
        var result = ArgParser.Parse(["grant-subscription"]);

        await Assert.That(result.GetOptional("url")).IsNull();
    }
}
