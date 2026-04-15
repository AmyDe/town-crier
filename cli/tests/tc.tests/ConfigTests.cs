namespace Tc.Tests;

public sealed class ConfigTests
{
    [Test]
    public async Task Should_LoadFromFile_When_FileExists()
    {
        var dir = Path.Combine(Path.GetTempPath(), Guid.NewGuid().ToString());
        Directory.CreateDirectory(dir);
        var path = Path.Combine(dir, "config.json");
        await File.WriteAllTextAsync(path, """{"url":"https://api.example.com","apiKey":"sk-test"}""");

        try
        {
            var config = TcConfig.Load(path, url: null, apiKey: null);

            await Assert.That(config.Url).IsEqualTo("https://api.example.com");
            await Assert.That(config.ApiKey).IsEqualTo("sk-test");
        }
        finally
        {
            Directory.Delete(dir, true);
        }
    }

    [Test]
    public async Task Should_UseCliArgs_When_OverridingFile()
    {
        var dir = Path.Combine(Path.GetTempPath(), Guid.NewGuid().ToString());
        Directory.CreateDirectory(dir);
        var path = Path.Combine(dir, "config.json");
        await File.WriteAllTextAsync(path, """{"url":"https://api.example.com","apiKey":"sk-file"}""");

        try
        {
            var config = TcConfig.Load(path, url: "http://localhost:8080", apiKey: "sk-override");

            await Assert.That(config.Url).IsEqualTo("http://localhost:8080");
            await Assert.That(config.ApiKey).IsEqualTo("sk-override");
        }
        finally
        {
            Directory.Delete(dir, true);
        }
    }

    [Test]
    public async Task Should_UseCliArgsOnly_When_NoFile()
    {
        var path = Path.Combine(Path.GetTempPath(), Guid.NewGuid().ToString(), "config.json");

        var config = TcConfig.Load(path, url: "http://localhost:8080", apiKey: "sk-arg");

        await Assert.That(config.Url).IsEqualTo("http://localhost:8080");
        await Assert.That(config.ApiKey).IsEqualTo("sk-arg");
    }

    [Test]
    public void Should_ThrowInvalidOperation_When_UrlMissing()
    {
        var path = Path.Combine(Path.GetTempPath(), Guid.NewGuid().ToString(), "config.json");

        Assert.Throws<InvalidOperationException>(() => TcConfig.Load(path, url: null, apiKey: "sk-test"));
    }

    [Test]
    public void Should_ThrowInvalidOperation_When_ApiKeyMissing()
    {
        var path = Path.Combine(Path.GetTempPath(), Guid.NewGuid().ToString(), "config.json");

        Assert.Throws<InvalidOperationException>(() => TcConfig.Load(path, url: "http://localhost:8080", apiKey: null));
    }

    [Test]
    public async Task Should_PartialOverride_When_OnlyUrlProvided()
    {
        var dir = Path.Combine(Path.GetTempPath(), Guid.NewGuid().ToString());
        Directory.CreateDirectory(dir);
        var path = Path.Combine(dir, "config.json");
        await File.WriteAllTextAsync(path, """{"url":"https://api.example.com","apiKey":"sk-file"}""");

        try
        {
            var config = TcConfig.Load(path, url: "http://localhost:8080", apiKey: null);

            await Assert.That(config.Url).IsEqualTo("http://localhost:8080");
            await Assert.That(config.ApiKey).IsEqualTo("sk-file");
        }
        finally
        {
            Directory.Delete(dir, true);
        }
    }
}
