package tc

import (
	"context"
	"errors"
	"fmt"
)

// version is printed by `tc version`.
const version = "tc 0.1.0"

const helpText = `tc — Town Crier admin CLI

Usage: tc <command> [options]

Commands:
  generate-offer-codes        Bulk-generate offer codes (single- or multi-use)
  list-offer-codes            List offer codes and their redemption progress
  grant-subscription          Grant or change a user's subscription tier
  list-users                  List users with email, ID, and subscription tier
  stats                       Show aggregate user-base statistics
  polling-status              Show whether dev and prod poll PlanIt
  polling-switch              Move PlanIt polling to dev, to prod, or off
  help                        Show this help message
  version                     Print version

generate-offer-codes options:
  --count <n>            Number of codes to generate (1-1000, required)
  --tier <tier>          Subscription tier (Personal|Pro, required)
  --duration-days <d>    Duration in days (1-365, required)
  --label <label>        Admin-facing label for the batch (required)
  --max-redemptions <m>  Redemption cap per code (1-10000, default: 1)

list-offer-codes options:
  --label <term>       Filter by label substring (case-insensitive)

list-users options:
  --search <term>      Filter by email substring (case-insensitive)
  --page-size <n>      Results per page (default: 20)

polling-switch options:
  --to <dev|prod|off>    Environment that polls afterwards (required)
  --reason <text>        Reason stored with the switch
  --allow-missing true   Continue when the other environment has no switch

polling-status and polling-switch read "environments" (dev and prod, each
with url and apiKey) from the config file and ignore --url and --api-key.

Global options:
  --url <url>          API base URL (overrides config file)
  --api-key <key>      Admin API key (overrides config file)

Config file: ~/.config/tc/config.json`

// Run parses args, resolves config, and dispatches to a command. It returns the
// process exit code and never calls os.Exit, so it stays testable. ctx carries
// cancellation (wired to SIGINT/SIGTERM in main).
func Run(ctx context.Context, env Env, rawArgs []string) int {
	args := ParseArgs(rawArgs)

	switch args.Command {
	case "version":
		fmt.Fprintln(env.Out, version)
		return exitOK
	case "help":
		fmt.Fprintln(env.Out, helpText)
		return exitOK
	case "polling-status", "polling-switch":
		cfgs, err := LoadEnvironments(DefaultConfigPath())
		if err != nil {
			fmt.Fprintln(env.Err, err.Error())
			return exitUsage
		}
		if args.Command == "polling-status" {
			return runPollingStatus(ctx, cfgs, env)
		}
		return runPollingSwitch(ctx, cfgs, env, args, realPollingClock)
	}

	cfg, err := LoadConfig(DefaultConfigPath(), optionalPtr(args.GetOptional("url")), optionalPtr(args.GetOptional("api-key")))
	if err != nil {
		path := DefaultConfigPath()
		switch {
		case errors.Is(err, ErrURLNotConfigured):
			fmt.Fprintf(env.Err, "API URL not configured. Set 'url' in %s or pass --url.\n", path)
		case errors.Is(err, ErrAPIKeyNotConfigured):
			fmt.Fprintf(env.Err, "API key not configured. Set 'apiKey' in %s or pass --api-key.\n", path)
		default:
			fmt.Fprintln(env.Err, err.Error())
		}
		return exitUsage
	}

	client := NewClient(cfg)

	switch args.Command {
	case "generate-offer-codes":
		return runGenerateOfferCodes(ctx, client, env, args)
	case "list-offer-codes":
		return runListOfferCodes(ctx, client, env, args)
	case "grant-subscription":
		return runGrantSubscription(ctx, client, env, args)
	case "list-users":
		return runListUsers(ctx, client, env, args)
	case "stats":
		return runStats(ctx, client, env, args)
	default:
		fmt.Fprintf(env.Err, "Unknown command: %s\n", args.Command)
		fmt.Fprintln(env.Err, "Run 'tc help' for a list of commands.")
		return exitUsage
	}
}

// optionalPtr lifts a GetOptional result into the *string LoadConfig expects:
// nil when absent so the file value wins, a pointer (even to "") when present.
func optionalPtr(value string, present bool) *string {
	if !present {
		return nil
	}
	return &value
}
