using Pulumi;

return await Pulumi.Deployment.RunAsync(() =>
{
    var config = new Config("town-crier");
    var env = config.Require("environment");

    var tags = new InputMap<string>
    {
        { "project", "town-crier" },
        { "managedBy", "pulumi" },
        { "environment", env },
    };

    if (env == "shared")
    {
        return SharedStack.Run(config, tags);
    }

    return EnvironmentStack.Run(config, env, tags);
});
