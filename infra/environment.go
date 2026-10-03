package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/pulumi/pulumi-azure-native-sdk/app/v3"
	"github.com/pulumi/pulumi-azure-native-sdk/authorization/v3"
	"github.com/pulumi/pulumi-azure-native-sdk/dbforpostgresql/v3"
	"github.com/pulumi/pulumi-azure-native-sdk/resources/v3"
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
	// apiContainerPort is the port the Go API listens on (api-go/internal/platform/config.go
	// defaults PORT to "8080"). Shared by the ingress target port and the health probes below
	// so the two can't drift.
	apiContainerPort = 8080
	// apiHealthPath is the health-check route the liveness/readiness probes hit. It is
	// registered anonymously (no Auth0 bearer token required — see
	// api-go/internal/auth/middleware.go's anonymousPatterns) and does no work per request: a
	// pre-encoded static body with no dependency calls (api-go/internal/health/handler.go), so
	// probing it on every cycle costs nothing and can't itself fail from a downstream outage.
	apiHealthPath = "/health"
	// apiOriginCertName is the Cloudflare Origin CA certificate uploaded to the shared
	// Container Apps environment. Both api hosts share it, so it is not per-stack.
	apiOriginCertName = "cert-api-origin-ca"
	// planItBaseURL is the PlanIt origin the poll job reads from.
	planItBaseURL = "https://www.planit.org.uk/"
	// devPollingAreaID is the PlanIt area_id of Kingston upon Thames, the one authority the
	// dev poll job reads. Changing it resets the dev data set. Taken from authorityId in
	// web/src/data/towns.json, which is PlanIt's area_id.
	devPollingAreaID = "314"
	// devPollingFullReadMaxAgeDays is the dev poll job's POLLING_FULL_READ_MAX_AGE_DAYS during
	// the dev trial (0 forces a full read every run). Reset to prodPollingFullReadMaxAgeDays
	// when the trial ends.
	devPollingFullReadMaxAgeDays = "0"
	// prodPollingFullReadMaxAgeDays is the steady-state POLLING_FULL_READ_MAX_AGE_DAYS.
	prodPollingFullReadMaxAgeDays = "7"
	// PlanIt is one client across both envs, so the two daily caps must sum to at most 300.
	prodPollingDailyCallCap = "240"
	devPollingDailyCallCap  = "60"
	// The day allowance must stay below the cap, or the night never reads the coverage band.
	prodPollingDayAllowance = "60"
	devPollingDayAllowance  = "15"
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

// apiHealthProbes builds the liveness and readiness probes for the Go API's Container App
// (tc-97k35.2, GH #943 scope-out item). Both hit GET /health on apiContainerPort: it is
// registered anonymously (no bearer token) and returns a pre-encoded static body with no
// downstream calls, so it is safe and cheap to probe on every cycle and can't itself flap from
// a dependency outage. Timing/threshold fields are left unset so ACA applies its own defaults
// (10s period, 3 failure threshold, 1s timeout) — the endpoint does no work, so there is no
// latency profile here that would justify overriding them.
func apiHealthProbes() app.ContainerAppProbeArray {
	httpGet := &app.ContainerAppProbeHttpGetArgs{
		Path: pulumi.String(apiHealthPath),
		Port: pulumi.Int(apiContainerPort),
	}
	return app.ContainerAppProbeArray{
		&app.ContainerAppProbeArgs{
			Type:    pulumi.String(string(app.TypeLiveness)),
			HttpGet: httpGet,
		},
		&app.ContainerAppProbeArgs{
			Type:    pulumi.String(string(app.TypeReadiness)),
			HttpGet: httpGet,
		},
	}
}

// envContext holds the shared inputs every worker job and the container app need.
type envContext struct {
	env                         string
	resourceGroupName           pulumi.StringOutput
	environmentID               pulumi.StringOutput
	acrLoginServer              pulumi.StringOutput
	acrPullIdentityID           pulumi.StringOutput
	cosmosDataIdentityID        pulumi.StringOutput
	cosmosDataIdentityClientID  pulumi.StringOutput
	postgresServerFqdn          pulumi.StringOutput
	appInsightsConnectionString pulumi.StringOutput
	acsConnectionString         pulumi.StringOutput
	apnsAuthKey                 pulumi.StringOutput
	apnsUseSandbox              string
	fcmProjectID                string
	fcmServiceAccountJSON       pulumi.StringOutput
	auth0Domain                 string
	auth0M2mClientID            pulumi.StringOutput
	auth0M2mClientSecret        pulumi.StringOutput
	tags                        pulumi.StringMap
}

func runEnvironmentStack(ctx *pulumi.Context, conf *config.Config, env string, tags pulumi.StringMap) error {
	frontendDomain := conf.Require("frontendDomain")
	apiDomain := conf.Require("apiDomain")
	// Optional public share-page host (#738 Slice 3). When set, it is bound as a SECOND custom
	// domain on the existing API ACA app (managed cert, CNAME validation). Unset (empty) skips
	// the cert + binding entirely — prod is a later staged step, so only dev sets it for now.
	shareDomain := conf.Get("shareDomain")
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

	// FCM (Firebase Cloud Messaging) for Android push (#780). ServiceAccountJSON is the full
	// service-account key JSON (secret, mirrors how apnsAuthKey carries the .p8); ProjectID is
	// plain per-stack config. Both stacks target the same Firebase project — FCM has no sandbox
	// concept, so there is no dev/prod split here (unlike apnsUseSandbox).
	fcmProjectID := conf.Require("fcmProjectId")
	fcmServiceAccountJSON := conf.RequireSecret("fcmServiceAccountJson")

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
	resourceGroup, err := resources.NewResourceGroup(ctx, ResourceGroupName(env), &resources.ResourceGroupArgs{
		ResourceGroupName: pulumi.String(ResourceGroupName(env)),
		Tags:              tags,
	})
	if err != nil {
		return err
	}

	// An Azure managed certificate cannot issue or renew for the api host: the record is
	// proxied by Cloudflare, so the CNAME points at an intermediate value, and the ingress
	// admits Cloudflare ranges only, so DigiCert cannot fetch the validation token. Both api
	// managed certs silently failed renewal and expired. The origin serves a Cloudflare
	// Origin CA certificate instead, which Cloudflare trusts under Full (strict). It is
	// uploaded to the shared environment out of band (the private key is not in this repo)
	// and looked up here by name.
	var apiCert app.LookupCertificateResultOutput
	if customDomainPhase >= 2 {
		apiCert = app.LookupCertificateOutput(ctx, app.LookupCertificateOutputArgs{
			CertificateName:   pulumi.String(apiOriginCertName),
			EnvironmentName:   containerAppsEnvironmentName,
			ResourceGroupName: sharedResourceGroupName,
		})
	}

	// The Go app owns the api custom domain unconditionally. Phase >= 2 binds with
	// SniEnabled; phase 1 uses an empty list so Azure can validate the hostname first.
	goApiCustomDomains := app.CustomDomainArray{}
	if customDomainPhase >= 2 {
		goApiCustomDomains = app.CustomDomainArray{
			&app.CustomDomainArgs{
				Name:          pulumi.String(apiDomain),
				CertificateId: apiCert.Id(),
				BindingType:   app.BindingTypeSniEnabled,
			},
		}
	}

	// Public share page (#738 Slice 3): bind the share host as a SECOND custom domain on the same
	// API ACA app so one app serves JSON on api. and HTML on share.. The origin-lock
	// ipSecurityRestrictions on the shared ingress are inherited automatically.
	//
	// Azure requires a TWO-PHASE rollout for a brand-new managed cert (RequireCustomHostnameInEnvironment):
	//   phase 1 — register the hostname on the app with binding Disabled (no cert); validates domain
	//             ownership via the asuid CNAME/TXT (cut in Cloudflare beforehand) so Azure will mint a cert.
	//   phase 2 — create the managed cert (now permitted) and flip the binding to SniEnabled.
	// sharePhase gates this and defaults to 1 whenever shareDomain is set, so a plain shareDomain change
	// registers the hostname and an explicit sharePhase=2 later binds the cert. Unset shareDomain = skipped.
	sharePhase := 1
	if v, err := conf.TryInt("sharePhase"); err == nil {
		sharePhase = v
	}
	if shareDomain != "" {
		if sharePhase >= 2 {
			shareCert, err := app.NewManagedCertificate(ctx, fmt.Sprintf("cert-share-%s", env), &app.ManagedCertificateArgs{
				EnvironmentName:        containerAppsEnvironmentName,
				ManagedCertificateName: pulumi.String(fmt.Sprintf("cert-share-%s", env)),
				ResourceGroupName:      sharedResourceGroupName,
				Properties: &app.ManagedCertificatePropertiesArgs{
					SubjectName:             pulumi.String(shareDomain),
					DomainControlValidation: pulumi.String("CNAME"),
				},
			})
			if err != nil {
				return err
			}
			goApiCustomDomains = append(goApiCustomDomains, &app.CustomDomainArgs{
				Name:          pulumi.String(shareDomain),
				CertificateId: shareCert.ID(),
				BindingType:   app.BindingTypeSniEnabled,
			})
		} else {
			// Phase 1: register the hostname with the binding disabled so Azure will permit the
			// managed cert in phase 2. No CertificateId while disabled.
			goApiCustomDomains = append(goApiCustomDomains, &app.CustomDomainArgs{
				Name:        pulumi.String(shareDomain),
				BindingType: app.BindingTypeDisabled,
			})
		}
	}

	ec := envContext{
		env:                        env,
		resourceGroupName:          resourceGroup.Name,
		environmentID:              containerAppsEnvironmentID,
		acrLoginServer:             acrLoginServer,
		acrPullIdentityID:          acrPullIdentityID,
		cosmosDataIdentityID:       cosmosDataIdentityID,
		cosmosDataIdentityClientID: cosmosDataIdentityClientID,
		// Postgres FQDN threaded through so the worker-job path (createWorkerJob /
		// addGoWorkerEnv, which only receives ec) can build the prod Postgres connection
		// env. The API container reaches `shared` directly, so it doesn't need this.
		postgresServerFqdn:          shared.GetStringOutput(pulumi.String("postgresServerFqdn")),
		appInsightsConnectionString: appInsightsConnectionString,
		acsConnectionString:         acsConnectionString,
		apnsAuthKey:                 apnsAuthKey,
		apnsUseSandbox:              apnsUseSandbox,
		fcmProjectID:                fcmProjectID,
		fcmServiceAccountJSON:       fcmServiceAccountJSON,
		auth0Domain:                 auth0Domain,
		auth0M2mClientID:            auth0M2mClientID,
		auth0M2mClientSecret:        auth0M2mClientSecret,
		tags:                        tags,
	}

	// Dev-only: Postgres database for town_crier_dev on the shared Flexible Server.
	// Part of the Cosmos → Postgres + PostGIS migration (memo 0010, epic tc-hpd2 / GH #645).
	// The password-based connection-string secret is retired; the dev API authenticates via
	// Entra managed identity using a runtime token (no stored password). See GH #653.
	//
	// town_crier_prod is intentionally NOT declared here: it was provisioned data-plane
	// (CREATE DATABASE + role + goose schema; GH #664 Phase C, bead tc-hpd2.3). Bringing it
	// under Pulumi management (a prod NewDatabase + `pulumi import`) is deferred to the gated
	// flip release so this merge-only change can't try to (re)create the live prod DB.
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
		app.EnvironmentVarArgs{Name: pulumi.String("OTEL_SERVICE_NAME"), Value: pulumi.String(ImageRepoAPI)},
		// Read by the in-process Azure Monitor metrics exporter (tc-0rt1).
		app.EnvironmentVarArgs{Name: pulumi.String("APPLICATIONINSIGHTS_CONNECTION_STRING"), Value: appInsightsConnectionString},
		app.EnvironmentVarArgs{Name: pulumi.String("AZURE_CLIENT_ID"), Value: cosmosDataIdentityClientID},
		app.EnvironmentVarArgs{Name: pulumi.String("AUTH0_DOMAIN"), Value: pulumi.String(auth0Domain)},
		app.EnvironmentVarArgs{Name: pulumi.String("AUTH0_AUDIENCE"), Value: pulumi.String(auth0Audience)},
		app.EnvironmentVarArgs{Name: pulumi.String("CORS_ALLOWED_ORIGINS"), Value: pulumi.String(fmt.Sprintf("https://%s", frontendDomain))},
		app.EnvironmentVarArgs{Name: pulumi.String("AUTH0_M2M_CLIENT_ID"), SecretRef: pulumi.String("auth0-m2m-client-id"), Value: pulumi.String("")},
		app.EnvironmentVarArgs{Name: pulumi.String("AUTH0_M2M_CLIENT_SECRET"), SecretRef: pulumi.String("auth0-m2m-client-secret"), Value: pulumi.String("")},
		app.EnvironmentVarArgs{Name: pulumi.String("ADMIN_API_KEY"), SecretRef: pulumi.String("admin-api-key"), Value: pulumi.String("")},
		app.EnvironmentVarArgs{Name: pulumi.String("POLLING_ENABLED_DEFAULT"), Value: pulumi.String(pollingEnabledDefault(env))},
		app.EnvironmentVarArgs{Name: pulumi.String("SITE_BUILD_KEY"), SecretRef: pulumi.String("site-build-key"), Value: pulumi.String("")},
		// Blob endpoint for the share-cards container (#738 Slice 3): the share-page OG handler
		// caches baked map cards here. Computed directly from env because the account name is
		// deterministic (sttowncrier{env}); this avoids a cross-resource dependency on the storage
		// account, which is created later in this stack (after apiEnvVars is built). Always set for
		// both dev and prod (DNS-independent); empty-safe on the consumer side.
		app.EnvironmentVarArgs{Name: pulumi.String("SHARE_CARDS_BLOB_URL"), Value: pulumi.String(fmt.Sprintf("https://sttowncrier%s.blob.core.windows.net", env))},
	}
	// Both env stacks (dev + prod — the only environment stacks) run on Postgres
	// single-store (memo 0010 / GH #669 prod, GH #681 dev). The API authenticates to
	// town_crier_<env> via Entra managed identity (POSTGRES_AUTH); the connection vars are
	// explicit and identical across envs except POSTGRES_DB. env is exactly "dev"/"prod"
	// here, so "town_crier_"+env mirrors the addGoWorkerEnv idiom. AZURE_CLIENT_ID is already
	// set above and reused for the token fetch — no duplication.
	apiEnvVars = append(apiEnvVars,
		app.EnvironmentVarArgs{Name: pulumi.String("POSTGRES_HOST"), Value: shared.GetStringOutput(pulumi.String("postgresServerFqdn"))},
		app.EnvironmentVarArgs{Name: pulumi.String("POSTGRES_DB"), Value: pulumi.String("town_crier_" + env)},
		app.EnvironmentVarArgs{Name: pulumi.String("POSTGRES_USER"), Value: pulumi.String("towncrier_api")},
		app.EnvironmentVarArgs{Name: pulumi.String("POSTGRES_SSLMODE"), Value: pulumi.String("require")},
		app.EnvironmentVarArgs{Name: pulumi.String("POSTGRES_AUTH"), Value: pulumi.String("azure-managed-identity")},
	)
	if env == "dev" {
		// Accept Sandbox StoreKit transactions on dev: TestFlight builds route to the dev
		// API (api-dev) and TestFlight purchases carry environment="Sandbox". Without this,
		// APPLE_ENVIRONMENT falls back to the code default "Production" only and the verify
		// handler rejects every TestFlight purchase, so pro features never unlock (tc-81c9).
		// Prod deliberately stays Production-only (omits this var) so a sandbox tester cannot
		// self-grant a paid tier against production.
		apiEnvVars = append(apiEnvVars,
			app.EnvironmentVarArgs{Name: pulumi.String("APPLE_ENVIRONMENT"), Value: pulumi.String("Sandbox,Production")},
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
			TargetPort:    pulumi.Int(apiContainerPort),
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

	_, err = app.NewContainerApp(ctx, ContainerAppAPIName(env), &app.ContainerAppArgs{
		ContainerAppName:     pulumi.String(ContainerAppAPIName(env)),
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
					// Liveness + readiness against GET /health (tc-97k35.2). Without an explicit
					// Probes list ACA falls back to a bare TCP check, which only proves the port
					// is open, not that the process can serve a request. Applies to both env
					// stacks (this same function builds both dev and prod's API Container App) —
					// there's no reason to gate prod's traffic on real health but leave dev on TCP.
					Probes: apiHealthProbes(),
				},
			},
			Scale: &app.ScaleArgs{
				// Keep one warm replica only for PROD to skip the ~15s ACA cold start.
				MinReplicas: pulumi.Int(minReplicas),
				MaxReplicas: pulumi.Int(1),
			},
		},
		Tags: tags,
	}, pulumi.IgnoreChanges([]string{"template.containers[0].image", "configuration.ingress.traffic", "template.revisionSuffix"}))
	if err != nil {
		return err
	}

	// Hourly poll job: the run loop paces itself against the daily call budget (POLLING.md).
	if err = createWorkerJob(ctx, ec, "poll", "0 * * * *", 3600, "poll"); err != nil {
		return err
	}

	if err = createWorkerJob(ctx, ec, "digest", "0 7 * * *", 600, "digest"); err != nil {
		return err
	}
	if err = createWorkerJob(ctx, ec, "digest-hourly", "0 * * * *", 300, "hourly-digest"); err != nil {
		return err
	}
	// Dormant account cleanup — daily at 03:30 UTC. Cascades UK GDPR Art.5(1)(e) erasure.
	if err = createWorkerJob(ctx, ec, "dormant-cleanup", "30 3 * * *", 600, "dormant-cleanup"); err != nil {
		return err
	}
	// Subscription sweep — daily at 04:30 UTC (offset an hour from dormant-cleanup so the two
	// full-scan jobs don't contend). Reverts lapsed offer-code/App Store paid tiers to Free in
	// Cosmos and syncs Auth0 metadata (ADR 0010 reconciliation; epic tc-rlja / GH #608).
	if err = createWorkerJob(ctx, ec, "subscription-sweep", "30 4 * * *", 600, "subscription-sweep"); err != nil {
		return err
	}
	// pg-purge — daily at 02:00 UTC. Replaces the Cosmos per-document TTLs: runs
	// WORKER_MODE=pg-purge which calls Postgres PurgeOlderThan to enforce the 90-day
	// (Notifications) and 180-day (DeviceRegistrations) retention defaults (memo 0010 / GH #669).
	// Created for both envs: it enforces retention against town_crier_dev on dev, the same as
	// prod against town_crier_prod (GH #681).
	if err = createWorkerJob(ctx, ec, "pg-purge", "0 2 * * *", 600, "pg-purge"); err != nil {
		return err
	}

	// Static Web App (Landing Page)
	staticWebApp, err := web.NewStaticSite(ctx, StaticWebAppName(env), &web.StaticSiteArgs{
		Name:              pulumi.String(StaticWebAppName(env)),
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
	}, pulumi.IgnoreChanges([]string{"branch", "provider", "repositoryUrl"}))
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
	seoSnapshotAccountName, seoSnapshotContainerName, err := createSeoSnapshotStorage(ctx, env, resourceGroup.Name, ciServicePrincipalID,
		shared.GetStringOutput(pulumi.String("cosmosDataIdentityPrincipalId")), tags)
	if err != nil {
		return err
	}

	ctx.Export("resourceGroupName", resourceGroup.Name)
	ctx.Export("staticWebAppName", staticWebApp.Name)
	ctx.Export("seoSnapshotStorageAccountName", seoSnapshotAccountName)
	ctx.Export("seoSnapshotContainerName", seoSnapshotContainerName)

	return nil
}

// createWorkerJob creates a Schedule-triggered Container Apps Job for a background worker.
func createWorkerJob(ctx *pulumi.Context, ec envContext, nameSuffix, cronExpression string, replicaTimeout int, workerMode string) error {
	// Base env shared by every worker job.
	envVars := app.EnvironmentVarArray{
		app.EnvironmentVarArgs{Name: pulumi.String("OTEL_SERVICE_NAME"), Value: pulumi.String(ImageRepoWorker)},
		app.EnvironmentVarArgs{Name: pulumi.String("WORKER_MODE"), Value: pulumi.String(workerMode)},
		app.EnvironmentVarArgs{Name: pulumi.String("AZURE_CLIENT_ID"), Value: ec.cosmosDataIdentityClientID},
		app.EnvironmentVarArgs{Name: pulumi.String("APPLICATIONINSIGHTS_CONNECTION_STRING"), Value: ec.appInsightsConnectionString},
	}
	envVars = addGoWorkerEnv(envVars, ec, workerMode)

	// The acs-connection-string, apns-auth-key and fcm-service-account secrets exist on every
	// job; dormant-cleanup and subscription-sweep also need the Auth0 Management (M2M) credentials.
	secrets := app.SecretArray{
		&app.SecretArgs{Name: pulumi.String("acs-connection-string"), Value: ec.acsConnectionString},
		&app.SecretArgs{Name: pulumi.String("apns-auth-key"), Value: ec.apnsAuthKey},
		&app.SecretArgs{Name: pulumi.String("fcm-service-account"), Value: ec.fcmServiceAccountJSON},
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

	configuration.TriggerType = pulumi.String(string(app.TriggerTypeSchedule))
	configuration.ScheduleTriggerConfig = &app.JobConfigurationScheduleTriggerConfigArgs{
		CronExpression:         pulumi.String(cronExpression),
		Parallelism:            pulumi.Int(1),
		ReplicaCompletionCount: pulumi.Int(1),
	}

	jobIdentities := pulumi.StringArray{
		ec.acrPullIdentityID,
		ec.cosmosDataIdentityID,
	}

	_, err := app.NewJob(ctx, fmt.Sprintf("job-tc-%s-%s", nameSuffix, ec.env), &app.JobArgs{
		JobName:           pulumi.String(fmt.Sprintf("job-tc-%s-%s", nameSuffix, ec.env)),
		ResourceGroupName: ec.resourceGroupName,
		EnvironmentId:     ec.environmentID,
		Configuration:     configuration,
		Identity: &app.ManagedServiceIdentityArgs{
			Type:                   pulumi.String(string(app.ManagedServiceIdentityTypeUserAssigned)),
			UserAssignedIdentities: jobIdentities,
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
func addGoWorkerEnv(envVars app.EnvironmentVarArray, ec envContext, workerMode string) app.EnvironmentVarArray {
	// Both envs: every worker job runs on Postgres single-store (memo 0010 / GH #669 prod,
	// GH #681 dev). POSTGRES_DB is per-env (town_crier_dev / town_crier_prod) — ec.env is
	// exactly "dev"/"prod". AZURE_CLIENT_ID is already set on every worker job
	// (createWorkerJob) and reused for the Entra MI token fetch, so it is not duplicated here.
	envVars = append(envVars,
		app.EnvironmentVarArgs{Name: pulumi.String("POSTGRES_HOST"), Value: ec.postgresServerFqdn},
		app.EnvironmentVarArgs{Name: pulumi.String("POSTGRES_DB"), Value: pulumi.String("town_crier_" + ec.env)},
		app.EnvironmentVarArgs{Name: pulumi.String("POSTGRES_USER"), Value: pulumi.String("towncrier_api")},
		app.EnvironmentVarArgs{Name: pulumi.String("POSTGRES_SSLMODE"), Value: pulumi.String("require")},
		app.EnvironmentVarArgs{Name: pulumi.String("POSTGRES_AUTH"), Value: pulumi.String("azure-managed-identity")},
	)

	if workerMode == "poll" {
		envVars = append(envVars, pollJobEnv(ec.env)...)
	}

	// APNs push: poll sends the instant new-application / decision alerts
	// (notifydispatch fan-out, #456); digest / hourly-digest send the weekly
	// digest push. The apns-auth-key secret is on every worker job (see the
	// shared secrets above). Without these env vars buildPushSender falls back to
	// the NoOp sender and silently drops every push — which is exactly why the
	// poll worker delivered no instant pushes in prod (tc-wjbm).
	if workerMode == "poll" || workerMode == "digest" || workerMode == "hourly-digest" {
		envVars = append(envVars,
			app.EnvironmentVarArgs{Name: pulumi.String("APNS_ENABLED"), Value: pulumi.String("true")},
			app.EnvironmentVarArgs{Name: pulumi.String("APNS_AUTH_KEY"), SecretRef: pulumi.String("apns-auth-key"), Value: pulumi.String("")},
			app.EnvironmentVarArgs{Name: pulumi.String("APNS_KEY_ID"), Value: pulumi.String("L2J5PQASN5")},
			app.EnvironmentVarArgs{Name: pulumi.String("APNS_TEAM_ID"), Value: pulumi.String("4574VQ7N2X")},
			app.EnvironmentVarArgs{Name: pulumi.String("APNS_BUNDLE_ID"), Value: pulumi.String(apnsBundleID)},
			app.EnvironmentVarArgs{Name: pulumi.String("APNS_USE_SANDBOX"), Value: pulumi.String(ec.apnsUseSandbox)},
		)
	}

	// FCM push (#780): the same three jobs that send APNs also send FCM to Android devices.
	// FCM_SERVICE_ACCOUNT_JSON is the service-account key JSON (secret, on every job like
	// apns-auth-key); FCM_PROJECT_ID is plain per-stack config. Without these
	// buildPlatformDispatcher falls back to the FCM NoOp sender (APNs delivery unaffected).
	if workerMode == "poll" || workerMode == "digest" || workerMode == "hourly-digest" {
		envVars = append(envVars,
			app.EnvironmentVarArgs{Name: pulumi.String("FCM_ENABLED"), Value: pulumi.String("true")},
			app.EnvironmentVarArgs{Name: pulumi.String("FCM_PROJECT_ID"), Value: pulumi.String(ec.fcmProjectID)},
			app.EnvironmentVarArgs{Name: pulumi.String("FCM_SERVICE_ACCOUNT_JSON"), SecretRef: pulumi.String("fcm-service-account"), Value: pulumi.String("")},
		)
	}

	// The poll worker sends no email.
	if workerMode == "digest" || workerMode == "hourly-digest" {
		envVars = append(envVars,
			app.EnvironmentVarArgs{Name: pulumi.String("ACS_CONNECTION_STRING"), SecretRef: pulumi.String("acs-connection-string"), Value: pulumi.String("")},
		)
	}

	// dormant-cleanup: Auth0 Management (M2M) for the Auth0 user delete in the cascade.
	// subscription-sweep: same Auth0 M2M creds to sync subscription_tier back to Free.
	if workerMode == "dormant-cleanup" || workerMode == "subscription-sweep" {
		envVars = append(envVars,
			app.EnvironmentVarArgs{Name: pulumi.String("AUTH0_DOMAIN"), Value: pulumi.String(ec.auth0Domain)},
			app.EnvironmentVarArgs{Name: pulumi.String("AUTH0_M2M_CLIENT_ID"), SecretRef: pulumi.String("auth0-m2m-client-id"), Value: pulumi.String("")},
			app.EnvironmentVarArgs{Name: pulumi.String("AUTH0_M2M_CLIENT_SECRET"), SecretRef: pulumi.String("auth0-m2m-client-secret"), Value: pulumi.String("")},
		)
	}

	return envVars
}

// pollSettings returns the poll job's PlanIt, pacing and notifier settings for env. Dev differs
// from prod only by the single-authority filter, its share of the call budget, the oracle and
// the full-read age used during the dev trial.
func pollSettings(env string) map[string]string {
	dailyCap, dayAllowance, fullReadMaxAge := prodPollingDailyCallCap, prodPollingDayAllowance, prodPollingFullReadMaxAgeDays
	if env == "dev" {
		dailyCap, dayAllowance, fullReadMaxAge = devPollingDailyCallCap, devPollingDayAllowance, devPollingFullReadMaxAgeDays
	}
	vars := map[string]string{
		"PLANIT_BASE_URL":                     planItBaseURL,
		"POLLING_ENABLED_DEFAULT":             pollingEnabledDefault(env),
		"POLLING_DAILY_CALL_CAP":              dailyCap,
		"POLLING_MIN_REQUEST_SPACING_SECONDS": "60",
		"POLLING_DELTA_SLOTS":                 "09:00,12:00,15:00,17:00",
		"POLLING_DELTA_MAX_PAGES":             "20",
		"POLLING_DAY_ALLOWANCE":               dayAllowance,
		"POLLING_FULL_READ_MAX_AGE_DAYS":      fullReadMaxAge,
		"POLLING_RUN_BUDGET_MINUTES":          "55",
		"NOTIFY_QUIET_START":                  "22:00",
		"NOTIFY_QUIET_END":                    "07:00",
		"NOTIFY_EVENT_SURGE_THRESHOLD":        "10000",
	}
	if env == "dev" {
		vars["POLLING_AREA_ID"] = devPollingAreaID
		vars["POLLING_ORACLE_ENABLED"] = "true"
	}
	return vars
}

// pollingEnabledDefault is POLLING_ENABLED_DEFAULT, which applies until the
// polling switch is first set through /v1/admin/polling. The API and the poll
// job must agree, so both read it from here.
func pollingEnabledDefault(env string) string {
	if env == "prod" {
		return "true"
	}
	return "false"
}

func pollJobEnv(env string) app.EnvironmentVarArray {
	vars := pollSettings(env)
	names := make([]string, 0, len(vars))
	for name := range vars {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make(app.EnvironmentVarArray, 0, len(names))
	for _, name := range names {
		out = append(out, app.EnvironmentVarArgs{Name: pulumi.String(name), Value: pulumi.String(vars[name])})
	}
	return out
}

// createSeoSnapshotStorage provisions the per-environment Storage Account + seo-snapshot blob
// container that holds the weekly SEO prerender snapshot (seo-snapshot.json), and grants the CI
// OIDC identity Storage Blob Data Contributor so the weekly seo-refresh job can write it and
// every build can read it (epic tc-w5w9 / GH #598). It also provisions the share-cards container
// for the public share-page OG images and grants the API's user-assigned managed identity
// (cosmosDataIdentity) Storage Blob Data Contributor so it can cache-once baked cards (#738
// Slice 3). Returns the account + seo container names so the caller can export them for the
// workflows to reference.
//
// This is the project's first Storage Account. It uses the smallest/cheapest profile:
// StorageV2, Standard_LRS, Hot. Shared-key access is disabled, so all data-plane access is
// AAD/RBAC only — CI authenticates via OIDC and must use `--auth-mode login` for blob I/O.
func createSeoSnapshotStorage(ctx *pulumi.Context, env string, resourceGroupName pulumi.StringOutput, ciServicePrincipalID string, cosmosDataIdentityPrincipalID pulumi.StringOutput, tags pulumi.StringMap) (accountName, containerName pulumi.StringOutput, err error) {
	// Storage account names are 3-24 chars, lowercase alphanumeric, globally unique. "st" prefix
	// follows the resource-type naming convention; the hyphens from the usual "-town-crier-"
	// pattern are dropped because they are invalid in a storage account name.
	account, err := storage.NewStorageAccount(ctx, StorageAccountName(env), &storage.StorageAccountArgs{
		AccountName:       pulumi.String(StorageAccountName(env)),
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
		NetworkRuleSet: &storage.NetworkRuleSetArgs{
			Bypass:        pulumi.String("None"),
			DefaultAction: storage.DefaultActionAllow,
		},
		Tags: tags,
		// Azure's API is inconsistent about echoing networkRuleSet.ipRules/virtualNetworkRules
		// as [] vs omitting them entirely — dev and prod disagreed in opposite directions when
		// this was declared as an explicit empty array, so ignore both sub-paths rather than
		// chase a literal value that only matches one stack (tc-61eoz.8).
	}, pulumi.IgnoreChanges([]string{"networkRuleSet.ipRules", "networkRuleSet.virtualNetworkRules"}))
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

	// Second container on the same account: share-cards holds the cache-once baked OG map cards
	// for the public share page (#738 Slice 3). Same private-access profile as seo-snapshot.
	_, err = storage.NewBlobContainer(ctx, fmt.Sprintf("share-cards-%s", env), &storage.BlobContainerArgs{
		AccountName:       account.Name,
		ResourceGroupName: resourceGroupName,
		ContainerName:     pulumi.String("share-cards"),
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

	// Same built-in role for the API's user-assigned managed identity (cosmosDataIdentity, whose
	// client id is injected as AZURE_CLIENT_ID). The share-page OG handler runs as the API app and
	// needs data-plane read+write to cache-once baked cards into the share-cards container (#738
	// Slice 3). Scoped to the whole account, mirroring the CI grant above.
	_, err = authorization.NewRoleAssignment(ctx, fmt.Sprintf("share-cards-blob-contributor-%s", env), &authorization.RoleAssignmentArgs{
		Scope: account.ID(),
		RoleDefinitionId: pulumi.Sprintf(
			"/subscriptions/%s/providers/Microsoft.Authorization/roleDefinitions/%s", subscriptionID, storageBlobDataContributorRoleID),
		PrincipalId:   cosmosDataIdentityPrincipalID,
		PrincipalType: pulumi.String(string(authorization.PrincipalTypeServicePrincipal)),
	})
	if err != nil {
		return pulumi.StringOutput{}, pulumi.StringOutput{}, err
	}

	return account.Name, container.Name, nil
}
