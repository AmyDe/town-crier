package main

import (
	"fmt"
	"os"
	"strings"

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

	// Allowlist the PostGIS extension via the azure.extensions server parameter. Without it,
	// `CREATE EXTENSION postgis` is rejected. Source must be "user-override" or Azure ignores
	// the assigned value. A server parameter change is a server-level operation that cannot run
	// while the server is busy with another operation, so this is serialised after the firewall
	// rule (DependsOn) — applying both concurrently right after server creation trips
	// "ServerIsBusy".
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
		Value:             pulumi.String("POSTGIS"),
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
		},
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
