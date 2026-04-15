using Tc;
using Tc.Commands;

var parsed = ArgParser.Parse(args);

if (parsed.Command is "version")
{
    await Console.Out.WriteLineAsync("tc 0.1.0").ConfigureAwait(false);
    return 0;
}

if (parsed.Command is "help")
{
    await PrintHelpAsync().ConfigureAwait(false);
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
    await Console.Error.WriteLineAsync(ex.Message).ConfigureAwait(false);
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
    _ => await UnknownCommandAsync(parsed.Command).ConfigureAwait(false),
};

static async Task<int> UnknownCommandAsync(string command)
{
    await Console.Error.WriteLineAsync($"Unknown command: {command}").ConfigureAwait(false);
    await Console.Error.WriteLineAsync("Run 'tc help' for a list of commands.").ConfigureAwait(false);
    return 1;
}

static async Task PrintHelpAsync()
{
    await Console.Out.WriteLineAsync("""
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
        """).ConfigureAwait(false);
}
