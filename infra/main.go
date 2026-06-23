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

		// TC-60DX FAILURE-PATH TEST — DO NOT MERGE. Compiles & vets clean, but
		// panics at preview time on a missing required config key, reproducing the
		// kind of crash (#619 / tc-g9p6) that the old piped step silently swallowed.
		conf.Require("tc60dxDeliberateCrash")

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
