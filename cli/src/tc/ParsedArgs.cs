namespace Tc;

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
