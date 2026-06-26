package main

import (
	"fmt"
	"strings"

	"github.com/pulumi/pulumi-azure-native-sdk/app/v3"
	"github.com/pulumi/pulumi-azure-native-sdk/authorization/v3"
	"github.com/pulumi/pulumi-azure-native-sdk/cosmosdb/v3"
	"github.com/pulumi/pulumi-azure-native-sdk/dbforpostgresql/v3"
	"github.com/pulumi/pulumi-azure-native-sdk/resources/v3"
	"github.com/pulumi/pulumi-azure-native-sdk/servicebus/v3"
	"github.com/pulumi/pulumi-azure-native-sdk/storage/v3"
	"github.com/pulumi/pulumi-azure-native-sdk/web/v3"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

const (
	// containerCpu is the CPU cores allocated to each container (App and Job).
	containerCpu = 0.25
	// containerMemory is the memory allocated to each container (App and Job).
	containerMemory = "0.5Gi"
	// apnsBundleID is the iOS app identifier; not user-configurable per stack.
	apnsBundleID = "uk.towncrierapp.mobile"
)

// cloudflareIPv4Ranges is Cloudflare's published list of IPv4 origin-pull ranges.
//
// Snapshot of https://www.cloudflare.com/ips-v4, fetched 2026-06-19. The Go API
// container app is fronted by the Cloudflare orange-cloud proxy (tc-j222); these
// ranges are applied as Allow rules on the ACA ingress (see
// cloudflareIngressIPRestrictions) so the *.azurecontainerapps.io origin only
// accepts traffic that arrives via Cloudflare.
//
// IPv4 ONLY — and deliberately so: the prod origin FQDN
// ca-town-crier-api-go-prod.<env>.uksouth.azurecontainerapps.io has an A record
// only and no AAAA (resolves to 85.210.27.198), so Cloudflare reaches the origin
// over IPv4. ACA ipSecurityRestrictions also only accepts IPv4 CIDRs. Adding the
// IPv6 ranges (https://www.cloudflare.com/ips-v6) would therefore be both
// unreachable in practice and rejected by ACA. Refresh this snapshot if
// Cloudflare publishes new ranges.
var cloudflareIPv4Ranges = []string{
	"173.245.48.0/20",
	"103.21.244.0/22",
	"103.22.200.0/22",
	"103.31.4.0/22",
	"141.101.64.0/18",
	"108.162.192.0/18",
	"190.93.240.0/20",
	"188.114.96.0/20",
	"197.234.240.0/22",
	"198.41.128.0/17",
	"162.158.0.0/15",
	"104.16.0.0/13",
	"104.24.0.0/14",
	"172.64.0.0/13",
	"131.0.72.0/22",
}

// cloudflareIngressIPRestrictions builds the ACA ingress ipSecurityRestrictions:
// one Allow rule per Cloudflare IPv4 range. Once any Allow rule is present, ACA
// denies every other source, so only Cloudflare can reach the origin. The same
// list is applied to both prod and dev (api-dev is also Cloudflare-proxied).
//
// This is safe and fully reversible: ACME managed-certificate renewal and all
// real client traffic arrive via the proxied hostname, so their source is a
// Cloudflare IP and is allowed. Removing the rules restores open ingress.
func cloudflareIngressIPRestrictions() app.IpSecurityRestrictionRuleArray {
	rules := make(app.IpSecurityRestrictionRuleArray, 0, len(cloudflareIPv4Ranges))
	for i, cidr := range cloudflareIPv4Ranges {
		rules = append(rules, &app.IpSecurityRestrictionRuleArgs{
			Action:         pulumi.String(string(app.ActionAllow)),
			Name:           pulumi.String(fmt.Sprintf("cloudflare-ipv4-%02d", i+1)),
			IpAddressRange: pulumi.String(cidr),
			Description:    pulumi.String(fmt.Sprintf("Cloudflare published IPv4 range %s", cidr)),
		})
	}
	return rules
}

// cosmosContainerDefinition defines a Cosmos DB container with its partition key and
// optional advanced settings.
type cosmosContainerDefinition struct {
	name             string
	partitionKeyPath string
	defaultTTL       *int
	uniqueKeyPaths   [][]string
	indexingPolicy   cosmosdb.IndexingPolicyPtrInput
}

// serviceBusPollingInfra captures the Service Bus resources used by the adaptive polling
// trigger: namespace (short name + FQDN) and queue name.
type serviceBusPollingInfra struct {
	namespaceShortName pulumi.StringOutput
	namespaceFqdn      pulumi.StringOutput
	queueName          pulumi.StringOutput
}

// envContext holds the shared inputs every worker job and the container app need.
type envContext struct {
	env                         string
	resourceGroupName           pulumi.StringOutput
	environmentID               pulumi.StringOutput
	acrLoginServer              pulumi.StringOutput
	acrPullIdentityID           pulumi.StringOutput
	cosmosDataIdentityID        pulumi.StringOutput
	cosmosAccountEndpoint       pulumi.StringOutput
	cosmosDatabaseName          pulumi.StringOutput
	cosmosDataIdentityClientID  pulumi.StringOutput
	appInsightsConnectionString pulumi.StringOutput
	acsConnectionString         pulumi.StringOutput
	apnsAuthKey                 pulumi.StringOutput
	apnsUseSandbox              string
	auth0Domain                 string
	auth0M2mClientID            pulumi.StringOutput
	auth0M2mClientSecret        pulumi.StringOutput
	tags                        pulumi.StringMap
}

func intPtr(v int) *int { return &v }

func runEnvironmentStack(ctx *pulumi.Context, conf *config.Config, env string, tags pulumi.StringMap) error {
	frontendDomain := conf.Require("frontendDomain")
	apiDomain := conf.Require("apiDomain")
	auth0Domain := conf.Require("auth0Domain")
	// CI OIDC identity (the town-crier-github-actions service principal) object ID. Granted
	// Storage Blob Data Contributor on the SEO snapshot account below. Same principal the
	// shared stack grants AcrPush; supplied via config because it's an external app reg.
	ciServicePrincipalID := conf.Require("ciServicePrincipalId")
	auth0Audience := conf.Require("auth0Audience")
	customDomainPhase := 2
	if v, err := conf.TryInt("customDomainPhase"); err == nil {
		customDomainPhase = v
	}
	adminAPIKey := conf.RequireSecret("adminApiKey")
	// Build key the Go endpoint validates for the gated SEO prerender route (tc-nnte).
	siteBuildKey := conf.RequireSecret("siteBuildKey")
	auth0M2mClientID := conf.RequireSecret("auth0M2mClientId")
	auth0M2mClientSecret := conf.RequireSecret("auth0M2mClientSecret")

	// APNs (Apple Push Notification service). AuthKey is the .p8 PEM contents (secret).
	// UseSandbox derives from the env: dev hits api.sandbox.push.apple.com.
	apnsAuthKey := conf.RequireSecret("apnsAuthKey")
	apnsUseSandbox := "false"
	if env == "dev" {
		apnsUseSandbox = "true"
	}

	// Shared stack outputs
	shared, err := pulumi.NewStackReference(ctx, "AmyDe/town-crier/shared", nil)
	if err != nil {
		return err
	}
	acrLoginServer := shared.GetStringOutput(pulumi.String("containerRegistryLoginServer"))
	acrPullIdentityID := shared.GetStringOutput(pulumi.String("acrPullIdentityId"))
	containerAppsEnvironmentID := shared.GetStringOutput(pulumi.String("containerAppsEnvironmentId"))
	cosmosDataIdentityID := shared.GetStringOutput(pulumi.String("cosmosDataIdentityId"))
	cosmosDataIdentityClientID := shared.GetStringOutput(pulumi.String("cosmosDataIdentityClientId"))
	cosmosAccountName := shared.GetStringOutput(pulumi.String("cosmosAccountName"))
	cosmosAccountEndpoint := shared.GetStringOutput(pulumi.String("cosmosAccountEndpoint"))
	appInsightsConnectionString := shared.GetStringOutput(pulumi.String("appInsightsConnectionString"))
	acsConnectionString := shared.GetStringOutput(pulumi.String("acsConnectionString"))
	sharedResourceGroupName := shared.GetStringOutput(pulumi.String("resourceGroupName"))
	// Extract the CAE name from its resource ID to avoid requiring a shared stack deploy
	// before the env stack can preview.
	containerAppsEnvironmentName := containerAppsEnvironmentID.ApplyT(func(id string) string {
		segments := strings.Split(id, "/")
		if len(segments) > 0 {
			return segments[len(segments)-1]
		}
		return ""
	}).(pulumi.StringOutput)

	// Resource Group
	resourceGroup, err := resources.NewResourceGroup(ctx, fmt.Sprintf("rg-town-crier-%s", env), &resources.ResourceGroupArgs{
		ResourceGroupName: pulumi.String(fmt.Sprintf("rg-town-crier-%s", env)),
		Tags:              tags,
	})
	if err != nil {
		return err
	}

	// Cosmos DB Database (in shared account)
	cosmosDatabase, err := cosmosdb.NewSqlResourceSqlDatabase(ctx, fmt.Sprintf("db-town-crier-%s", env), &cosmosdb.SqlResourceSqlDatabaseArgs{
		AccountName:       cosmosAccountName,
		ResourceGroupName: sharedResourceGroupName,
		DatabaseName:      pulumi.String(fmt.Sprintf("town-crier-%s", env)),
		Resource: &cosmosdb.SqlDatabaseResourceArgs{
			Id: pulumi.String(fmt.Sprintf("town-crier-%s", env)),
		},
	})
	if err != nil {
		return err
	}

	// Cosmos DB Containers — definition slice + creation loop
	containerDefinitions := []cosmosContainerDefinition{
		// Applications — partitioned by authority code, spatial index on location
		{
			name:             "Applications",
			partitionKeyPath: "/authorityCode",
			defaultTTL:       intPtr(-1), // TTL enabled, per-document control
			uniqueKeyPaths:   [][]string{{"/planitName"}},
			indexingPolicy: &cosmosdb.IndexingPolicyArgs{
				Automatic:    pulumi.Bool(true),
				IndexingMode: cosmosdb.IndexingModeConsistent,
				IncludedPaths: cosmosdb.IncludedPathArray{
					&cosmosdb.IncludedPathArgs{Path: pulumi.String("/authorityCode/?")},
					&cosmosdb.IncludedPathArgs{Path: pulumi.String("/appState/?")},
					&cosmosdb.IncludedPathArgs{Path: pulumi.String("/appType/?")},
					&cosmosdb.IncludedPathArgs{Path: pulumi.String("/decidedDate/?")},
					&cosmosdb.IncludedPathArgs{Path: pulumi.String("/lastDifferent/?")},
				},
				ExcludedPaths: cosmosdb.ExcludedPathArray{
					&cosmosdb.ExcludedPathArgs{Path: pulumi.String("/*")},
					&cosmosdb.ExcludedPathArgs{Path: pulumi.String("/\"_etag\"/?")},
				},
				SpatialIndexes: cosmosdb.SpatialSpecArray{
					&cosmosdb.SpatialSpecArgs{
						Path:  pulumi.String("/location/?"),
						Types: pulumi.StringArray{pulumi.String(string(cosmosdb.SpatialTypePoint))},
					},
				},
				CompositeIndexes: cosmosdb.CompositePathArrayArray{
					cosmosdb.CompositePathArray{
						&cosmosdb.CompositePathArgs{Path: pulumi.String("/authorityCode"), Order: cosmosdb.CompositePathSortOrderAscending},
						&cosmosdb.CompositePathArgs{Path: pulumi.String("/lastDifferent"), Order: cosmosdb.CompositePathSortOrderDescending},
					},
				},
			},
		},
		// Users — partitioned by id
		{name: "Users", partitionKeyPath: "/id"},
		// WatchZones — partitioned by userId, unique on (userId, name).
		// Keeps Cosmos default full indexing (IncludedPaths "/*") so the Phase 1
		// authority pre-filter (WHERE c.authorityId = @authorityId, tc-8dud) stays
		// index-served; ADDS a GeoJSON Point spatial index on /location so the
		// Phase 2c index-served FindZonesContaining query (tc-qbq4) can bind to it.
		// Inert until that query switches to reference c.location.
		{
			name:             "WatchZones",
			partitionKeyPath: "/userId",
			uniqueKeyPaths:   [][]string{{"/userId", "/name"}},
			indexingPolicy: &cosmosdb.IndexingPolicyArgs{
				Automatic:    pulumi.Bool(true),
				IndexingMode: cosmosdb.IndexingModeConsistent,
				IncludedPaths: cosmosdb.IncludedPathArray{
					&cosmosdb.IncludedPathArgs{Path: pulumi.String("/*")},
				},
				ExcludedPaths: cosmosdb.ExcludedPathArray{
					&cosmosdb.ExcludedPathArgs{Path: pulumi.String("/\"_etag\"/?")},
				},
				SpatialIndexes: cosmosdb.SpatialSpecArray{
					&cosmosdb.SpatialSpecArgs{
						Path:  pulumi.String("/location/?"),
						Types: pulumi.StringArray{pulumi.String(string(cosmosdb.SpatialTypePoint))},
					},
				},
			},
		},
		// Notifications — partitioned by userId, 90-day TTL
		{name: "Notifications", partitionKeyPath: "/userId", defaultTTL: intPtr(90 * 24 * 60 * 60)},
		// NotificationState — one watermark document per user (read-state cutoff)
		{name: "NotificationState", partitionKeyPath: "/userId"},
		// Leases — for change feed processor checkpointing
		{name: "Leases", partitionKeyPath: "/id"},
		// DeviceRegistrations — partitioned by userId
		{name: "DeviceRegistrations", partitionKeyPath: "/userId"},
		// SavedApplications — partitioned by userId
		{name: "SavedApplications", partitionKeyPath: "/userId"},
		// PollState — single document storing last poll timestamp
		{name: "PollState", partitionKeyPath: "/id"},
		// OfferCodes — partitioned by code for point reads on redemption
		{name: "OfferCodes", partitionKeyPath: "/code"},
		// AppleNotifications — idempotency store for App Store Server Notifications
		{name: "AppleNotifications", partitionKeyPath: "/id"},
	}

	for _, def := range containerDefinitions {
		resourceArgs := &cosmosdb.SqlContainerResourceArgs{
			Id: pulumi.String(def.name),
			PartitionKey: &cosmosdb.ContainerPartitionKeyArgs{
				Paths: pulumi.StringArray{pulumi.String(def.partitionKeyPath)},
				Kind:  cosmosdb.PartitionKindHash,
			},
		}
		if def.defaultTTL != nil {
			resourceArgs.DefaultTtl = pulumi.Int(*def.defaultTTL)
		}
		if def.uniqueKeyPaths != nil {
			uniqueKeys := cosmosdb.UniqueKeyArray{}
			for _, paths := range def.uniqueKeyPaths {
				stringPaths := pulumi.StringArray{}
				for _, p := range paths {
					stringPaths = append(stringPaths, pulumi.String(p))
				}
				uniqueKeys = append(uniqueKeys, &cosmosdb.UniqueKeyArgs{Paths: stringPaths})
			}
			resourceArgs.UniqueKeyPolicy = &cosmosdb.UniqueKeyPolicyArgs{UniqueKeys: uniqueKeys}
		}
		if def.indexingPolicy != nil {
			resourceArgs.IndexingPolicy = def.indexingPolicy
		}

		_, err = cosmosdb.NewSqlResourceSqlContainer(ctx, fmt.Sprintf("container-%s-%s", strings.ToLower(def.name), env), &cosmosdb.SqlResourceSqlContainerArgs{
			AccountName:       cosmosAccountName,
			ResourceGroupName: sharedResourceGroupName,
			DatabaseName:      cosmosDatabase.Name,
			ContainerName:     pulumi.String(def.name),
			Resource:          resourceArgs,
		})
		if err != nil {
			return err
		}
	}

	// Managed Certificate for API custom domain (phase >= 2 binds it with SniEnabled).
	var apiManagedCert *app.ManagedCertificate
	if customDomainPhase >= 2 {
		apiManagedCert, err = app.NewManagedCertificate(ctx, fmt.Sprintf("cert-api-%s", env), &app.ManagedCertificateArgs{
			EnvironmentName:        containerAppsEnvironmentName,
			ManagedCertificateName: pulumi.String(fmt.Sprintf("cert-api-%s", env)),
			ResourceGroupName:      sharedResourceGroupName,
			Properties: &app.ManagedCertificatePropertiesArgs{
				SubjectName:             pulumi.String(apiDomain),
				DomainControlValidation: pulumi.String("CNAME"),
			},
		})
		if err != nil {
			return err
		}
	}

	// The Go app owns the api custom domain unconditionally. Phase >= 2 binds with
	// SniEnabled; phase 1 uses an empty list so Azure can validate the hostname first.
	goApiCustomDomains := app.CustomDomainArray{}
	if customDomainPhase >= 2 {
		goApiCustomDomains = app.CustomDomainArray{
			&app.CustomDomainArgs{
				Name:          pulumi.String(apiDomain),
				CertificateId: apiManagedCert.ID(),
				BindingType:   app.BindingTypeSniEnabled,
			},
		}
	}

	ec := envContext{
		env:                         env,
		resourceGroupName:           resourceGroup.Name,
		environmentID:               containerAppsEnvironmentID,
		acrLoginServer:              acrLoginServer,
		acrPullIdentityID:           acrPullIdentityID,
		cosmosDataIdentityID:        cosmosDataIdentityID,
		cosmosAccountEndpoint:       cosmosAccountEndpoint,
		cosmosDatabaseName:          cosmosDatabase.Name,
		cosmosDataIdentityClientID:  cosmosDataIdentityClientID,
		appInsightsConnectionString: appInsightsConnectionString,
		acsConnectionString:         acsConnectionString,
		apnsAuthKey:                 apnsAuthKey,
		apnsUseSandbox:              apnsUseSandbox,
		auth0Domain:                 auth0Domain,
		auth0M2mClientID:            auth0M2mClientID,
		auth0M2mClientSecret:        auth0M2mClientSecret,
		tags:                        tags,
	}

	// Dev-only: Postgres database for town_crier_dev on the shared Flexible Server.
	// Part of the Cosmos → Postgres + PostGIS migration (memo 0010, epic tc-hpd2 / GH #645).
	// The password-based connection-string secret is retired; the dev API authenticates via
	// Entra managed identity using a runtime token (no stored password). See GH #653.
	if env == "dev" {
		postgresServerName := shared.GetStringOutput(pulumi.String("postgresServerName"))

		_, err = dbforpostgresql.NewDatabase(ctx, fmt.Sprintf("psql-db-town-crier-%s", env), &dbforpostgresql.DatabaseArgs{
			DatabaseName:      pulumi.String("town_crier_dev"),
			ResourceGroupName: sharedResourceGroupName,
			ServerName:        postgresServerName,
			Charset:           pulumi.String("UTF8"),
			Collation:         pulumi.String("en_US.utf8"),
		})
		if err != nil {
			return err
		}
	}

	secrets := app.SecretArray{
		&app.SecretArgs{Name: pulumi.String("auth0-m2m-client-id"), Value: auth0M2mClientID},
		&app.SecretArgs{Name: pulumi.String("auth0-m2m-client-secret"), Value: auth0M2mClientSecret},
		// Admin key the Go X-Admin-Key gate validates for /v1/admin requests (tc-52t6).
		&app.SecretArgs{Name: pulumi.String("admin-api-key"), Value: adminAPIKey},
		// Build key the Go gate validates for the SEO prerender endpoint (tc-nnte).
		&app.SecretArgs{Name: pulumi.String("site-build-key"), Value: siteBuildKey},
	}

	// Build the API container env-var array. Postgres connection vars are dev-only: the dev
	// API authenticates to town_crier_dev via Entra managed identity (GH #653). AZURE_CLIENT_ID
	// is already present and reused for the token fetch — no duplication.
	apiEnvVars := app.EnvironmentVarArray{
		app.EnvironmentVarArgs{Name: pulumi.String("OTEL_SERVICE_NAME"), Value: pulumi.String("town-crier-api-go")},
		// Read by the in-process Azure Monitor metrics exporter (tc-0rt1).
		app.EnvironmentVarArgs{Name: pulumi.String("APPLICATIONINSIGHTS_CONNECTION_STRING"), Value: appInsightsConnectionString},
		app.EnvironmentVarArgs{Name: pulumi.String("COSMOS_ENDPOINT"), Value: cosmosAccountEndpoint},
		app.EnvironmentVarArgs{Name: pulumi.String("COSMOS_DATABASE"), Value: cosmosDatabase.Name},
		app.EnvironmentVarArgs{Name: pulumi.String("AZURE_CLIENT_ID"), Value: cosmosDataIdentityClientID},
		app.EnvironmentVarArgs{Name: pulumi.String("AUTH0_DOMAIN"), Value: pulumi.String(auth0Domain)},
		app.EnvironmentVarArgs{Name: pulumi.String("AUTH0_AUDIENCE"), Value: pulumi.String(auth0Audience)},
		app.EnvironmentVarArgs{Name: pulumi.String("CORS_ALLOWED_ORIGINS"), Value: pulumi.String(fmt.Sprintf("https://%s", frontendDomain))},
		app.EnvironmentVarArgs{Name: pulumi.String("AUTH0_M2M_CLIENT_ID"), SecretRef: pulumi.String("auth0-m2m-client-id")},
		app.EnvironmentVarArgs{Name: pulumi.String("AUTH0_M2M_CLIENT_SECRET"), SecretRef: pulumi.String("auth0-m2m-client-secret")},
		app.EnvironmentVarArgs{Name: pulumi.String("ADMIN_API_KEY"), SecretRef: pulumi.String("admin-api-key")},
		app.EnvironmentVarArgs{Name: pulumi.String("SITE_BUILD_KEY"), SecretRef: pulumi.String("site-build-key")},
	}
	if env == "dev" {
		apiEnvVars = append(apiEnvVars,
			app.EnvironmentVarArgs{Name: pulumi.String("POSTGRES_HOST"), Value: shared.GetStringOutput(pulumi.String("postgresServerFqdn"))},
			app.EnvironmentVarArgs{Name: pulumi.String("POSTGRES_DB"), Value: pulumi.String("town_crier_dev")},
			app.EnvironmentVarArgs{Name: pulumi.String("POSTGRES_USER"), Value: pulumi.String("towncrier_api")},
			app.EnvironmentVarArgs{Name: pulumi.String("POSTGRES_SSLMODE"), Value: pulumi.String("require")},
			app.EnvironmentVarArgs{Name: pulumi.String("POSTGRES_AUTH"), Value: pulumi.String("azure-managed-identity")},
		)
	}

	// Container App (Go API), created for both dev and prod — the only environment stacks
	// (shared is handled separately). The placeholder quickstart image listens on 80, so the
	// first revision stays unhealthy until CD pushes the real town-crier-api-go image.
	configuration := &app.ConfigurationArgs{
		// Single in prod; Multiple in dev (pr-gate stages per-PR revisions).
		ActiveRevisionsMode: pulumi.String(string(app.ActiveRevisionsModeMultiple)),
		Ingress: &app.IngressArgs{
			External:      pulumi.Bool(true),
			TargetPort:    pulumi.Int(8080),
			Transport:     pulumi.String(string(app.IngressTransportMethodHttp)),
			CustomDomains: goApiCustomDomains,
			// Lock the *.azurecontainerapps.io origin to Cloudflare IPv4 ranges so it can
			// only be reached via the Cloudflare proxy, not bypassed directly (tc-0und).
			IpSecurityRestrictions: cloudflareIngressIPRestrictions(),
		},
		Registries: app.RegistryCredentialsArray{
			&app.RegistryCredentialsArgs{
				Server:   acrLoginServer,
				Identity: acrPullIdentityID,
			},
		},
		Secrets: secrets,
	}
	minReplicas := 0
	if env == "prod" {
		configuration.ActiveRevisionsMode = pulumi.String(string(app.ActiveRevisionsModeSingle))
		minReplicas = 1
	} else {
		// MaxInactiveRevisions only applies to Multiple mode (caps ACR storage growth
		// from staged PR revisions); omit it under Single.
		configuration.MaxInactiveRevisions = pulumi.Int(5)
	}

	_, err = app.NewContainerApp(ctx, fmt.Sprintf("ca-town-crier-api-go-%s", env), &app.ContainerAppArgs{
		ContainerAppName:     pulumi.String(fmt.Sprintf("ca-town-crier-api-go-%s", env)),
		ResourceGroupName:    resourceGroup.Name,
		ManagedEnvironmentId: containerAppsEnvironmentID,
		Configuration:        configuration,
		Identity: &app.ManagedServiceIdentityArgs{
			Type: pulumi.String(string(app.ManagedServiceIdentityTypeUserAssigned)),
			UserAssignedIdentities: pulumi.StringArray{
				acrPullIdentityID,
				cosmosDataIdentityID,
			},
		},
		Template: &app.TemplateArgs{
			Containers: app.ContainerArray{
				&app.ContainerArgs{
					Name:  pulumi.String("api-go"),
					Image: pulumi.String("mcr.microsoft.com/k8se/quickstart:latest"),
					Resources: &app.ContainerResourcesArgs{
						Cpu:    pulumi.Float64(containerCpu),
						Memory: pulumi.String(containerMemory),
					},
					Env: apiEnvVars,
				},
			},
			Scale: &app.ScaleArgs{
				// Keep one warm replica only for PROD to skip the ~15s ACA cold start.
				MinReplicas: pulumi.Int(minReplicas),
				MaxReplicas: pulumi.Int(1),
			},
		},
		Tags: tags,
	}, pulumi.IgnoreChanges([]string{"template.containers[0].image", "configuration.ingress.traffic"}))
	if err != nil {
		return err
	}

	// Service Bus — adaptive polling trigger (prod only). The worker identity gets Data
	// Owner RBAC on the namespace so it can send and receive without SAS keys.
	var pollingBus *serviceBusPollingInfra
	if env == "prod" {
		pollingBus, err = createServiceBusPollingInfra(ctx, env, resourceGroup.Name,
			shared.GetStringOutput(pulumi.String("cosmosDataIdentityPrincipalId")), tags)
		if err != nil {
			return err
		}
	}

	// Container Apps Jobs. In prod the "poll" job is event-triggered off the Service Bus
	// queue; a parallel cron "poll-bootstrap" re-seeds the queue if it is empty. Dev has no
	// poll job. See docs/adr/0024-service-bus-only-polling.md.
	if pollingBus != nil {
		if err = createWorkerJob(ctx, ec, "poll", "", 600, "poll-sb", pollingBus); err != nil {
			return err
		}
		if err = createWorkerJob(ctx, ec, "poll-bootstrap", "*/30 * * * *", 120, "poll-bootstrap", pollingBus); err != nil {
			return err
		}
	}

	if err = createWorkerJob(ctx, ec, "digest", "0 7 * * *", 600, "digest", nil); err != nil {
		return err
	}
	if err = createWorkerJob(ctx, ec, "digest-hourly", "0 * * * *", 300, "hourly-digest", nil); err != nil {
		return err
	}
	// Dormant account cleanup — daily at 03:30 UTC. Cascades UK GDPR Art.5(1)(e) erasure.
	if err = createWorkerJob(ctx, ec, "dormant-cleanup", "30 3 * * *", 600, "dormant-cleanup", nil); err != nil {
		return err
	}
	// Subscription sweep — daily at 04:30 UTC (offset an hour from dormant-cleanup so the two
	// full-scan jobs don't contend). Reverts lapsed offer-code/App Store paid tiers to Free in
	// Cosmos and syncs Auth0 metadata (ADR 0010 reconciliation; epic tc-rlja / GH #608).
	if err = createWorkerJob(ctx, ec, "subscription-sweep", "30 4 * * *", 600, "subscription-sweep", nil); err != nil {
		return err
	}

	// Static Web App (Landing Page)
	staticWebApp, err := web.NewStaticSite(ctx, fmt.Sprintf("swa-town-crier-%s", env), &web.StaticSiteArgs{
		Name:              pulumi.String(fmt.Sprintf("swa-town-crier-%s", env)),
		ResourceGroupName: resourceGroup.Name,
		Location:          pulumi.String("westeurope"),
		Sku: &web.SkuDescriptionArgs{
			Name: pulumi.String("Free"),
			Tier: pulumi.String("Free"),
		},
		BuildProperties: &web.StaticSiteBuildPropertiesArgs{
			AppLocation:    pulumi.String("/"),
			OutputLocation: pulumi.String(""),
		},
		Tags: tags,
	})
	if err != nil {
		return err
	}

	// Static Web App Custom Domain. Apex domains require TXT validation; subdomains use the
	// default CNAME delegation.
	isApexDomain := len(strings.Split(frontendDomain, ".")) == 2
	swaCustomDomainArgs := &web.StaticSiteCustomDomainArgs{
		Name:              staticWebApp.Name,
		DomainName:        pulumi.String(frontendDomain),
		ResourceGroupName: resourceGroup.Name,
	}
	if isApexDomain {
		swaCustomDomainArgs.ValidationMethod = pulumi.String("dns-txt-token")
	}
	_, err = web.NewStaticSiteCustomDomain(ctx, fmt.Sprintf("swa-domain-%s", env), swaCustomDomainArgs)
	if err != nil {
		return err
	}

	// SEO snapshot storage (epic tc-w5w9 / GH #598): a per-environment Storage Account +
	// seo-snapshot blob container, with the CI OIDC identity granted Storage Blob Data
	// Contributor (weekly seo-refresh writes the snapshot; every build reads it).
	seoSnapshotAccountName, seoSnapshotContainerName, err := createSeoSnapshotStorage(ctx, env, resourceGroup.Name, ciServicePrincipalID, tags)
	if err != nil {
		return err
	}

	ctx.Export("resourceGroupName", resourceGroup.Name)
	ctx.Export("cosmosAccountEndpoint", cosmosAccountEndpoint)
	ctx.Export("staticWebAppName", staticWebApp.Name)
	ctx.Export("seoSnapshotStorageAccountName", seoSnapshotAccountName)
	ctx.Export("seoSnapshotContainerName", seoSnapshotContainerName)

	return nil
}

// createWorkerJob creates a Container Apps Job for a background worker. cronExpression == ""
// + non-nil pollingBus produces an Event-triggered job; otherwise a Schedule-triggered cron
// job.
func createWorkerJob(ctx *pulumi.Context, ec envContext, nameSuffix, cronExpression string, replicaTimeout int, workerMode string, pollingBus *serviceBusPollingInfra) error {
	// Base env shared by every worker job.
	envVars := app.EnvironmentVarArray{
		app.EnvironmentVarArgs{Name: pulumi.String("OTEL_SERVICE_NAME"), Value: pulumi.String("town-crier-worker-go")},
		app.EnvironmentVarArgs{Name: pulumi.String("WORKER_MODE"), Value: pulumi.String(workerMode)},
		app.EnvironmentVarArgs{Name: pulumi.String("AZURE_CLIENT_ID"), Value: ec.cosmosDataIdentityClientID},
		app.EnvironmentVarArgs{Name: pulumi.String("APPLICATIONINSIGHTS_CONNECTION_STRING"), Value: ec.appInsightsConnectionString},
	}
	envVars = addGoWorkerEnv(envVars, ec, workerMode, pollingBus)

	useEventTrigger := cronExpression == ""
	if useEventTrigger && pollingBus == nil {
		return fmt.Errorf("event-triggered jobs require a serviceBusPollingInfra (queue + namespace)")
	}

	// The acs-connection-string and apns-auth-key secrets exist on every job; dormant-cleanup
	// and subscription-sweep also need the Auth0 Management (M2M) credentials.
	secrets := app.SecretArray{
		&app.SecretArgs{Name: pulumi.String("acs-connection-string"), Value: ec.acsConnectionString},
		&app.SecretArgs{Name: pulumi.String("apns-auth-key"), Value: ec.apnsAuthKey},
	}
	if workerMode == "dormant-cleanup" || workerMode == "subscription-sweep" {
		secrets = append(secrets,
			&app.SecretArgs{Name: pulumi.String("auth0-m2m-client-id"), Value: ec.auth0M2mClientID},
			&app.SecretArgs{Name: pulumi.String("auth0-m2m-client-secret"), Value: ec.auth0M2mClientSecret},
		)
	}

	configuration := &app.JobConfigurationArgs{
		ReplicaTimeout: pulumi.Int(replicaTimeout),
		Registries: app.RegistryCredentialsArray{
			&app.RegistryCredentialsArgs{
				Server:   ec.acrLoginServer,
				Identity: ec.acrPullIdentityID,
			},
		},
		Secrets: secrets,
	}

	if useEventTrigger {
		// KEDA azure-servicebus scaler — authenticates with the user-assigned managed
		// identity (no SAS key). The worker also has RBAC on the namespace via pollingBus.
		configuration.TriggerType = pulumi.String(string(app.TriggerTypeEvent))
		configuration.EventTriggerConfig = &app.JobConfigurationEventTriggerConfigArgs{
			Parallelism:            pulumi.Int(1),
			ReplicaCompletionCount: pulumi.Int(1),
			Scale: &app.JobScaleArgs{
				MinExecutions:   pulumi.Int(0),
				MaxExecutions:   pulumi.Int(1),
				PollingInterval: pulumi.Int(30),
				Rules: app.JobScaleRuleArray{
					&app.JobScaleRuleArgs{
						Name:     pulumi.String("servicebus-queue"),
						Type:     pulumi.String("azure-servicebus"),
						Identity: ec.cosmosDataIdentityID,
						Metadata: pulumi.StringMap{
							"namespace":    pollingBus.namespaceShortName,
							"queueName":    pollingBus.queueName,
							"messageCount": pulumi.String("1"),
						},
					},
				},
			},
		}
	} else {
		configuration.TriggerType = pulumi.String(string(app.TriggerTypeSchedule))
		configuration.ScheduleTriggerConfig = &app.JobConfigurationScheduleTriggerConfigArgs{
			CronExpression:         pulumi.String(cronExpression),
			Parallelism:            pulumi.Int(1),
			ReplicaCompletionCount: pulumi.Int(1),
		}
	}

	_, err := app.NewJob(ctx, fmt.Sprintf("job-tc-%s-%s", nameSuffix, ec.env), &app.JobArgs{
		JobName:           pulumi.String(fmt.Sprintf("job-tc-%s-%s", nameSuffix, ec.env)),
		ResourceGroupName: ec.resourceGroupName,
		EnvironmentId:     ec.environmentID,
		Configuration:     configuration,
		Identity: &app.ManagedServiceIdentityArgs{
			Type: pulumi.String(string(app.ManagedServiceIdentityTypeUserAssigned)),
			UserAssignedIdentities: pulumi.StringArray{
				ec.acrPullIdentityID,
				ec.cosmosDataIdentityID,
			},
		},
		Template: &app.JobTemplateArgs{
			Containers: app.ContainerArray{
				&app.ContainerArgs{
					Name:  pulumi.String("worker"),
					Image: pulumi.String("mcr.microsoft.com/k8se/quickstart:latest"),
					Resources: &app.ContainerResourcesArgs{
						Cpu:    pulumi.Float64(containerCpu),
						Memory: pulumi.String(containerMemory),
					},
					Env: envVars,
				},
			},
		},
		Tags: ec.tags,
	}, pulumi.IgnoreChanges([]string{"template.containers[0].image"}))
	return err
}

// addGoWorkerEnv appends the Go worker's env vars (SINGLE-underscore names). The consumer
// is api-go/internal/platform/config.go.
func addGoWorkerEnv(envVars app.EnvironmentVarArray, ec envContext, workerMode string, pollingBus *serviceBusPollingInfra) app.EnvironmentVarArray {
	// All modes: Go-named Cosmos endpoint/database.
	envVars = append(envVars,
		app.EnvironmentVarArgs{Name: pulumi.String("COSMOS_ENDPOINT"), Value: ec.cosmosAccountEndpoint},
		app.EnvironmentVarArgs{Name: pulumi.String("COSMOS_DATABASE"), Value: ec.cosmosDatabaseName},
	)

	// poll / poll-bootstrap: Service Bus namespace + queue.
	if pollingBus != nil {
		envVars = append(envVars,
			app.EnvironmentVarArgs{Name: pulumi.String("SERVICE_BUS_NAMESPACE"), Value: pollingBus.namespaceFqdn},
			app.EnvironmentVarArgs{Name: pulumi.String("SERVICE_BUS_QUEUE_NAME"), Value: pollingBus.queueName},
		)
	}

	// poll only: PlanIt client + polling-cycle budgets (defaults made explicit).
	if workerMode == "poll-sb" {
		envVars = append(envVars,
			app.EnvironmentVarArgs{Name: pulumi.String("PLANIT_BASE_URL"), Value: pulumi.String("https://www.planit.org.uk/")},
			app.EnvironmentVarArgs{Name: pulumi.String("PLANIT_THROTTLE_DELAY_SECONDS"), Value: pulumi.String("2")},
			app.EnvironmentVarArgs{Name: pulumi.String("PLANIT_RETRY_MAX_RETRIES"), Value: pulumi.String("3")},
			app.EnvironmentVarArgs{Name: pulumi.String("PLANIT_RETRY_INITIAL_BACKOFF_SECONDS"), Value: pulumi.String("1")},
			app.EnvironmentVarArgs{Name: pulumi.String("PLANIT_RETRY_RATE_LIMIT_BACKOFF_SECONDS"), Value: pulumi.String("5")},
			app.EnvironmentVarArgs{Name: pulumi.String("POLLING_MAX_PAGES_PER_AUTHORITY_PER_CYCLE"), Value: pulumi.String("3")},
			app.EnvironmentVarArgs{Name: pulumi.String("POLLING_HANDLER_BUDGET_SECONDS"), Value: pulumi.String("240")},
			app.EnvironmentVarArgs{Name: pulumi.String("POLL_REPLICA_TIMEOUT_SECONDS"), Value: pulumi.String("600")},
			app.EnvironmentVarArgs{Name: pulumi.String("POLL_SHUTDOWN_GRACE_SECONDS"), Value: pulumi.String("30")},
		)
	}

	// digest / hourly-digest: APNs push + ACS email.
	if workerMode == "digest" || workerMode == "hourly-digest" {
		envVars = append(envVars,
			app.EnvironmentVarArgs{Name: pulumi.String("APNS_ENABLED"), Value: pulumi.String("true")},
			app.EnvironmentVarArgs{Name: pulumi.String("APNS_AUTH_KEY"), SecretRef: pulumi.String("apns-auth-key")},
			app.EnvironmentVarArgs{Name: pulumi.String("APNS_KEY_ID"), Value: pulumi.String("L2J5PQASN5")},
			app.EnvironmentVarArgs{Name: pulumi.String("APNS_TEAM_ID"), Value: pulumi.String("4574VQ7N2X")},
			app.EnvironmentVarArgs{Name: pulumi.String("APNS_BUNDLE_ID"), Value: pulumi.String(apnsBundleID)},
			app.EnvironmentVarArgs{Name: pulumi.String("APNS_USE_SANDBOX"), Value: pulumi.String(ec.apnsUseSandbox)},
			app.EnvironmentVarArgs{Name: pulumi.String("ACS_CONNECTION_STRING"), SecretRef: pulumi.String("acs-connection-string")},
		)
	}

	// dormant-cleanup: Auth0 Management (M2M) for the Auth0 user delete in the cascade.
	// subscription-sweep: same Auth0 M2M creds to sync subscription_tier back to Free.
	if workerMode == "dormant-cleanup" || workerMode == "subscription-sweep" {
		envVars = append(envVars,
			app.EnvironmentVarArgs{Name: pulumi.String("AUTH0_DOMAIN"), Value: pulumi.String(ec.auth0Domain)},
			app.EnvironmentVarArgs{Name: pulumi.String("AUTH0_M2M_CLIENT_ID"), SecretRef: pulumi.String("auth0-m2m-client-id")},
			app.EnvironmentVarArgs{Name: pulumi.String("AUTH0_M2M_CLIENT_SECRET"), SecretRef: pulumi.String("auth0-m2m-client-secret")},
		)
	}

	return envVars
}

// createServiceBusPollingInfra provisions the Service Bus namespace + queue + RBAC used by
// the adaptive polling trigger.
func createServiceBusPollingInfra(ctx *pulumi.Context, env string, resourceGroupName pulumi.StringOutput, cosmosDataIdentityPrincipalID pulumi.StringOutput, tags pulumi.StringMap) (*serviceBusPollingInfra, error) {
	// Basic tier supports queues and scheduled messages — all the adaptive polling loop
	// needs. Location is pinned to uksouth (the RG metadata location is ukwest; see tc-ds1e).
	namespaceResource, err := servicebus.NewNamespace(ctx, fmt.Sprintf("sb-town-crier-%s", env), &servicebus.NamespaceArgs{
		NamespaceName:     pulumi.String(fmt.Sprintf("sb-town-crier-%s", env)),
		ResourceGroupName: resourceGroupName,
		Location:          pulumi.String("uksouth"),
		Sku: &servicebus.SBSkuArgs{
			Name: servicebus.SkuNameBasic,
			Tier: servicebus.SkuTierBasic,
		},
		Tags: tags,
	})
	if err != nil {
		return nil, err
	}

	// Polling trigger queue. LockDuration is capped at 5min by Azure across all tiers.
	// See ADR 0024 and the asb-lockduration-capped-at-5m memory.
	queue, err := servicebus.NewQueue(ctx, fmt.Sprintf("sbq-poll-%s", env), &servicebus.QueueArgs{
		QueueName:                        pulumi.String("poll"),
		NamespaceName:                    namespaceResource.Name,
		ResourceGroupName:                resourceGroupName,
		DefaultMessageTimeToLive:         pulumi.String("PT1H"),
		LockDuration:                     pulumi.String("PT5M"),
		MaxDeliveryCount:                 pulumi.Int(10),
		DeadLetteringOnMessageExpiration: pulumi.Bool(true),
	})
	if err != nil {
		return nil, err
	}

	// Built-in role: Azure Service Bus Data Owner — data-plane send/receive. Scoped to the
	// namespace.
	const serviceBusDataOwnerRoleID = "090c5cfd-751d-490a-894a-3ce6f1109419"
	subscriptionID := subscriptionFromID(namespaceResource.ID())
	_, err = authorization.NewRoleAssignment(ctx, fmt.Sprintf("sb-poll-data-owner-%s", env), &authorization.RoleAssignmentArgs{
		Scope: namespaceResource.ID(),
		RoleDefinitionId: pulumi.Sprintf(
			"/subscriptions/%s/providers/Microsoft.Authorization/roleDefinitions/%s", subscriptionID, serviceBusDataOwnerRoleID),
		PrincipalId:   cosmosDataIdentityPrincipalID,
		PrincipalType: pulumi.String(string(authorization.PrincipalTypeServicePrincipal)),
	})
	if err != nil {
		return nil, err
	}

	// Built-in role: Reader — management-plane GET so the bootstrap probe can read
	// countDetails. Scoped to the queue itself. See ADR 0024 + tc-ujl1.
	const readerRoleID = "acdd72a7-3385-48ef-bd42-f606fba81ae7"
	_, err = authorization.NewRoleAssignment(ctx, fmt.Sprintf("sb-poll-queue-reader-%s", env), &authorization.RoleAssignmentArgs{
		Scope: queue.ID(),
		RoleDefinitionId: pulumi.Sprintf(
			"/subscriptions/%s/providers/Microsoft.Authorization/roleDefinitions/%s", subscriptionID, readerRoleID),
		PrincipalId:   cosmosDataIdentityPrincipalID,
		PrincipalType: pulumi.String(string(authorization.PrincipalTypeServicePrincipal)),
	})
	if err != nil {
		return nil, err
	}

	fqdn := namespaceResource.Name.ApplyT(func(n string) string {
		return fmt.Sprintf("%s.servicebus.windows.net", n)
	}).(pulumi.StringOutput)

	return &serviceBusPollingInfra{
		namespaceShortName: namespaceResource.Name,
		namespaceFqdn:      fqdn,
		queueName:          queue.Name,
	}, nil
}

// createSeoSnapshotStorage provisions the per-environment Storage Account + seo-snapshot blob
// container that holds the weekly SEO prerender snapshot (seo-snapshot.json), and grants the CI
// OIDC identity Storage Blob Data Contributor so the weekly seo-refresh job can write it and
// every build can read it (epic tc-w5w9 / GH #598). Returns the account + container names so the
// caller can export them for the workflows to reference.
//
// This is the project's first Storage Account. It uses the smallest/cheapest profile:
// StorageV2, Standard_LRS, Hot. Shared-key access is disabled, so all data-plane access is
// AAD/RBAC only — CI authenticates via OIDC and must use `--auth-mode login` for blob I/O.
func createSeoSnapshotStorage(ctx *pulumi.Context, env string, resourceGroupName pulumi.StringOutput, ciServicePrincipalID string, tags pulumi.StringMap) (accountName, containerName pulumi.StringOutput, err error) {
	// Storage account names are 3-24 chars, lowercase alphanumeric, globally unique. "st" prefix
	// follows the resource-type naming convention; the hyphens from the usual "-town-crier-"
	// pattern are dropped because they are invalid in a storage account name.
	account, err := storage.NewStorageAccount(ctx, fmt.Sprintf("sttowncrier%s", env), &storage.StorageAccountArgs{
		AccountName:       pulumi.String(fmt.Sprintf("sttowncrier%s", env)),
		ResourceGroupName: resourceGroupName,
		Kind:              pulumi.String(string(storage.KindStorageV2)),
		Sku: &storage.SkuArgs{
			Name: pulumi.String(string(storage.SkuName_Standard_LRS)),
		},
		AccessTier:             storage.AccessTierHot,
		AllowBlobPublicAccess:  pulumi.Bool(false),
		AllowSharedKeyAccess:   pulumi.Bool(false),
		EnableHttpsTrafficOnly: pulumi.Bool(true),
		MinimumTlsVersion:      pulumi.String(string(storage.MinimumTlsVersion_TLS1_2)),
		Tags:                   tags,
	})
	if err != nil {
		return pulumi.StringOutput{}, pulumi.StringOutput{}, err
	}

	container, err := storage.NewBlobContainer(ctx, fmt.Sprintf("seo-snapshot-%s", env), &storage.BlobContainerArgs{
		AccountName:       account.Name,
		ResourceGroupName: resourceGroupName,
		ContainerName:     pulumi.String("seo-snapshot"),
		PublicAccess:      storage.PublicAccessNone,
	})
	if err != nil {
		return pulumi.StringOutput{}, pulumi.StringOutput{}, err
	}

	// Built-in role: Storage Blob Data Contributor — data-plane read+write of blobs. Scoped to
	// the account (it holds only this one container). PrincipalId is the CI service principal
	// (town-crier-github-actions) object ID.
	const storageBlobDataContributorRoleID = "ba92f5b4-2d11-453d-a403-e96b0029c9fe"
	subscriptionID := subscriptionFromID(account.ID())
	_, err = authorization.NewRoleAssignment(ctx, fmt.Sprintf("seo-snapshot-blob-contributor-%s", env), &authorization.RoleAssignmentArgs{
		Scope: account.ID(),
		RoleDefinitionId: pulumi.Sprintf(
			"/subscriptions/%s/providers/Microsoft.Authorization/roleDefinitions/%s", subscriptionID, storageBlobDataContributorRoleID),
		PrincipalId:   pulumi.String(ciServicePrincipalID),
		PrincipalType: pulumi.String(string(authorization.PrincipalTypeServicePrincipal)),
	})
	if err != nil {
		return pulumi.StringOutput{}, pulumi.StringOutput{}, err
	}

	return account.Name, container.Name, nil
}
