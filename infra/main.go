// Town Crier Infrastructure (Pulumi program).
//
// Logical resource names, the project name ("town-crier"), and the azure-native
// provider version (3.16.0) are pinned so the URNs match the existing
// shared/dev/prod state exactly. Do not rename resources — add aliases instead.
package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		conf := config.New(ctx, "town-crier")
		env := conf.Require("environment")

		tags := pulumi.StringMap{
			"project":     pulumi.String("town-crier"),
			"managedBy":   pulumi.String("pulumi"),
			"environment": pulumi.String(env),
		}

		if env == "shared" {
			return runSharedStack(ctx, conf, tags)
		}
		return runEnvironmentStack(ctx, conf, env, tags)
	})
}
