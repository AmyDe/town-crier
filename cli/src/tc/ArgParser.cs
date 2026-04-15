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
