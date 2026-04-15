# tc Admin CLI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Native AOT CLI tool at `/cli` for interacting with Town Crier admin API endpoints, starting with the `grant-subscription` command.

**Architecture:** Standalone .NET 10 console app with no DI container or host builder. Manual arg parsing dispatches to static command methods that call the API via a thin HttpClient wrapper. Config loaded from `~/.config/tc/config.json` with CLI arg overrides.

**Tech Stack:** .NET 10, Native AOT, System.Text.Json source generators, TUnit 0.12

**Spec:** `docs/superpowers/specs/2026-04-15-tc-admin-cli-design.md`

---

### Task 1: Project scaffolding

**Files:**
- Create: `cli/tc.sln`
- Create: `cli/src/tc/tc.csproj`
- Create: `cli/tests/tc.tests/tc.tests.csproj`
- Create: `cli/.editorconfig`
- Create: `cli/Directory.Build.props`

- [ ] **Step 1: Create the solution and source project**

```bash
mkdir -p cli/src/tc cli/tests/tc.tests
cd cli
dotnet new sln --name tc
dotnet new console --name tc --output src/tc --framework net10.0
dotnet sln add src/tc/tc.csproj
```

- [ ] **Step 2: Configure the source csproj for Native AOT**

Replace the contents of `cli/src/tc/tc.csproj` with:

```xml
<Project Sdk="Microsoft.NET.Sdk">

  <PropertyGroup>
    <OutputType>Exe</OutputType>
    <TargetFramework>net10.0</TargetFramework>
    <RootNamespace>Tc</RootNamespace>
    <ImplicitUsings>enable</ImplicitUsings>
    <Nullable>enable</Nullable>
    <PublishAot>true</PublishAot>
  </PropertyGroup>

</Project>
```

- [ ] **Step 3: Create the test project**

```bash
cd cli
dotnet new classlib --name tc.tests --output tests/tc.tests --framework net10.0
dotnet sln add tests/tc.tests/tc.tests.csproj
```

Replace the contents of `cli/tests/tc.tests/tc.tests.csproj` with:

```xml
<Project Sdk="Microsoft.NET.Sdk">

  <PropertyGroup>
    <TargetFramework>net10.0</TargetFramework>
    <RootNamespace>Tc.Tests</RootNamespace>
    <ImplicitUsings>enable</ImplicitUsings>
    <Nullable>enable</Nullable>
    <OutputType>Exe</OutputType>
  </PropertyGroup>

  <ItemGroup>
    <PackageReference Include="TUnit" Version="0.12.*" />
  </ItemGroup>

  <ItemGroup>
    <ProjectReference Include="..\..\src\tc\tc.csproj" />
  </ItemGroup>

</Project>
```

Delete the auto-generated `Class1.cs` from both projects if present.

- [ ] **Step 4: Create Directory.Build.props**

Create `cli/Directory.Build.props`:

```xml
<Project>
  <PropertyGroup>
    <AnalysisLevel>latest</AnalysisLevel>
    <AnalysisMode>All</AnalysisMode>
    <TreatWarningsAsErrors>true</TreatWarningsAsErrors>
    <CodeAnalysisTreatWarningsAsErrors>true</CodeAnalysisTreatWarningsAsErrors>
    <EnforceCodeStyleInBuild>true</EnforceCodeStyleInBuild>
  </PropertyGroup>
  <ItemGroup>
    <PackageReference Include="SonarAnalyzer.CSharp" Version="9.32.0.97167">
      <PrivateAssets>all</PrivateAssets>
      <IncludeAssets>runtime; build; native; contentfiles; analyzers; buildtransitive</IncludeAssets>
    </PackageReference>
    <PackageReference Include="StyleCop.Analyzers" Version="1.2.0-beta.556">
      <PrivateAssets>all</PrivateAssets>
      <IncludeAssets>runtime; build; native; contentfiles; analyzers; buildtransitive</IncludeAssets>
    </PackageReference>
  </ItemGroup>
</Project>
```

- [ ] **Step 5: Create .editorconfig**

Create `cli/.editorconfig`:

```ini
root = true

[*]
indent_style = space

[*.cs]
indent_size = 4

# StyleCop — disable file header requirement and XML doc analysis
dotnet_diagnostic.SA1633.severity = none
dotnet_diagnostic.SA0001.severity = none
dotnet_diagnostic.SA1600.severity = none
dotnet_diagnostic.SA1601.severity = none
dotnet_diagnostic.SA1602.severity = none
dotnet_diagnostic.SA1200.severity = none

# IL trim/AOT warnings are false positives in dotnet format (source generators handle them at build time)
dotnet_diagnostic.IL2026.severity = none
dotnet_diagnostic.IL3050.severity = none

# Test projects
[tests/**/*.cs]
dotnet_diagnostic.CA1707.severity = none
dotnet_diagnostic.CA1515.severity = none
dotnet_diagnostic.CA2007.severity = none
dotnet_diagnostic.CA1812.severity = none
```

- [ ] **Step 6: Write a placeholder Program.cs and verify the solution builds**

Replace `cli/src/tc/Program.cs` with:

```csharp
return 0;
```

Run:
```bash
cd cli && dotnet build
```

Expected: Build succeeded with 0 errors.

- [ ] **Step 7: Verify tests run**

Run:
```bash
cd cli && dotnet test
```

Expected: 0 tests discovered (no test classes yet), exit code 0.

- [ ] **Step 8: Commit**

```bash
git add cli/
git commit -m "feat(cli): scaffold tc solution with Native AOT and TUnit"
```

---

### Task 2: Arg parsing

**Files:**
- Create: `cli/src/tc/ArgParser.cs`
- Create: `cli/tests/tc.tests/ArgParserTests.cs`

- [ ] **Step 1: Write the failing tests**

Create `cli/tests/tc.tests/ArgParserTests.cs`:

```csharp
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run:
```bash
cd cli && dotnet test
```

Expected: Build failure — `ArgParser` type does not exist.

- [ ] **Step 3: Implement ArgParser**

Create `cli/src/tc/ArgParser.cs`:

```csharp
namespace Tc;

internal static class ArgParser
{
    private static readonly HashSet<string> HelpAliases = new(StringComparer.OrdinalIgnoreCase)
    {
        "help", "-h", "--help",
    };

    public static ParsedArgs Parse(string[] args)
    {
        if (args.Length == 0 || HelpAliases.Contains(args[0]))
        {
            return new ParsedArgs("help", new Dictionary<string, string>(StringComparer.OrdinalIgnoreCase));
        }

        var command = args[0];
        var options = new Dictionary<string, string>(StringComparer.OrdinalIgnoreCase);

        for (var i = 1; i < args.Length - 1; i += 2)
        {
            var key = args[i];
            var value = args[i + 1];

            if (key.StartsWith("--", StringComparison.Ordinal))
            {
                options[key[2..]] = value;
            }
        }

        return new ParsedArgs(command, options);
    }
}

internal sealed class ParsedArgs
{
    private readonly Dictionary<string, string> options;

    public ParsedArgs(string command, Dictionary<string, string> options)
    {
        this.Command = command;
        this.options = options;
    }

    public string Command { get; }

    public string GetRequired(string name)
    {
        if (!this.options.TryGetValue(name, out var value))
        {
            throw new ArgumentException($"Missing required argument: --{name}");
        }

        return value;
    }

    public string? GetOptional(string name)
    {
        return this.options.TryGetValue(name, out var value) ? value : null;
    }
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run:
```bash
cd cli && dotnet test
```

Expected: All 8 tests pass.

- [ ] **Step 5: Commit**

```bash
git add cli/src/tc/ArgParser.cs cli/tests/tc.tests/ArgParserTests.cs
git commit -m "feat(cli): add arg parser with key-value pair extraction"
```

---

### Task 3: Config loading

**Files:**
- Create: `cli/src/tc/Config.cs`
- Create: `cli/src/tc/Json/TcJsonContext.cs`
- Create: `cli/tests/tc.tests/ConfigTests.cs`

- [ ] **Step 1: Write the failing tests**

Create `cli/tests/tc.tests/ConfigTests.cs`:

```csharp
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run:
```bash
cd cli && dotnet test
```

Expected: Build failure — `TcConfig` type does not exist.

- [ ] **Step 3: Create the JSON serializer context**

Create `cli/src/tc/Json/TcJsonContext.cs`:

```csharp
using System.Text.Json.Serialization;

namespace Tc.Json;

[JsonSerializable(typeof(ConfigFile))]
[JsonSerializable(typeof(GrantSubscriptionRequest))]
internal sealed partial class TcJsonContext : JsonSerializerContext;

internal sealed class ConfigFile
{
    public string? Url { get; set; }

    public string? ApiKey { get; set; }
}

internal sealed class GrantSubscriptionRequest
{
    public required string Email { get; init; }

    public required string SubscriptionTier { get; init; }
}
```

- [ ] **Step 4: Implement TcConfig**

Create `cli/src/tc/Config.cs`:

```csharp
using System.Text.Json;
using Tc.Json;

namespace Tc;

internal sealed class TcConfig
{
    public required string Url { get; init; }

    public required string ApiKey { get; init; }

    public static string DefaultPath => Path.Combine(
        Environment.GetFolderPath(Environment.SpecialFolder.UserProfile),
        ".config",
        "tc",
        "config.json");

    public static TcConfig Load(string path, string? url, string? apiKey)
    {
        var fileUrl = (string?)null;
        var fileApiKey = (string?)null;

        if (File.Exists(path))
        {
            var json = File.ReadAllText(path);
            var file = JsonSerializer.Deserialize(json, TcJsonContext.Default.ConfigFile);
            if (file is not null)
            {
                fileUrl = file.Url;
                fileApiKey = file.ApiKey;
            }
        }

        var resolvedUrl = url ?? fileUrl;
        var resolvedApiKey = apiKey ?? fileApiKey;

        if (string.IsNullOrEmpty(resolvedUrl))
        {
            throw new InvalidOperationException(
                $"API URL not configured. Set 'url' in {path} or pass --url.");
        }

        if (string.IsNullOrEmpty(resolvedApiKey))
        {
            throw new InvalidOperationException(
                $"API key not configured. Set 'apiKey' in {path} or pass --api-key.");
        }

        return new TcConfig
        {
            Url = resolvedUrl,
            ApiKey = resolvedApiKey,
        };
    }
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run:
```bash
cd cli && dotnet test
```

Expected: All 14 tests pass (8 arg parser + 6 config).

- [ ] **Step 6: Commit**

```bash
git add cli/src/tc/Config.cs cli/src/tc/Json/TcJsonContext.cs cli/tests/tc.tests/ConfigTests.cs
git commit -m "feat(cli): add config loading with file and CLI arg merge"
```

---

### Task 4: API client

**Files:**
- Create: `cli/src/tc/ApiClient.cs`

No unit tests for this class — it is a thin HttpClient wrapper with no logic to test independently. It will be exercised via the grant-subscription command integration.

- [ ] **Step 1: Implement ApiClient**

Create `cli/src/tc/ApiClient.cs`:

```csharp
using System.Net.Http.Json;
using System.Text.Json.Serialization.Metadata;

namespace Tc;

internal sealed class ApiClient : IDisposable
{
    private const string ApiKeyHeader = "X-Admin-Key";

    private readonly HttpClient client;

    public ApiClient(TcConfig config)
    {
        this.client = new HttpClient
        {
            BaseAddress = new Uri(config.Url.TrimEnd('/')),
        };
        this.client.DefaultRequestHeaders.Add(ApiKeyHeader, config.ApiKey);
    }

    public async Task<HttpResponseMessage> PutAsJsonAsync<T>(
        string path,
        T body,
        JsonTypeInfo<T> typeInfo,
        CancellationToken ct)
    {
        return await this.client.PutAsJsonAsync(path, body, typeInfo, ct).ConfigureAwait(false);
    }

    public void Dispose()
    {
        this.client.Dispose();
    }
}
```

- [ ] **Step 2: Verify the solution builds**

Run:
```bash
cd cli && dotnet build
```

Expected: Build succeeded with 0 errors.

- [ ] **Step 3: Commit**

```bash
git add cli/src/tc/ApiClient.cs
git commit -m "feat(cli): add API client with admin key header"
```

---

### Task 5: Grant subscription command

**Files:**
- Create: `cli/src/tc/Commands/GrantSubscriptionCommand.cs`

- [ ] **Step 1: Implement the command**

Create `cli/src/tc/Commands/GrantSubscriptionCommand.cs`:

```csharp
using System.Text.Json;
using Tc.Json;

namespace Tc.Commands;

internal static class GrantSubscriptionCommand
{
    private static readonly HashSet<string> ValidTiers = new(StringComparer.OrdinalIgnoreCase)
    {
        "Free", "Personal", "Pro",
    };

    public static async Task<int> RunAsync(ApiClient client, ParsedArgs args, CancellationToken ct)
    {
        string email;
        try
        {
            email = args.GetRequired("email");
        }
        catch (ArgumentException)
        {
            Console.Error.WriteLine("Missing required argument: --email");
            Console.Error.WriteLine("Usage: tc grant-subscription --email <email> --tier <Free|Personal|Pro>");
            return 1;
        }

        string tier;
        try
        {
            tier = args.GetRequired("tier");
        }
        catch (ArgumentException)
        {
            Console.Error.WriteLine("Missing required argument: --tier");
            Console.Error.WriteLine("Usage: tc grant-subscription --email <email> --tier <Free|Personal|Pro>");
            return 1;
        }

        if (!ValidTiers.Contains(tier))
        {
            Console.Error.WriteLine($"Invalid tier: {tier}. Must be one of: Free, Personal, Pro");
            return 1;
        }

        // Normalise tier casing to match API expectation
        tier = ValidTiers.First(t => string.Equals(t, tier, StringComparison.OrdinalIgnoreCase));

        var request = new GrantSubscriptionRequest
        {
            Email = email,
            SubscriptionTier = tier,
        };

        var response = await client.PutAsJsonAsync(
            "/v1/admin/subscriptions",
            request,
            TcJsonContext.Default.GrantSubscriptionRequest,
            ct).ConfigureAwait(false);

        if (response.StatusCode == System.Net.HttpStatusCode.NotFound)
        {
            Console.Error.WriteLine($"User not found: {email}");
            return 2;
        }

        if (!response.IsSuccessStatusCode)
        {
            var body = await response.Content.ReadAsStringAsync(ct).ConfigureAwait(false);
            Console.Error.WriteLine($"API error ({(int)response.StatusCode}): {body}");
            return 2;
        }

        Console.WriteLine($"Subscription granted: {email} -> {tier}");
        return 0;
    }
}
```

- [ ] **Step 2: Verify the solution builds**

Run:
```bash
cd cli && dotnet build
```

Expected: Build succeeded with 0 errors.

- [ ] **Step 3: Commit**

```bash
git add cli/src/tc/Commands/GrantSubscriptionCommand.cs
git commit -m "feat(cli): add grant-subscription command"
```

---

### Task 6: Wire up Program.cs with help, version, and dispatch

**Files:**
- Modify: `cli/src/tc/Program.cs`

- [ ] **Step 1: Implement the main entry point**

Replace `cli/src/tc/Program.cs` with:

```csharp
using Tc;
using Tc.Commands;

var parsed = ArgParser.Parse(args);

if (parsed.Command is "version")
{
    Console.WriteLine("tc 0.1.0");
    return 0;
}

if (parsed.Command is "help")
{
    PrintHelp();
    return 0;
}

TcConfig config;
try
{
    config = TcConfig.Load(
        TcConfig.DefaultPath,
        url: parsed.GetOptional("url"),
        apiKey: parsed.GetOptional("api-key"));
}
catch (InvalidOperationException ex)
{
    Console.Error.WriteLine(ex.Message);
    return 1;
}

using var client = new ApiClient(config);
using var cts = new CancellationTokenSource();
Console.CancelKeyPress += (_, e) =>
{
    e.Cancel = true;
    cts.Cancel();
};

return parsed.Command switch
{
    "grant-subscription" => await GrantSubscriptionCommand.RunAsync(client, parsed, cts.Token).ConfigureAwait(false),
    _ => UnknownCommand(parsed.Command),
};

static int UnknownCommand(string command)
{
    Console.Error.WriteLine($"Unknown command: {command}");
    Console.Error.WriteLine("Run 'tc help' for a list of commands.");
    return 1;
}

static void PrintHelp()
{
    Console.WriteLine("""
        tc — Town Crier admin CLI

        Usage: tc <command> [options]

        Commands:
          grant-subscription   Grant or change a user's subscription tier
          help                 Show this help message
          version              Print version

        Global options:
          --url <url>          API base URL (overrides config file)
          --api-key <key>      Admin API key (overrides config file)

        Config file: ~/.config/tc/config.json
        """);
}
```

- [ ] **Step 2: Verify the solution builds**

Run:
```bash
cd cli && dotnet build
```

Expected: Build succeeded with 0 errors.

- [ ] **Step 3: Run all tests**

Run:
```bash
cd cli && dotnet test
```

Expected: All 14 tests pass.

- [ ] **Step 4: Smoke test the CLI**

Run:
```bash
cd cli && dotnet run --project src/tc -- help
```

Expected: Help text printed to stdout.

Run:
```bash
cd cli && dotnet run --project src/tc -- version
```

Expected: `tc 0.1.0`

Run:
```bash
cd cli && dotnet run --project src/tc -- grant-subscription
```

Expected: Error about missing config (or missing --email), exit code 1.

- [ ] **Step 5: Commit**

```bash
git add cli/src/tc/Program.cs
git commit -m "feat(cli): wire up command dispatch with help and version"
```

---

### Task 7: Verify Native AOT publish

**Files:** None modified — this is a verification step.

- [ ] **Step 1: Publish with Native AOT**

Run:
```bash
cd cli && dotnet publish src/tc -c Release -o publish/
```

Expected: Publish succeeded. A native binary `tc` (or `tc.exe` on Windows) exists in `cli/publish/`.

- [ ] **Step 2: Verify the binary runs**

Run:
```bash
cd cli && ./publish/tc help
```

Expected: Help text printed, same as `dotnet run`.

Run:
```bash
cd cli && ./publish/tc version
```

Expected: `tc 0.1.0`

- [ ] **Step 3: Commit (no code changes — just verify)**

No commit needed. This task is verification only.

---

### Task 8: Add cli to .gitignore and final cleanup

**Files:**
- Create: `cli/.gitignore`

- [ ] **Step 1: Create .gitignore for the cli directory**

Create `cli/.gitignore`:

```
bin/
obj/
publish/
```

- [ ] **Step 2: Run full build and test suite one final time**

```bash
cd cli && dotnet build && dotnet test
```

Expected: Build succeeded, all 14 tests pass.

- [ ] **Step 3: Commit**

```bash
git add cli/.gitignore
git commit -m "chore(cli): add .gitignore for build outputs"
```
