package main

import (
	"fmt"
	"strings"

	"github.com/pulumi/pulumi-azure-native-sdk/app/v3"
	"github.com/pulumi/pulumi-azure-native-sdk/authorization/v3"
	"github.com/pulumi/pulumi-azure-native-sdk/cosmosdb/v3"
	"github.com/pulumi/pulumi-azure-native-sdk/resources/v3"
	"github.com/pulumi/pulumi-azure-native-sdk/servicebus/v3"
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
	auth0Audience := conf.Require("auth0Audience")
	customDomainPhase := 2
	if v, err := conf.TryInt("customDomainPhase"); err == nil {
		customDomainPhase = v
	}
	adminAPIKey := conf.RequireSecret("adminApiKey")
	autoGrantProDomains := conf.RequireSecret("autoGrantProDomains")
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
					&cosmosdb.IncludedPathArgs{Path: pulumi.String("/status/?")},
					&cosmosdb.IncludedPathArgs{Path: pulumi.String("/applicationType/?")},
					&cosmosdb.IncludedPathArgs{Path: pulumi.String("/decisionDate/?")},
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
		// WatchZones — partitioned by userId, unique on (userId, name)
		{name: "WatchZones", partitionKeyPath: "/userId", uniqueKeyPaths: [][]string{{"/userId", "/name"}}},
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

	// Container App (Go API) — created for BOTH dev and prod. The placeholder quickstart
	// image listens on 80, so the first revision stays unhealthy until CD pushes the real
	// town-crier-api-go image.
	if env == "dev" || env == "prod" {
		configuration := &app.ConfigurationArgs{
			// Single in prod; Multiple in dev (pr-gate stages per-PR revisions).
			ActiveRevisionsMode: pulumi.String(string(app.ActiveRevisionsModeMultiple)),
			Ingress: &app.IngressArgs{
				External:      pulumi.Bool(true),
				TargetPort:    pulumi.Int(8080),
				Transport:     pulumi.String(string(app.IngressTransportMethodHttp)),
				CustomDomains: goApiCustomDomains,
			},
			Registries: app.RegistryCredentialsArray{
				&app.RegistryCredentialsArgs{
					Server:   acrLoginServer,
					Identity: acrPullIdentityID,
				},
			},
			Secrets: app.SecretArray{
				&app.SecretArgs{Name: pulumi.String("auth0-m2m-client-id"), Value: auth0M2mClientID},
				&app.SecretArgs{Name: pulumi.String("auth0-m2m-client-secret"), Value: auth0M2mClientSecret},
				&app.SecretArgs{Name: pulumi.String("auto-grant-pro-domains"), Value: autoGrantProDomains},
				// Same admin key as the .NET app so the Go X-Admin-Key gate accepts
				// identical requests (tc-52t6).
				&app.SecretArgs{Name: pulumi.String("admin-api-key"), Value: adminAPIKey},
			},
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
						Env: app.EnvironmentVarArray{
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
							app.EnvironmentVarArgs{Name: pulumi.String("SUBSCRIPTION_AUTOGRANT_PRODOMAINS"), SecretRef: pulumi.String("auto-grant-pro-domains")},
							app.EnvironmentVarArgs{Name: pulumi.String("ADMIN_API_KEY"), SecretRef: pulumi.String("admin-api-key")},
						},
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

	ctx.Export("resourceGroupName", resourceGroup.Name)
	ctx.Export("cosmosAccountEndpoint", cosmosAccountEndpoint)
	ctx.Export("staticWebAppName", staticWebApp.Name)

	return nil
}

// createWorkerJob creates a Container Apps Job for a background worker. cronExpression == ""
// + non-nil pollingBus produces an Event-triggered job; otherwise a Schedule-triggered cron
// job.
func createWorkerJob(ctx *pulumi.Context, ec envContext, nameSuffix, cronExpression string, replicaTimeout int, workerMode string, pollingBus *serviceBusPollingInfra) error {
	// Base env. WORKER_MODE is inserted after OTEL_SERVICE_NAME (matches the C# List.Insert(1)).
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
	// also needs the Auth0 Management (M2M) credentials.
	secrets := app.SecretArray{
		&app.SecretArgs{Name: pulumi.String("acs-connection-string"), Value: ec.acsConnectionString},
		&app.SecretArgs{Name: pulumi.String("apns-auth-key"), Value: ec.apnsAuthKey},
	}
	if workerMode == "dormant-cleanup" {
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

// addGoWorkerEnv appends the Go worker's env vars (SINGLE-underscore names) in the same
// order as the original C# AddGoWorkerEnv. The consumer is api-go/internal/platform/config.go.
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

	// poll only: PlanIt client + polling-cycle budgets (the .NET defaults made explicit).
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
	if workerMode == "dormant-cleanup" {
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
