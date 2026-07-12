package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/pulumi/pulumi-azure-native-sdk/alertsmanagement/v3"
	"github.com/pulumi/pulumi-azure-native-sdk/app/v3"
	appinsights "github.com/pulumi/pulumi-azure-native-sdk/applicationinsights/v3"
	"github.com/pulumi/pulumi-azure-native-sdk/authorization/v3"
	"github.com/pulumi/pulumi-azure-native-sdk/communication/v3"
	"github.com/pulumi/pulumi-azure-native-sdk/consumption/v3"
	"github.com/pulumi/pulumi-azure-native-sdk/containerregistry/v3"
	"github.com/pulumi/pulumi-azure-native-sdk/dbforpostgresql/v3"
	"github.com/pulumi/pulumi-azure-native-sdk/managedidentity/v3"
	"github.com/pulumi/pulumi-azure-native-sdk/monitor/v3"
	"github.com/pulumi/pulumi-azure-native-sdk/operationalinsights/v3"
	"github.com/pulumi/pulumi-azure-native-sdk/portal/v3"
	"github.com/pulumi/pulumi-azure-native-sdk/resources/v3"
	"github.com/pulumi/pulumi-random/sdk/v4/go/random"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

// dashboardTimespan24HoursMs is the relative timespan (ms) for the dashboard metric tiles.
const dashboardTimespan24HoursMs = 86400000

// subscriptionFromID extracts the subscription ID (segment 2) from an ARM resource ID.
func subscriptionFromID(id pulumi.IDOutput) pulumi.StringOutput {
	return id.ApplyT(func(s pulumi.ID) string {
		return strings.Split(string(s), "/")[2]
	}).(pulumi.StringOutput)
}

func runSharedStack(ctx *pulumi.Context, conf *config.Config, tags pulumi.StringMap) error {
	ciServicePrincipalID := conf.Require("ciServicePrincipalId")

	// Resource Group
	resourceGroup, err := resources.NewResourceGroup(ctx, "rg-town-crier-shared", &resources.ResourceGroupArgs{
		ResourceGroupName: pulumi.String("rg-town-crier-shared"),
		Tags:              tags,
	})
	if err != nil {
		return err
	}

	// Azure Container Registry (shared across environments).
	//
	// RetentionPolicy is Premium-SKU only — Azure rejects any Policies block on Basic
	// ("Policies are only supported for managed registries in Premium SKU"), so
	// untagged-manifest cleanup must happen out-of-band. See tc-dq46.
	containerRegistry, err := containerregistry.NewRegistry(ctx, "acrtowncriershared", &containerregistry.RegistryArgs{
		RegistryName:      pulumi.String("acrtowncriershared"),
		ResourceGroupName: resourceGroup.Name,
		Sku: &containerregistry.SkuArgs{
			Name: containerregistry.SkuNameBasic,
		},
		AdminUserEnabled: pulumi.Bool(false),
		Tags:             tags,
	})
	if err != nil {
		return err
	}

	// User-assigned managed identity for AcrPull
	acrPullIdentity, err := managedidentity.NewUserAssignedIdentity(ctx, "id-town-crier-acr-pull", &managedidentity.UserAssignedIdentityArgs{
		ResourceName:      pulumi.String("id-town-crier-acr-pull"),
		ResourceGroupName: resourceGroup.Name,
		Tags:              tags,
	})
	if err != nil {
		return err
	}

	// Extract subscription ID from the ACR's resource ID
	// ACR ID format: /subscriptions/{subId}/resourceGroups/{rg}/providers/Microsoft.ContainerRegistry/registries/{name}
	subscriptionID := subscriptionFromID(containerRegistry.ID())

	// AcrPull role assignment — managed identity can pull images from the ACR
	_, err = authorization.NewRoleAssignment(ctx, "acr-pull-role", &authorization.RoleAssignmentArgs{
		Scope: containerRegistry.ID(),
		RoleDefinitionId: pulumi.Sprintf(
			"/subscriptions/%s/providers/Microsoft.Authorization/roleDefinitions/7f951dda-4ed3-4680-a7ca-43fe172d538d", subscriptionID),
		PrincipalId:   acrPullIdentity.PrincipalId,
		PrincipalType: pulumi.String(string(authorization.PrincipalTypeServicePrincipal)),
	})
	if err != nil {
		return err
	}

	// AcrPush role assignment — CI service principal can push images to the ACR
	_, err = authorization.NewRoleAssignment(ctx, "acr-push-role", &authorization.RoleAssignmentArgs{
		Scope: containerRegistry.ID(),
		RoleDefinitionId: pulumi.Sprintf(
			"/subscriptions/%s/providers/Microsoft.Authorization/roleDefinitions/8311e382-0749-4cb8-b61a-304f252e45ec", subscriptionID),
		PrincipalId:   pulumi.String(ciServicePrincipalID),
		PrincipalType: pulumi.String(string(authorization.PrincipalTypeServicePrincipal)),
	})
	if err != nil {
		return err
	}

	// Log Analytics Workspace (shared across environments)
	logAnalytics, err := operationalinsights.NewWorkspace(ctx, "log-town-crier-shared", &operationalinsights.WorkspaceArgs{
		WorkspaceName:     pulumi.String("log-town-crier-shared"),
		ResourceGroupName: resourceGroup.Name,
		Sku: &operationalinsights.WorkspaceSkuArgs{
			Name: operationalinsights.WorkspaceSkuNameEnumPerGB2018,
		},
		RetentionInDays: pulumi.Int(30), // PerGB2018 SKU minimum is 30 days
		WorkspaceCapping: &operationalinsights.WorkspaceCappingArgs{
			DailyQuotaGb: pulumi.Float64(1.0),
		},
		Tags: tags,
	})
	if err != nil {
		return err
	}

	// Application Insights (shared, backed by Log Analytics)
	appInsights, err := appinsights.NewComponent(ctx, "appi-town-crier-shared", &appinsights.ComponentArgs{
		ResourceName:        pulumi.String("appi-town-crier-shared"),
		ResourceGroupName:   resourceGroup.Name,
		WorkspaceResourceId: logAnalytics.ID(),
		ApplicationType:     pulumi.String("web"),
		Kind:                pulumi.String("web"),
		IngestionMode:       appinsights.IngestionModeLogAnalytics,
		RetentionInDays:     pulumi.Int(30), // Minimum allowed by Azure (30, 60, 90, 120, etc.)
		Tags:                tags,
	})
	if err != nil {
		return err
	}

	// Container Apps Environment (shared across environments). Logs go to
	// azure-monitor via the managed OpenTelemetry agent.
	containerAppsEnv, err := app.NewManagedEnvironment(ctx, "cae-town-crier-shared", &app.ManagedEnvironmentArgs{
		EnvironmentName:   pulumi.String("cae-town-crier-shared"),
		ResourceGroupName: resourceGroup.Name,
		AppLogsConfiguration: &app.AppLogsConfigurationArgs{
			Destination: pulumi.String("azure-monitor"),
		},
		AppInsightsConfiguration: &app.AppInsightsConfigurationArgs{
			ConnectionString: appInsights.ConnectionString,
		},
		OpenTelemetryConfiguration: &app.OpenTelemetryConfigurationArgs{
			TracesConfiguration: &app.TracesConfigurationArgs{
				Destinations: pulumi.StringArray{pulumi.String("appInsights")},
			},
			LogsConfiguration: &app.LogsConfigurationArgs{
				Destinations: pulumi.StringArray{pulumi.String("appInsights")},
			},
		},
		Tags: tags,
	})
	if err != nil {
		return err
	}

	// Diagnostic Setting routes Container Apps Environment logs to the shared Log Analytics
	// workspace via the platform's DCR pipeline. The "allLogs" category group includes
	// ContainerAppConsoleLogs, ContainerAppSystemLogs, and any future categories Azure adds.
	_, err = monitor.NewDiagnosticSetting(ctx, "diag-cae-town-crier-shared", &monitor.DiagnosticSettingArgs{
		Name:        pulumi.String("diag-cae-town-crier-shared"),
		ResourceUri: containerAppsEnv.ID(),
		WorkspaceId: logAnalytics.ID(),
		Logs: monitor.LogSettingsArray{
			&monitor.LogSettingsArgs{
				CategoryGroup: pulumi.String("allLogs"),
				Enabled:       pulumi.Bool(true),
			},
		},
	})
	if err != nil {
		return err
	}

	// ARM_SUBSCRIPTION_ID is set by CI (and local `pulumi up`) via the Azure login step; it
	// feeds the subscription-wide budget scope below.
	armSubscriptionID := os.Getenv("ARM_SUBSCRIPTION_ID")
	if armSubscriptionID == "" {
		return fmt.Errorf("ARM_SUBSCRIPTION_ID must be set for the subscription budget scope")
	}

	// ARM_TENANT_ID is set by CI and local Pulumi runs via the Azure login step.
	// A config key (azureTenantId) takes precedence so the value can be pinned per stack
	// without relying on the environment — mirrors how armSubscriptionID is read above.
	tenantID := conf.Get("azureTenantId")
	if tenantID == "" {
		tenantID = os.Getenv("ARM_TENANT_ID")
	}

	// Set the native ContainerAppConsoleLogs table to the Basic plan.
	_, err = operationalinsights.NewTable(ctx, "table-containerappconsolelogs-basic", &operationalinsights.TableArgs{
		ResourceGroupName: resourceGroup.Name,
		WorkspaceName:     logAnalytics.Name,
		TableName:         pulumi.String("ContainerAppConsoleLogs"),
		Plan:              operationalinsights.TablePlanEnumBasic,
	})
	if err != nil {
		return err
	}

	// Move the AppTraces table to the Basic Logs plan. See tc-9ggc.
	_, err = operationalinsights.NewTable(ctx, "table-apptraces-basic", &operationalinsights.TableArgs{
		ResourceGroupName: resourceGroup.Name,
		WorkspaceName:     logAnalytics.Name,
		TableName:         pulumi.String("AppTraces"),
		Plan:              operationalinsights.TablePlanEnumBasic,
	})
	if err != nil {
		return err
	}

	// Cap the workspace-based App* tables at the workspace's 30-day retention. See tc-23yb.
	// AppTraces is omitted (already on Basic = 8 days).
	appTablesToCapAt30Days := []string{
		"AppRequests",
		"AppDependencies",
		"AppExceptions",
		"AppMetrics",
		"AppPageViews",
		"AppAvailabilityResults",
		"AppBrowserTimings",
		"AppEvents",
		"AppPerformanceCounters",
		"AppSystemEvents",
	}
	for _, tableName := range appTablesToCapAt30Days {
		_, err = operationalinsights.NewTable(ctx, fmt.Sprintf("table-%s-30day", strings.ToLower(tableName)), &operationalinsights.TableArgs{
			ResourceGroupName:    resourceGroup.Name,
			WorkspaceName:        logAnalytics.Name,
			TableName:            pulumi.String(tableName),
			RetentionInDays:      pulumi.Int(30),
			TotalRetentionInDays: pulumi.Int(30),
		})
		if err != nil {
			return err
		}
	}

	// User-assigned managed identity for Cosmos DB data access
	cosmosDataIdentity, err := managedidentity.NewUserAssignedIdentity(ctx, "id-town-crier-cosmos-data", &managedidentity.UserAssignedIdentityArgs{
		ResourceName:      pulumi.String("id-town-crier-cosmos-data"),
		ResourceGroupName: resourceGroup.Name,
		Tags:              tags,
	})
	if err != nil {
		return err
	}

	// User-assigned managed identity for the dev-only dev-seed job's read-only
	// access to town_crier_prod. Deliberately separate from cosmosDataIdentity
	// (which every other Container App/Job in both dev and prod shares, and
	// which already holds full DML on both databases): a bug in this job's SQL
	// must not be able to write to prod. Mapped to a SELECT-only Postgres role
	// via `pgbootstrap -readonly` — see
	// docs/adr/0038-dev-seed-least-privilege-prod-read.md. Consumed by the
	// dev-seed Container Apps Job created in infra/environment.go (tc-grvu.6);
	// not wired into any resource in this file.
	devSeedReaderIdentity, err := managedidentity.NewUserAssignedIdentity(ctx, "id-town-crier-dev-seed-reader", &managedidentity.UserAssignedIdentityArgs{
		ResourceName:      pulumi.String("id-town-crier-dev-seed-reader"),
		ResourceGroupName: resourceGroup.Name,
		Tags:              tags,
	})
	if err != nil {
		return err
	}

	// Azure Database for PostgreSQL — Flexible Server (shared across environments; this single
	// server hosts both the dev and prod databases long-term). First infra step of the Cosmos →
	// Postgres + PostGIS migration (memo 0010, epic tc-hpd2 / GH #645). Pre-revenue profile:
	// Burstable B1ms, single-AZ (HA disabled), public network access (no VNet/delegated subnet),
	// 7-day non-geo-redundant backups. The dev database, prod database, and the app wiring swap
	// are handled in later phases — this block provisions only the server itself.
	//
	// Admin password is generated and held as a Pulumi secret. OverrideSpecial restricts the
	// special characters to a URL-safe set so the value drops into a postgres:// connection
	// string without percent-encoding; the Min* constraints guarantee Azure's password
	// complexity rule (at least three of upper/lower/numeric/special).
	postgresAdminPassword, err := random.NewRandomPassword(ctx, "psql-town-crier-shared-admin", &random.RandomPasswordArgs{
		Length:          pulumi.Int(32),
		Special:         pulumi.Bool(true),
		OverrideSpecial: pulumi.String("-_.~"),
		MinUpper:        pulumi.Int(1),
		MinLower:        pulumi.Int(1),
		MinNumeric:      pulumi.Int(1),
		MinSpecial:      pulumi.Int(1),
	})
	if err != nil {
		return err
	}

	postgresServer, err := dbforpostgresql.NewServer(ctx, "psql-town-crier-shared", &dbforpostgresql.ServerArgs{
		ServerName:                 pulumi.String("psql-town-crier-shared"),
		ResourceGroupName:          resourceGroup.Name,
		Location:                   resourceGroup.Location,
		Version:                    pulumi.String("16"),
		CreateMode:                 dbforpostgresql.CreateModeDefault,
		AdministratorLogin:         pulumi.String("tcadmin"),
		AdministratorLoginPassword: postgresAdminPassword.Result,
		Sku: &dbforpostgresql.SkuArgs{
			Name: pulumi.String("Standard_B1ms"),
			Tier: dbforpostgresql.SkuTierBurstable,
		},
		Storage: &dbforpostgresql.StorageArgs{
			StorageSizeGB: pulumi.Int(32),
			Type:          dbforpostgresql.StorageType_Premium_LRS,
		},
		HighAvailability: &dbforpostgresql.HighAvailabilityArgs{
			Mode: dbforpostgresql.PostgreSqlFlexibleServerHighAvailabilityModeDisabled,
		},
		Backup: &dbforpostgresql.BackupTypeArgs{
			BackupRetentionDays: pulumi.Int(7),
			GeoRedundantBackup:  dbforpostgresql.GeoRedundantBackupDisabled,
		},
		Network: &dbforpostgresql.NetworkArgs{
			PublicNetworkAccess: dbforpostgresql.PublicNetworkAccessEnumEnabled,
		},
		// Enable Entra (AAD) authentication alongside password auth. PasswordAuth stays
		// Enabled to preserve tcadmin break-glass access through the migration; password-only
		// mode will be disabled in a later hardening step. See GH #653.
		AuthConfig: &dbforpostgresql.AuthConfigArgs{
			ActiveDirectoryAuth: pulumi.String("Enabled"),
			PasswordAuth:        pulumi.String("Enabled"),
			TenantId:            pulumi.String(tenantID),
		},
		Tags: tags,
	})
	if err != nil {
		return err
	}

	// "Allow Azure services" firewall rule — the all-zeros (0.0.0.0/0.0.0.0) special rule lets
	// Azure-hosted resources (the dev Container App) reach the server over its public endpoint.
	postgresFirewall, err := dbforpostgresql.NewFirewallRule(ctx, "psql-town-crier-shared-allow-azure", &dbforpostgresql.FirewallRuleArgs{
		FirewallRuleName:  pulumi.String("allow-azure-services"),
		ResourceGroupName: resourceGroup.Name,
		ServerName:        postgresServer.Name,
		StartIpAddress:    pulumi.String("0.0.0.0"),
		EndIpAddress:      pulumi.String("0.0.0.0"),
	})
	if err != nil {
		return err
	}

	// Allowlist the PostGIS and pg_trgm extensions via the azure.extensions server parameter.
	// Without it, `CREATE EXTENSION postgis` / `CREATE EXTENSION pg_trgm` are rejected. pg_trgm
	// (trigram matching) is required by the application-search-indexes migration for fuzzy/
	// partial text search. Source must be "user-override" or Azure ignores the assigned value.
	// A server parameter change is a server-level operation that cannot run while the server is
	// busy with another operation, so this is serialised after the firewall rule (DependsOn) —
	// applying both concurrently right after server creation trips "ServerIsBusy".
	//
	// Note: built-in PgBouncer (the `pgbouncer.enabled` parameter) is deliberately not set — it
	// is unsupported on the Burstable tier (B1ms) and Azure rejects it with
	// ServerConfigurationNotAllowed. The Go data layer uses pgx, which pools connections
	// client-side; revisit built-in PgBouncer only if this server is moved to General Purpose.
	_, err = dbforpostgresql.NewConfiguration(ctx, "psql-town-crier-shared-azure-extensions", &dbforpostgresql.ConfigurationArgs{
		ConfigurationName: pulumi.String("azure.extensions"),
		ResourceGroupName: resourceGroup.Name,
		ServerName:        postgresServer.Name,
		Source:            pulumi.String("user-override"),
		Value:             pulumi.String("POSTGIS,pg_trgm"),
	}, pulumi.DependsOn([]pulumi.Resource{postgresFirewall}))
	if err != nil {
		return err
	}

	// Entra administrator on the shared Postgres server. The repo owner's account is used
	// so `cmd/pgbootstrap` (Slice B of GH #653) can run locally under `az login` credentials
	// to create the towncrier_api role and grant least-privilege DML rights. The CI service
	// principal can be added as a second Administrator later if CI bootstrap is needed.
	aadAdmin, err := dbforpostgresql.NewAdministrator(ctx, "psql-town-crier-shared-aad-admin", &dbforpostgresql.AdministratorArgs{
		ResourceGroupName: resourceGroup.Name,
		ServerName:        postgresServer.Name,
		ObjectId:          pulumi.String(conf.Require("postgresAadAdminObjectId")),
		PrincipalName:     pulumi.String(conf.Require("postgresAadAdminPrincipalName")),
		PrincipalType:     pulumi.String("User"),
		TenantId:          pulumi.String(tenantID),
	}, pulumi.DependsOn([]pulumi.Resource{postgresServer}))
	if err != nil {
		return err
	}

	// Second Entra administrator: the CI service principal (town-crier-github-actions,
	// objectId in ciServicePrincipalID). This is the identity half of ADR 0036 — it lets the
	// gated CD migrate job run `pgmigrate` (goose DDL) against the shared server as itself,
	// authenticating passwordlessly over OIDC. For a Flexible-Server service-principal admin the
	// Postgres login username equals this PrincipalName, and the CD migrate job connects with
	// exactly "town-crier-github-actions", so this literal must stay in lockstep with the
	// workflow's `-admin-user` value.
	//
	// Two Entra-admin writes to the same server can collide with "ServerIsBusy" (the same
	// class of failure the azure.extensions Configuration guards against by serialising after
	// the firewall). DependsOn both the server AND the human admin above so the two admin
	// writes serialise rather than race.
	_, err = dbforpostgresql.NewAdministrator(ctx, "psql-town-crier-shared-ci-admin", &dbforpostgresql.AdministratorArgs{
		ResourceGroupName: resourceGroup.Name,
		ServerName:        postgresServer.Name,
		ObjectId:          pulumi.String(ciServicePrincipalID),
		PrincipalName:     pulumi.String("town-crier-github-actions"),
		PrincipalType:     pulumi.String("ServicePrincipal"),
		TenantId:          pulumi.String(tenantID),
	}, pulumi.DependsOn([]pulumi.Resource{postgresServer, aadAdmin}))
	if err != nil {
		return err
	}

	// Subscription-wide cost guard rail — fires at £50 cumulative monthly spend across the
	// whole subscription. See tc-yica and docs/cost-forecast/2026-05-02.md.
	alertEmail := conf.Require("alertEmail")
	_, err = consumption.NewBudget(ctx, "budget-subscription-monthly", &consumption.BudgetArgs{
		BudgetName: pulumi.String("budget-subscription-monthly"),
		Scope:      pulumi.String(fmt.Sprintf("/subscriptions/%s", armSubscriptionID)),
		Amount:     pulumi.Float64(50),
		Category:   consumption.CategoryTypeCost,
		TimeGrain:  consumption.TimeGrainTypeMonthly,
		TimePeriod: &consumption.BudgetTimePeriodArgs{
			StartDate: pulumi.String("2026-05-01T00:00:00Z"),
		},
		Notifications: consumption.NotificationMap{
			"actual_GreaterThan_50_Percent": &consumption.NotificationArgs{
				Enabled:       pulumi.Bool(true),
				Operator:      consumption.OperatorTypeGreaterThan,
				Threshold:     pulumi.Float64(50),
				ThresholdType: pulumi.String(string(consumption.ThresholdTypeActual)),
				ContactEmails: pulumi.StringArray{pulumi.String(alertEmail)},
				Locale:        consumption.CultureCode_En_Gb,
			},
			"actual_GreaterThan_100_Percent": &consumption.NotificationArgs{
				Enabled:       pulumi.Bool(true),
				Operator:      consumption.OperatorTypeGreaterThan,
				Threshold:     pulumi.Float64(100),
				ThresholdType: pulumi.String(string(consumption.ThresholdTypeActual)),
				ContactEmails: pulumi.StringArray{pulumi.String(alertEmail)},
				Locale:        consumption.CultureCode_En_Gb,
			},
			// Forecast notification (GH #943 Phase 5) — warns ahead of the actual-spend
			// notifications above by firing on Azure's cost *forecast*, not just realised spend.
			"forecasted_GreaterThan_100_Percent": &consumption.NotificationArgs{
				Enabled:       pulumi.Bool(true),
				Operator:      consumption.OperatorTypeGreaterThan,
				Threshold:     pulumi.Float64(100),
				ThresholdType: pulumi.String("Forecasted"),
				ContactEmails: pulumi.StringArray{pulumi.String(alertEmail)},
				Locale:        consumption.CultureCode_En_Gb,
			},
		},
	})
	if err != nil {
		return err
	}

	// Action Group (tc-ttjor / GH #938 PR3) — single email receiver, reused by both alerts
	// below and by the poll-queue-depth metric alert in the prod env stack (infra/environment.go).
	// Deliberately created HERE rather than in the prod env stack (as the bead literally asked):
	// the PlanIt failure-rate log alert below must live in this file (it queries the shared Log
	// Analytics workspace), and wiring it to an action group owned by the prod stack would need a
	// StackReference running shared -> prod, inverting the foundational shared -> env dependency
	// direction every other cross-stack read in this codebase uses. The env stack already reads
	// shared outputs the normal way round, so the prod metric alert consumes actionGroupId
	// (exported below) instead.
	actionGroup, err := monitor.NewActionGroup(ctx, "ag-town-crier-shared", &monitor.ActionGroupArgs{
		ActionGroupName:   pulumi.String("ag-town-crier-shared"),
		ResourceGroupName: resourceGroup.Name,
		Location:          pulumi.String("Global"),
		GroupShortName:    pulumi.String("tcalerts"),
		Enabled:           pulumi.Bool(true),
		EmailReceivers: monitor.EmailReceiverArray{
			monitor.EmailReceiverArgs{
				Name:                 pulumi.String("owner"),
				EmailAddress:         pulumi.String(alertEmail),
				UseCommonAlertSchema: pulumi.Bool(true),
			},
		},
		// Azure mobile app push receiver (GH #943 Phase 1) — the same Azure account
		// (amy@salter.uk) that already receives Service Health pushes via the auto-created
		// azureapp-auto group, so alerts fire on-device without needing a dedicated app install.
		AzureAppPushReceivers: monitor.AzureAppPushReceiverArray{
			monitor.AzureAppPushReceiverArgs{
				Name:         pulumi.String("owner-app"),
				EmailAddress: pulumi.String("amy@salter.uk"),
			},
		},
		Tags: tags,
	})
	if err != nil {
		return err
	}

	// Scheduled query (log) alert — PlanIt non-429 dependency failure ratio over the last hour,
	// computed from AppDependencies on this shared Log Analytics workspace (tc-ttjor / GH #938
	// PR3). Go emits no AppMetrics by design (no Go Azure Monitor metrics exporter), so a
	// log-based alert is the only option here. The query requires >=20 calls in the window so a
	// quiet hour with one stray failure can't read as 100%. ResultCode on these dependency spans
	// is the OTel span status ('0'/'2'), NOT the HTTP status — hence filtering on
	// Properties['http.response.status_code'] instead, per the issue.
	//
	// Location is pinned to uksouth (rather than left to the provider default the way the
	// workspace above is) because, unlike the Global action group, Log Analytics-based scheduled
	// query rules are regional and must be explicit — mirrors the same rationale documented on
	// the Service Bus namespace in environment.go (tc-ds1e).
	const planitFailureRatioQuery = `AppDependencies
| where Target has "planit"
| summarize Total = count(), NonRetryableFailures = countif(Success == false and tostring(Properties["http.response.status_code"]) != "429")
| where Total >= 20
| extend FailureRatioPercent = round(100.0 * NonRetryableFailures / Total, 1)
| where FailureRatioPercent > 30`

	_, err = monitor.NewScheduledQueryRule(ctx, "alert-planit-failure-rate-shared", &monitor.ScheduledQueryRuleArgs{
		RuleName:            pulumi.String("alert-planit-failure-rate-shared"),
		ResourceGroupName:   resourceGroup.Name,
		Location:            pulumi.String("uksouth"),
		Kind:                pulumi.String("LogAlert"),
		Description:         pulumi.String("PlanIt non-429 dependency failure ratio exceeded 30% over the last hour (>=20 calls). See GH #938."),
		DisplayName:         pulumi.String("PlanIt non-429 failure rate"),
		Severity:            pulumi.Float64(2), // Warning
		Enabled:             pulumi.Bool(true),
		EvaluationFrequency: pulumi.String("PT15M"),
		WindowSize:          pulumi.String("PT1H"),
		Scopes:              pulumi.StringArray{logAnalytics.ID()},
		Criteria: monitor.ScheduledQueryRuleCriteriaArgs{
			AllOf: monitor.ConditionArray{
				monitor.ConditionArgs{
					Query: pulumi.String(planitFailureRatioQuery),
					// The query already filters down to failing windows (FailureRatioPercent >
					// 30 with >=20 calls); "count of rows produced > 0" is the standard
					// scheduled-query-rule pattern for "fire when the filtered query returns
					// anything", so no MetricMeasureColumn is needed.
					TimeAggregation: pulumi.String("Count"),
					Operator:        pulumi.String("GreaterThan"),
					Threshold:       pulumi.Float64(0),
				},
			},
		},
		Actions: monitor.ActionsArgs{
			ActionGroups: pulumi.StringArray{actionGroup.ID()},
		},
		Tags: tags,
	})
	if err != nil {
		return err
	}

	// Azure Communication Services (Email) — UK data location.
	emailServiceUk, err := communication.NewEmailService(ctx, "email-town-crier-uk", &communication.EmailServiceArgs{
		EmailServiceName:  pulumi.String("email-town-crier-uk"),
		ResourceGroupName: resourceGroup.Name,
		Location:          pulumi.String("global"),
		DataLocation:      pulumi.String("UK"),
		Tags:              tags,
	})
	if err != nil {
		return err
	}

	// Custom-domain sender identity for the UK EmailService. Logical name retains the
	// `-new` suffix from the dual-resource migration (tc-8634, tc-zx5g).
	towncrierAppDomain, err := communication.NewDomain(ctx, "domain-towncrierapp-uk-new", &communication.DomainArgs{
		DomainName:        pulumi.String("towncrierapp.uk"),
		EmailServiceName:  emailServiceUk.Name,
		ResourceGroupName: resourceGroup.Name,
		Location:          pulumi.String("global"),
		DomainManagement:  pulumi.String(string(communication.DomainManagementCustomerManaged)),
		Tags:              tags,
	})
	if err != nil {
		return err
	}

	// LinkedDomains authorises the CommunicationService to send mail from the
	// towncrierapp.uk Domain. Without this, sends fail with 404 DomainNotLinked (tc-luxj).
	communicationServiceUk, err := communication.NewCommunicationService(ctx, "acs-town-crier-uk", &communication.CommunicationServiceArgs{
		CommunicationServiceName: pulumi.String("acs-town-crier-uk"),
		ResourceGroupName:        resourceGroup.Name,
		Location:                 pulumi.String("global"),
		DataLocation:             pulumi.String("UK"),
		LinkedDomains:            pulumi.StringArray{towncrierAppDomain.ID()},
		Tags:                     tags,
	})
	if err != nil {
		return err
	}

	// Authorises the local-part `hello` for sends from towncrierapp.uk. See tc-6tak.
	_, err = communication.NewSenderUsername(ctx, "sender-hello-towncrier-uk", &communication.SenderUsernameArgs{
		SenderUsername:    pulumi.String("hello"),
		DomainName:        towncrierAppDomain.Name,
		EmailServiceName:  emailServiceUk.Name,
		ResourceGroupName: resourceGroup.Name,
		Username:          pulumi.String("hello"),
		DisplayName:       pulumi.String("Town Crier"),
	})
	if err != nil {
		return err
	}

	// Alert posture (GH #943, tc-97k35.1). The 2026-07-12 audit found the action group and
	// PlanIt rule above already ported (tc-ttjor, GH #938) but the rest of the P1+P2 posture
	// unimplemented. This codifies the remainder: availability tests, Postgres/job/ACS metric
	// alerts, API/webhook/APNs/ACS/Auth0 log alerts, job-absence detection, Service Health,
	// Failure Anomalies. Everything below scopes prod resources via constructed ID strings
	// (armSubscriptionID + documented literal names) rather than cross-stack reads, per the
	// issue's design: alert rules are ARM meta-resources that can live in the shared RG while
	// pointing at resources in rg-town-crier-prod.
	//
	// alert-poll-queue-depth-prod is deliberately NOT duplicated here. It is already
	// Pulumi-managed by the prod stack (infra/environment.go, createPollQueueDepthAlert) and was
	// applied there via cd-prod (releases v0.19.3/v0.19.4) — it was never a CLI orphan, so the
	// issue's Phase 1 item 3 "move it into the shared stack" premise doesn't hold. The prod-stack
	// copy stays canonical.

	// Phase 2: availability tests. Two standard WebTests against the live prod endpoints, each
	// with a companion MetricAlert that fires when >=2 of the 3 EMEA probe locations report the
	// endpoint down.
	if err = createAvailabilityCheck(ctx, resourceGroup, appInsights, actionGroup, tags,
		"webtest-api-prod", "https://api.towncrierapp.uk/health"); err != nil {
		return err
	}
	if err = createAvailabilityCheck(ctx, resourceGroup, appInsights, actionGroup, tags,
		"webtest-web-prod", "https://towncrierapp.uk/"); err != nil {
		return err
	}

	// Phase 3: platform metric alerts.

	// Postgres — scoped to the shared Flexible Server provisioned above (B1ms burstable, 32GB,
	// auto-grow disabled, so storage/CPU-credit exhaustion are real operational risks here).
	type postgresAlertSpec struct {
		suffix     string // resource-name suffix, e.g. "storage" -> alert-pg-storage-shared
		metricName string
		operator   string
		threshold  float64
		windowSize string
		evalFreq   string
		severity   int
	}
	postgresAlerts := []postgresAlertSpec{
		{"storage", "storage_percent", "GreaterThan", 80, "PT30M", "PT15M", 2},
		{"cpu-credits", "cpu_credits_remaining", "LessThan", 30, "PT30M", "PT15M", 2},
		{"connections", "active_connections", "GreaterThan", 25, "PT30M", "PT15M", 3},
		{"alive", "is_db_alive", "LessThan", 1, "PT5M", "PT5M", 1},
	}
	for _, spec := range postgresAlerts {
		alertName := fmt.Sprintf("alert-pg-%s-shared", spec.suffix)
		_, err = monitor.NewMetricAlert(ctx, alertName, &monitor.MetricAlertArgs{
			RuleName:            pulumi.String(alertName),
			ResourceGroupName:   resourceGroup.Name,
			Location:            pulumi.String("global"),
			Description:         pulumi.String(fmt.Sprintf("Postgres %s %s %v on psql-town-crier-shared.", spec.metricName, strings.ToLower(spec.operator), spec.threshold)),
			Severity:            pulumi.Int(spec.severity),
			Enabled:             pulumi.Bool(true),
			AutoMitigate:        pulumi.Bool(true),
			EvaluationFrequency: pulumi.String(spec.evalFreq),
			WindowSize:          pulumi.String(spec.windowSize),
			Scopes:              pulumi.StringArray{postgresServer.ID()},
			Criteria: monitor.MetricAlertSingleResourceMultipleMetricCriteriaArgs{
				OdataType: pulumi.String("Microsoft.Azure.Monitor.SingleResourceMultipleMetricCriteria"),
				AllOf: monitor.MetricCriteriaArray{
					monitor.MetricCriteriaArgs{
						CriterionType:   pulumi.String("StaticThresholdCriterion"),
						Name:            pulumi.String(spec.metricName),
						MetricName:      pulumi.String(spec.metricName),
						MetricNamespace: pulumi.String("Microsoft.DBforPostgreSQL/flexibleServers"),
						Operator:        pulumi.String(spec.operator),
						Threshold:       pulumi.Float64(spec.threshold),
						TimeAggregation: pulumi.String("Average"),
					},
				},
			},
			Actions: monitor.MetricAlertActionArray{
				monitor.MetricAlertActionArgs{ActionGroupId: actionGroup.ID()},
			},
			Tags: tags,
		})
		if err != nil {
			return err
		}
	}

	// Container Apps jobs — one failed-execution alert per prod job, generated via a loop over
	// the job-name slice (rather than seven copy-pasted blocks). Scopes are constructed ARM IDs:
	// these jobs live in the prod stack (infra/environment.go, createWorkerJob), named
	// job-tc-<name>-prod.
	prodFailedExecutionJobs := []string{
		"poll",
		"poll-bootstrap",
		"digest",
		"digest-hourly",
		"dormant-cleanup",
		"subscription-sweep",
		"pg-purge",
	}
	for _, job := range prodFailedExecutionJobs {
		alertName := fmt.Sprintf("alert-job-failed-%s-prod", job)
		jobID := fmt.Sprintf(
			"/subscriptions/%s/resourceGroups/rg-town-crier-prod/providers/Microsoft.App/jobs/job-tc-%s-prod",
			armSubscriptionID, job)
		_, err = monitor.NewMetricAlert(ctx, alertName, &monitor.MetricAlertArgs{
			RuleName:            pulumi.String(alertName),
			ResourceGroupName:   resourceGroup.Name,
			Location:            pulumi.String("global"),
			Description:         pulumi.String(fmt.Sprintf("Container Apps job job-tc-%s-prod reported a Failed execution.", job)),
			Severity:            pulumi.Int(2),
			Enabled:             pulumi.Bool(true),
			AutoMitigate:        pulumi.Bool(true),
			EvaluationFrequency: pulumi.String("PT15M"),
			WindowSize:          pulumi.String("PT30M"),
			Scopes:              pulumi.StringArray{pulumi.String(jobID)},
			Criteria: monitor.MetricAlertSingleResourceMultipleMetricCriteriaArgs{
				OdataType: pulumi.String("Microsoft.Azure.Monitor.SingleResourceMultipleMetricCriteria"),
				AllOf: monitor.MetricCriteriaArray{
					monitor.MetricCriteriaArgs{
						CriterionType:   pulumi.String("StaticThresholdCriterion"),
						Name:            pulumi.String("FailedExecutions"),
						MetricName:      pulumi.String("Executions"),
						MetricNamespace: pulumi.String("Microsoft.App/jobs"),
						Dimensions: monitor.MetricDimensionArray{
							monitor.MetricDimensionArgs{
								Name:     pulumi.String("state"),
								Operator: pulumi.String("Include"),
								Values:   pulumi.StringArray{pulumi.String("Failed")},
							},
						},
						Operator:        pulumi.String("GreaterThan"),
						Threshold:       pulumi.Float64(0),
						TimeAggregation: pulumi.String("Total"),
					},
				},
			},
			Actions: monitor.MetricAlertActionArray{
				monitor.MetricAlertActionArgs{ActionGroupId: actionGroup.ID()},
			},
			Tags: tags,
		})
		if err != nil {
			return err
		}
	}

	// ACS email delivery — catches post-acceptance bounces/blocks the AppDependencies "ACS
	// email send" span can't see (that span only covers the synchronous accept call).
	_, err = monitor.NewMetricAlert(ctx, "alert-acs-email-delivery-shared", &monitor.MetricAlertArgs{
		RuleName:            pulumi.String("alert-acs-email-delivery-shared"),
		ResourceGroupName:   resourceGroup.Name,
		Location:            pulumi.String("global"),
		Description:         pulumi.String("ACS reported a non-Success terminal delivery status (bounce, block, etc.) for a previously-accepted email send."),
		Severity:            pulumi.Int(2),
		Enabled:             pulumi.Bool(true),
		AutoMitigate:        pulumi.Bool(true),
		EvaluationFrequency: pulumi.String("PT15M"),
		WindowSize:          pulumi.String("PT1H"),
		Scopes:              pulumi.StringArray{communicationServiceUk.ID()},
		Criteria: monitor.MetricAlertSingleResourceMultipleMetricCriteriaArgs{
			OdataType: pulumi.String("Microsoft.Azure.Monitor.SingleResourceMultipleMetricCriteria"),
			AllOf: monitor.MetricCriteriaArray{
				monitor.MetricCriteriaArgs{
					CriterionType:   pulumi.String("StaticThresholdCriterion"),
					Name:            pulumi.String("NonSuccessDeliveryStatusUpdates"),
					MetricName:      pulumi.String("DeliveryStatusUpdate"),
					MetricNamespace: pulumi.String("Microsoft.Communication/CommunicationServices"),
					Dimensions: monitor.MetricDimensionArray{
						monitor.MetricDimensionArgs{
							Name:     pulumi.String("Result"),
							Operator: pulumi.String("Exclude"),
							Values:   pulumi.StringArray{pulumi.String("Success")},
						},
					},
					Operator:        pulumi.String("GreaterThan"),
					Threshold:       pulumi.Float64(0),
					TimeAggregation: pulumi.String("Count"),
				},
			},
		},
		Actions: monitor.MetricAlertActionArray{
			monitor.MetricAlertActionArgs{ActionGroupId: actionGroup.ID()},
		},
		Tags: tags,
	})
	if err != nil {
		return err
	}

	// Phase 4: log query alerts. <prodfilter> throughout comments below refers to
	// `tostring(Properties["deployment.environment"]) == "prod"`, needed on every query that
	// could see dev telemetry (dev and prod share this App Insights component). The PlanIt rule
	// above and the Auth0 rule below are the documented exceptions (PlanIt: only prod polls;
	// Auth0: an Auth0 outage affects both environments).
	const apiFiveXXQuery = `AppRequests
| where AppRoleName has "api-go"
| where tostring(Properties["deployment.environment"]) == "prod"
| where toint(ResultCode) >= 500`
	if err = createLogAlert(ctx, resourceGroup, logAnalytics.ID(), actionGroup, tags, logAlertSpec{
		name:        "alert-api-5xx-prod",
		displayName: "API 5xx errors (prod)",
		description: "More than 10 prod API requests returned a 5xx status in the last 15 minutes.",
		severity:    1,
		window:      "PT15M",
		freq:        "PT5M",
		query:       apiFiveXXQuery,
		threshold:   10,
	}); err != nil {
		return err
	}

	const appstoreWebhookFailuresQuery = `AppRequests
| where Name == "POST /v1/webhooks/appstore"
| where tostring(Properties["deployment.environment"]) == "prod"
| where toint(ResultCode) >= 500`
	if err = createLogAlert(ctx, resourceGroup, logAnalytics.ID(), actionGroup, tags, logAlertSpec{
		name:        "alert-appstore-webhook-failures-prod",
		displayName: "App Store webhook failures (prod)",
		description: "The App Store server-notifications webhook returned a 5xx in prod. Persistent 5xx here permanently loses App Store subscription events once Apple stops retrying.",
		severity:    2,
		window:      "PT1H",
		freq:        "PT15M",
		query:       appstoreWebhookFailuresQuery,
		threshold:   0,
	}); err != nil {
		return err
	}

	// Same shape as the PlanIt rule above: the query itself computes the failure ratio and
	// emits a row only when the >=10-call, >20%-failure threshold is breached, so the alert
	// condition is just "did the query return anything" (threshold 0, Count aggregation).
	const apnsFailureRateQuery = `AppDependencies
| where Name == "APNs push"
| where tostring(Properties["deployment.environment"]) == "prod"
| summarize Total = count(), Fails = countif(Success == false)
| where Total >= 10
| extend FailureRatioPercent = round(100.0 * Fails / Total, 1)
| where FailureRatioPercent > 20`
	if err = createLogAlert(ctx, resourceGroup, logAnalytics.ID(), actionGroup, tags, logAlertSpec{
		name:        "alert-apns-failure-rate-prod",
		displayName: "APNs push failure rate (prod)",
		description: "APNs push failure ratio exceeded 20% over the last hour (>=10 sends). See GH #943.",
		severity:    2,
		window:      "PT1H",
		freq:        "PT15M",
		query:       apnsFailureRateQuery,
		threshold:   0,
	}); err != nil {
		return err
	}

	const acsEmailSendFailuresQuery = `AppDependencies
| where Name == "ACS email send"
| where tostring(Properties["deployment.environment"]) == "prod"
| where Success == false`
	if err = createLogAlert(ctx, resourceGroup, logAnalytics.ID(), actionGroup, tags, logAlertSpec{
		name:        "alert-acs-email-send-failures-prod",
		displayName: "ACS email send failures (prod)",
		description: "More than 2 prod ACS email send calls failed in the last hour.",
		severity:    2,
		window:      "PT1H",
		freq:        "PT15M",
		query:       acsEmailSendFailuresQuery,
		threshold:   2,
	}); err != nil {
		return err
	}

	// No <prodfilter> — an Auth0 outage affects both environments, and the token cache masks
	// blips, so sustained failures here are real regardless of which env saw them.
	const auth0FailuresQuery = `AppDependencies
| where Name startswith "Auth0"
| where Success == false`
	if err = createLogAlert(ctx, resourceGroup, logAnalytics.ID(), actionGroup, tags, logAlertSpec{
		name:        "alert-auth0-failures-shared",
		displayName: "Auth0 dependency failures",
		description: "More than 2 Auth0 dependency calls failed in the last 30 minutes (either environment).",
		severity:    2,
		window:      "PT30M",
		freq:        "PT15M",
		query:       auth0FailuresQuery,
		threshold:   2,
	}); err != nil {
		return err
	}

	// Job-absence alerts. Failed executions are covered by the Phase 3 Container Apps job
	// metric alerts above (a worker that exits 1 leaves the execution in a Failed state); these
	// two log alerts instead catch "never ran at all", which a metric alert can't express. The
	// expected-vs-actual leftouter join guarantees a row (Runs == 0) even when AppDependencies
	// has zero matching spans, which a plain `| where` filter on an empty result set cannot.
	const jobAbsenceFrequentQuery = `let expected = datatable(JobSpan:string)["Polling Cycle (SB)", "Polling Bootstrap", "Hourly Digest Cycle"];
expected
| join kind=leftouter (
    AppDependencies
    | where tostring(Properties["deployment.environment"]) == "prod"
    | summarize Runs = count() by Name
) on $left.JobSpan == $right.Name
| extend Runs = coalesce(Runs, 0)
| where Runs == 0`
	if err = createLogAlert(ctx, resourceGroup, logAnalytics.ID(), actionGroup, tags, logAlertSpec{
		name:        "alert-worker-absence-frequent-prod",
		displayName: "Frequent worker cycle absent (prod)",
		description: "A prod job expected to run at least hourly (poll, poll-bootstrap, or hourly-digest) produced no AppDependencies span in the last 3 hours. Dimensioned by JobSpan so the fired alert names the missing job.",
		severity:    2,
		window:      "PT3H",
		freq:        "PT30M",
		query:       jobAbsenceFrequentQuery,
		threshold:   0,
		dimensions: monitor.DimensionArray{
			monitor.DimensionArgs{
				Name:     pulumi.String("JobSpan"),
				Operator: pulumi.String("Include"),
				Values:   pulumi.StringArray{pulumi.String("*")},
			},
		},
	}); err != nil {
		return err
	}

	const jobAbsenceDailyQuery = `let expected = datatable(JobSpan:string)["Digest Cycle", "Dormant Cleanup Cycle", "Subscription Sweep Cycle", "Postgres Purge Cycle"];
expected
| join kind=leftouter (
    AppDependencies
    | where tostring(Properties["deployment.environment"]) == "prod"
    | summarize Runs = count() by Name
) on $left.JobSpan == $right.Name
| extend Runs = coalesce(Runs, 0)
| where Runs == 0`
	if err = createLogAlert(ctx, resourceGroup, logAnalytics.ID(), actionGroup, tags, logAlertSpec{
		name:        "alert-worker-absence-daily-prod",
		displayName: "Daily worker cycle absent (prod)",
		description: "A prod job expected to run at least daily (digest, dormant-cleanup, subscription-sweep, or pg-purge) produced no AppDependencies span in the last 24 hours. Dimensioned by JobSpan so the fired alert names the missing job.",
		severity:    2,
		window:      "P1D",
		freq:        "PT1H",
		query:       jobAbsenceDailyQuery,
		threshold:   0,
		dimensions: monitor.DimensionArray{
			monitor.DimensionArgs{
				Name:     pulumi.String("JobSpan"),
				Operator: pulumi.String("Include"),
				Values:   pulumi.StringArray{pulumi.String("*")},
			},
		},
	}); err != nil {
		return err
	}

	// Phase 5: subscription-level hygiene.

	// Service Health — replaces reliance on the auto-created azureapp-auto action group, which
	// only pushes to the Azure mobile app and isn't itself managed anywhere.
	_, err = monitor.NewActivityLogAlert(ctx, "alert-service-health-shared", &monitor.ActivityLogAlertArgs{
		ActivityLogAlertName: pulumi.String("alert-service-health-shared"),
		ResourceGroupName:    resourceGroup.Name,
		Location:             pulumi.String("Global"),
		Description:          pulumi.String("Azure Service Health event (incident, maintenance, informational, or security advisory) affecting this subscription."),
		Enabled:              pulumi.Bool(true),
		Scopes:               pulumi.StringArray{pulumi.String(fmt.Sprintf("/subscriptions/%s", armSubscriptionID))},
		Condition: monitor.AlertRuleAllOfConditionArgs{
			AllOf: monitor.AlertRuleAnyOfOrLeafConditionArray{
				monitor.AlertRuleAnyOfOrLeafConditionArgs{
					Field:  pulumi.String("category"),
					Equals: pulumi.String("ServiceHealth"),
				},
			},
		},
		Actions: monitor.ActionListArgs{
			ActionGroups: monitor.ActionGroupTypeArray{
				monitor.ActionGroupTypeArgs{ActionGroupId: actionGroup.ID()},
			},
		},
		Tags: tags,
	})
	if err != nil {
		return err
	}

	// Failure Anomalies — Application Insights' built-in smart detector for abnormal rises in
	// failed request rate, wired to the same action group as everything else here.
	_, err = alertsmanagement.NewSmartDetectorAlertRule(ctx, "alert-failure-anomalies-shared", &alertsmanagement.SmartDetectorAlertRuleArgs{
		AlertRuleName:     pulumi.String("alert-failure-anomalies-shared"),
		ResourceGroupName: resourceGroup.Name,
		Location:          pulumi.String("global"),
		Description:       pulumi.String("Application Insights Failure Anomalies smart detection on the shared App Insights component."),
		State:             alertsmanagement.AlertRuleStateEnabled,
		Severity:          alertsmanagement.SeveritySev3,
		Frequency:         pulumi.String("PT1M"),
		Scope:             pulumi.StringArray{appInsights.ID()},
		Detector: alertsmanagement.DetectorArgs{
			Id: pulumi.String("FailureAnomaliesDetector"),
		},
		ActionGroups: alertsmanagement.ActionGroupsInformationArgs{
			GroupIds: pulumi.StringArray{actionGroup.ID()},
		},
		Tags: tags,
	})
	if err != nil {
		return err
	}

	// Operational Dashboard
	appInsightsID := appInsights.ID().ToStringOutput()
	_, err = portal.NewDashboard(ctx, "dash-towncrier-operational", &portal.DashboardArgs{
		DashboardName:     pulumi.String("dash-towncrier-operational"),
		ResourceGroupName: resourceGroup.Name,
		Location:          resourceGroup.Location,
		Tags:              tags,
		Properties: &portal.DashboardPropertiesWithProvisioningStateArgs{
			Lenses: portal.DashboardLensArray{
				&portal.DashboardLensArgs{
					Order: pulumi.Int(0),
					Parts: portal.DashboardPartsArray{
						// Row 1: Users & Engagement
						dashboardPart(0, 0, 4, 4, kqlTile(appInsightsID,
							"let data = requests | where name == 'GET /v1/me' | summarize Value=dcount(user_AuthenticatedId) by timestamp=bin(timestamp, 1h); let empty = datatable(timestamp:datetime, Value:long)[]; union data, empty | render timechart",
							"Active Users")),
						dashboardPart(4, 0, 4, 4, metricTile(appInsightsID, "towncrier.users.registered", "Registrations")),
						dashboardPart(8, 0, 4, 4, metricTile(appInsightsID, "towncrier.search.performed", "Searches")),
						// Row 2: Watch Zones & Notifications
						dashboardPart(0, 4, 4, 4, metricTile(appInsightsID, "towncrier.watchzones.created", "Watch Zones Created")),
						dashboardPart(4, 4, 4, 4, metricTile(appInsightsID, "towncrier.watchzones.deleted", "Watch Zones Deleted")),
						dashboardPart(8, 4, 4, 4, metricTile(appInsightsID, "towncrier.notifications.sent", "Notifications Sent")),
						// Row 3: Sync & Infrastructure Health
						dashboardPart(0, 8, 4, 4, metricChartTile(appInsightsID, "Sync Success vs Failure",
							metricSpec{name: "towncrier.polling.authorities_polled", label: "Successes"},
							metricSpec{name: "towncrier.polling.failures", label: "Failures"})),
						dashboardPart(4, 8, 4, 4, metricTile(appInsightsID, "towncrier.polling.applications_ingested", "Applications Ingested")),
						dashboardPart(8, 8, 4, 4, metricTile(appInsightsID, "towncrier.api.errors", "API Errors")),
						// Row 4: PlanIt API Health
						dashboardPart(0, 12, 6, 4, kqlTile(appInsightsID,
							"customMetrics | where name == 'towncrier.planit.http_errors' | extend status = tostring(customDimensions['http.response.status_code']) | where status == '429' | summarize Value=sum(value) by timestamp=bin(timestamp, 1h) | render timechart",
							"PlanIt 429s")),
						dashboardPart(6, 12, 6, 4, kqlTile(appInsightsID,
							"customMetrics | where name == 'towncrier.planit.http_errors' | extend status = tostring(customDimensions['http.response.status_code']) | where status != '429' | summarize Value=sum(value) by timestamp=bin(timestamp, 1h), status | render timechart",
							"PlanIt Errors")),
						// Row 5: Email
						dashboardPart(0, 16, 6, 4, metricTile(appInsightsID, "towncrier.emails.sent", "Emails Sent")),
						dashboardPart(6, 16, 6, 4, metricTile(appInsightsID, "towncrier.emails.failed", "Email Failures")),
					},
				},
			},
		},
	})
	if err != nil {
		return err
	}

	ctx.Export("resourceGroupName", resourceGroup.Name)
	ctx.Export("containerRegistryLoginServer", containerRegistry.LoginServer)
	ctx.Export("acrPullIdentityId", acrPullIdentity.ID())
	ctx.Export("cosmosDataIdentityId", cosmosDataIdentity.ID())
	ctx.Export("cosmosDataIdentityClientId", cosmosDataIdentity.ClientId)
	// PrincipalId is required by env-stack role assignments (Service Bus Data Owner on the
	// polling namespace) because RBAC grants are keyed to the principal (object) ID.
	ctx.Export("cosmosDataIdentityPrincipalId", cosmosDataIdentity.PrincipalId)
	ctx.Export("devSeedReaderIdentityId", devSeedReaderIdentity.ID())
	ctx.Export("devSeedReaderIdentityClientId", devSeedReaderIdentity.ClientId)
	// PrincipalId is the OID pgbootstrap maps to the SELECT-only Postgres role
	// (pgaadauth_create_principal_with_oid) — see
	// docs/adr/0038-dev-seed-least-privilege-prod-read.md.
	ctx.Export("devSeedReaderIdentityPrincipalId", devSeedReaderIdentity.PrincipalId)
	ctx.Export("containerAppsEnvironmentId", containerAppsEnv.ID())
	// Postgres Flexible Server (Cosmos → Postgres migration, epic tc-hpd2). The admin password
	// is a Pulumi secret (RandomPassword.Result); env stacks read these to build per-env
	// database connection strings.
	ctx.Export("postgresServerName", postgresServer.Name)
	ctx.Export("postgresServerFqdn", postgresServer.FullyQualifiedDomainName)
	ctx.Export("postgresAdminLogin", pulumi.String("tcadmin"))
	ctx.Export("postgresAdminPassword", postgresAdminPassword.Result)
	ctx.Export("appInsightsId", appInsights.ID())
	ctx.Export("appInsightsConnectionString", appInsights.ConnectionString)
	// Action group (tc-ttjor / GH #938 PR3): consumed by the prod env stack's poll-queue-depth
	// metric alert (infra/environment.go) so both alerts notify the same email receiver.
	ctx.Export("actionGroupId", actionGroup.ID())
	ctx.Export("acsConnectionString", communication.ListCommunicationServiceKeysOutput(ctx, communication.ListCommunicationServiceKeysOutputArgs{
		ResourceGroupName:        resourceGroup.Name,
		CommunicationServiceName: communicationServiceUk.Name,
	}).PrimaryConnectionString())

	return nil
}

// dashboardPart builds one positioned dashboard part from a metadata tile.
func dashboardPart(x, y, colSpan, rowSpan int, metadata portal.DashboardPartMetadataArgs) portal.DashboardPartsArgs {
	return portal.DashboardPartsArgs{
		Position: portal.DashboardPartsPositionArgs{
			X:       pulumi.Int(x),
			Y:       pulumi.Int(y),
			ColSpan: pulumi.Int(colSpan),
			RowSpan: pulumi.Int(rowSpan),
		},
		Metadata: metadata,
	}
}

// metricSpec is one App Insights custom metric series within a chart tile.
type metricSpec struct {
	name  string // custom metric name, e.g. "towncrier.users.registered"
	label string // series display name shown in the chart legend
}

// metricChartTile renders one or more App Insights custom metrics as a MonitorChartPart.
func metricChartTile(appInsightsID pulumi.StringOutput, title string, metrics ...metricSpec) portal.DashboardPartMetadataArgs {
	series := make(pulumi.Array, len(metrics))
	for i, m := range metrics {
		series[i] = pulumi.Map{
			"resourceMetadata":    pulumi.Map{"id": appInsightsID},
			"name":                pulumi.String(m.name),
			"aggregationType":     pulumi.Int(1),
			"namespace":           pulumi.String("azure.applicationinsights"),
			"metricVisualization": pulumi.Map{"displayName": pulumi.String(m.label)},
		}
	}
	return portal.DashboardPartMetadataArgs{
		Type: pulumi.String("Extension/HubsExtension/PartType/MonitorChartPart"),
		Settings: pulumi.Map{
			"content": pulumi.Map{
				"options": pulumi.Map{
					"chart": pulumi.Map{
						"metrics":       series,
						"title":         pulumi.String(title),
						"titleKind":     pulumi.Int(1),
						"visualization": pulumi.Map{"chartType": pulumi.Int(2)},
						"timespan": pulumi.Map{
							"relative": pulumi.Map{"duration": pulumi.Int(dashboardTimespan24HoursMs)},
						},
					},
				},
			},
		},
	}
}

// metricTile renders a single custom metric whose series label matches the chart title.
func metricTile(appInsightsID pulumi.StringOutput, metricName, title string) portal.DashboardPartMetadataArgs {
	return metricChartTile(appInsightsID, title, metricSpec{name: metricName, label: title})
}

// kqlTile renders an Analytics (KQL query) dashboard part bound to the App Insights component.
func kqlTile(appInsightsID pulumi.StringOutput, query, title string) portal.DashboardPartMetadataArgs {
	componentID := appInsightsID.ApplyT(func(id string) map[string]interface{} {
		segments := strings.Split(id, "/")
		return map[string]interface{}{
			"SubscriptionId": segments[2],
			"ResourceGroup":  segments[4],
			"Name":           segments[8],
			"ResourceId":     id,
		}
	})

	return portal.DashboardPartMetadataArgs{
		Type: pulumi.String("Extension/AppInsightsExtension/PartType/AnalyticsPart"),
		Settings: pulumi.Map{
			"content": pulumi.Map{
				"Query":                   pulumi.String(query),
				"ControlType":             pulumi.String("FrameControlChart"),
				"SpecificChart":           pulumi.String("Line"),
				"PartTitle":               pulumi.String(title),
				"IsQueryContainTimeRange": pulumi.Bool(false),
				"Dimensions": pulumi.Map{
					"xAxis": pulumi.Map{
						"name": pulumi.String("timestamp"),
						"type": pulumi.String("datetime"),
					},
					"yAxis": pulumi.Array{
						pulumi.Map{
							"name": pulumi.String("Value"),
							"type": pulumi.String("long"),
						},
					},
				},
			},
		},
		Inputs: pulumi.Array{
			pulumi.Map{
				"name":  pulumi.String("ComponentId"),
				"value": componentID,
			},
		},
	}
}

// availabilityTestLocations is the standard 3-location EMEA coverage set shared by both
// availability WebTests below (GH #943 Phase 2).
var availabilityTestLocations = appinsights.WebTestGeolocationArray{
	appinsights.WebTestGeolocationArgs{Location: pulumi.String("emea-gb-db3-azr")},
	appinsights.WebTestGeolocationArgs{Location: pulumi.String("emea-nl-ams-azr")},
	appinsights.WebTestGeolocationArgs{Location: pulumi.String("emea-fr-pra-edge")},
}

// webTestTags merges the standard tag set with the "hidden-link:<appInsightsID>": "Resource"
// tag Azure requires to associate a WebTest with its Application Insights component. The tag
// KEY embeds the component's resource ID, so the whole map can only be known once appInsightsID
// resolves — hence building it inside ApplyT rather than as a plain pulumi.StringMap literal.
// tags values are always literal pulumi.String (see the tags built in main.go), so the type
// assertion below is safe.
func webTestTags(appInsightsID pulumi.IDOutput, tags pulumi.StringMap) pulumi.StringMapOutput {
	return appInsightsID.ApplyT(func(id pulumi.ID) map[string]string {
		merged := make(map[string]string, len(tags)+1)
		for k, v := range tags {
			if s, ok := v.(pulumi.String); ok {
				merged[k] = string(s)
			}
		}
		merged[fmt.Sprintf("hidden-link:%s", id)] = "Resource"
		return merged
	}).(pulumi.StringMapOutput)
}

// createAvailabilityCheck provisions one Application Insights standard WebTest against url,
// plus the companion MetricAlert that fires when >=2 of the 3 EMEA probe locations report it
// down (GH #943 Phase 2). name is used as both the WebTest's logical/Azure resource name (e.g.
// "webtest-api-prod") and the basis for its alert's name.
func createAvailabilityCheck(ctx *pulumi.Context, resourceGroup *resources.ResourceGroup, appInsights *appinsights.Component, actionGroup *monitor.ActionGroup, tags pulumi.StringMap, name, url string) error {
	webTest, err := appinsights.NewWebTest(ctx, name, &appinsights.WebTestArgs{
		WebTestName:        pulumi.String(name),
		ResourceGroupName:  resourceGroup.Name,
		Location:           pulumi.String("uksouth"),
		Kind:               appinsights.WebTestKindStandard,
		WebTestKind:        appinsights.WebTestKindStandard,
		SyntheticMonitorId: pulumi.String(name),
		Enabled:            pulumi.Bool(true),
		Frequency:          pulumi.Int(300),
		Timeout:            pulumi.Int(30),
		RetryEnabled:       pulumi.Bool(true),
		Locations:          availabilityTestLocations,
		Request: &appinsights.WebTestPropertiesRequestArgs{
			RequestUrl: pulumi.String(url),
			HttpVerb:   pulumi.String("GET"),
		},
		ValidationRules: &appinsights.WebTestPropertiesValidationRulesArgs{
			ExpectedHttpStatusCode: pulumi.Int(200),
		},
		Tags: webTestTags(appInsights.ID(), tags),
	})
	if err != nil {
		return err
	}

	alertName := fmt.Sprintf("alert-%s-shared", name)
	_, err = monitor.NewMetricAlert(ctx, alertName, &monitor.MetricAlertArgs{
		RuleName:            pulumi.String(alertName),
		ResourceGroupName:   resourceGroup.Name,
		Location:            pulumi.String("global"),
		Description:         pulumi.String(fmt.Sprintf("Availability test %s failed from >=2 of 3 EMEA locations.", name)),
		Severity:            pulumi.Int(1),
		Enabled:             pulumi.Bool(true),
		AutoMitigate:        pulumi.Bool(true),
		EvaluationFrequency: pulumi.String("PT5M"),
		WindowSize:          pulumi.String("PT15M"),
		Scopes:              pulumi.StringArray{webTest.ID(), appInsights.ID()},
		Criteria: monitor.WebtestLocationAvailabilityCriteriaArgs{
			OdataType:           pulumi.String("Microsoft.Azure.Monitor.WebtestLocationAvailabilityCriteria"),
			WebTestId:           webTest.ID(),
			ComponentId:         appInsights.ID(),
			FailedLocationCount: pulumi.Float64(2),
		},
		Actions: monitor.MetricAlertActionArray{
			monitor.MetricAlertActionArgs{ActionGroupId: actionGroup.ID()},
		},
		Tags: tags,
	})
	return err
}

// logAlertSpec describes one ScheduledQueryRule (Phase 4 log query alerts). dimensions is
// optional (nil for the plain row-count alerts; set for the two job-absence alerts so the
// fired alert names the missing job via the JobSpan column).
type logAlertSpec struct {
	name        string
	displayName string
	description string
	severity    float64
	window      string
	freq        string
	query       string
	threshold   float64
	dimensions  monitor.DimensionArray
}

// createLogAlert provisions one ScheduledQueryRule (Kind LogAlert) scoped to the shared Log
// Analytics workspace, wired to actionGroup. Every query is expected to already filter itself
// down to the rows that should fire the alert (mirrors the PlanIt rule above), so the criterion
// is always "count of returned rows compares against threshold" with Count aggregation.
func createLogAlert(ctx *pulumi.Context, resourceGroup *resources.ResourceGroup, logAnalyticsID pulumi.IDOutput, actionGroup *monitor.ActionGroup, tags pulumi.StringMap, spec logAlertSpec) error {
	condition := monitor.ConditionArgs{
		Query:           pulumi.String(spec.query),
		TimeAggregation: pulumi.String("Count"),
		Operator:        pulumi.String("GreaterThan"),
		Threshold:       pulumi.Float64(spec.threshold),
		Dimensions:      spec.dimensions,
	}
	_, err := monitor.NewScheduledQueryRule(ctx, spec.name, &monitor.ScheduledQueryRuleArgs{
		RuleName:            pulumi.String(spec.name),
		ResourceGroupName:   resourceGroup.Name,
		Location:            pulumi.String("uksouth"),
		Kind:                pulumi.String("LogAlert"),
		Description:         pulumi.String(spec.description),
		DisplayName:         pulumi.String(spec.displayName),
		Severity:            pulumi.Float64(spec.severity),
		Enabled:             pulumi.Bool(true),
		EvaluationFrequency: pulumi.String(spec.freq),
		WindowSize:          pulumi.String(spec.window),
		Scopes:              pulumi.StringArray{logAnalyticsID},
		Criteria: monitor.ScheduledQueryRuleCriteriaArgs{
			AllOf: monitor.ConditionArray{condition},
		},
		Actions: monitor.ActionsArgs{
			ActionGroups: pulumi.StringArray{actionGroup.ID()},
		},
		Tags: tags,
	})
	return err
}
