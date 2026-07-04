plugins {
    alias(libs.plugins.android.application)
    alias(libs.plugins.kotlin.android)
    alias(libs.plugins.kotlin.compose)
    alias(libs.plugins.kotlin.serialization)
}

android {
    namespace = "uk.towncrierapp.mobile"
    compileSdk = 35

    defaultConfig {
        applicationId = "uk.towncrierapp.mobile"
        minSdk = 26
        targetSdk = 35
        versionCode = 1
        versionName = "1.0.0"

        // Auth0 Android SDK manifest placeholders (tc-f2il, epic #770 D4).
        // Same tenant + domain for both flavors — only the applicationId
        // suffix differs, which the SDK reads automatically to build the
        // per-flavor callback path (.../android/{applicationId}/callback).
        manifestPlaceholders["auth0Domain"] = "towncrierapp.uk.auth0.com"
        manifestPlaceholders["auth0Scheme"] = "https"
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_21
        targetCompatibility = JavaVersion.VERSION_21
    }

    buildFeatures {
        compose = true
        buildConfig = true
    }

    // API_BASE_URL via buildConfigField is the ONLY environment mechanism
    // (epic #770, decision D5) — no .env files, no per-flavor secrets.
    //
    // AUTH0_AUDIENCE is normally identical to API_BASE_URL (the flavor talks
    // to its own API and requests a matching-audience token), but the `local`
    // flavor below decouples them: it talks to a local API over cleartext
    // HTTP but still requests a *dev*-audience token, because the local API
    // is configured to validate that audience (tc-z95t).
    flavorDimensions += "environment"
    productFlavors {
        create("dev") {
            dimension = "environment"
            applicationIdSuffix = ".dev"
            buildConfigField("String", "API_BASE_URL", "\"https://api-dev.towncrierapp.uk\"")
            buildConfigField("String", "AUTH0_AUDIENCE", "\"https://api-dev.towncrierapp.uk\"")
        }
        create("prod") {
            dimension = "environment"
            buildConfigField("String", "API_BASE_URL", "\"https://api.towncrierapp.uk\"")
            buildConfigField("String", "AUTH0_AUDIENCE", "\"https://api.towncrierapp.uk\"")
        }
        // Points at a locally-running containerised Go API (docker-compose,
        // not this Gradle build) via 10.0.2.2 — the Android emulator's alias
        // for the host machine's localhost. Debug-only in practice (no
        // release signing config targets it); see
        // src/local/res/xml/network_security_config.xml for the narrowly-
        // scoped cleartext exception this flavor alone needs.
        //
        // applicationIdSuffix deliberately REUSES ".dev" rather than adding a
        // new ".local" suffix: the Auth0 Native application's allowed
        // callback URLs are keyed off applicationId
        // (.../android/{applicationId}/callback), and only the ".dev"
        // package is registered there. Reusing it means Universal Login
        // works out of the box with zero Auth0 tenant reconfiguration; the
        // trade-off is that installing `local` replaces a `dev` install (and
        // vice versa) on the same device, same as any other same-applicationId
        // variant swap — expected, not a bug (see the emulator walkthrough's
        // full-uninstall step).
        create("local") {
            dimension = "environment"
            applicationIdSuffix = ".dev"
            buildConfigField("String", "API_BASE_URL", "\"http://10.0.2.2:8080\"")
            buildConfigField("String", "AUTH0_AUDIENCE", "\"https://api-dev.towncrierapp.uk\"")
        }
    }

    buildTypes {
        release {
            isMinifyEnabled = false
        }
    }

    testOptions {
        unitTests.all { it.useJUnitPlatform() }
        unitTests.isIncludeAndroidResources = false
    }
}

kotlin {
    jvmToolchain(
        libs.versions.javaToolchain
            .get()
            .toInt(),
    )
}

dependencies {
    implementation(project(":domain"))
    implementation(project(":data"))
    implementation(project(":presentation"))

    implementation(libs.androidx.core.ktx)
    implementation(libs.androidx.lifecycle.runtime.ktx)
    implementation(libs.androidx.lifecycle.runtime.compose)
    implementation(libs.androidx.lifecycle.viewmodel.compose)
    implementation(libs.androidx.activity.compose)
    implementation(libs.androidx.navigation.compose)

    implementation(platform(libs.compose.bom))
    implementation(libs.compose.ui)
    implementation(libs.compose.ui.graphics)
    implementation(libs.compose.ui.tooling.preview)
    implementation(libs.compose.material3)
    // Bottom-navigation icons (tc-z95t): Home/Place are in the core icon set.
    implementation(libs.compose.material.icons.core)
    debugImplementation(libs.compose.ui.tooling)

    implementation(libs.kotlinx.coroutines.android)
    // Type-safe Navigation Compose routes (@Serializable destinations).
    implementation(libs.kotlinx.serialization.json)

    // Auth0 SDK, OkHttp transport, DataStore latch — the composition root is
    // the only module that constructs the Android-touching leaves these need
    // (SecureCredentialsManager, DataStore<Preferences>) (tc-f2il).
    implementation(libs.auth0)
    implementation(libs.okhttp)
    implementation(libs.androidx.datastore.preferences)
    // Play In-App Review ("Rate the App", tc-4jjw / #778) — needs a real
    // Activity, so it's wired here rather than in `:presentation`.
    implementation(libs.play.review.ktx)
    // The Settings gear glyph isn't in material-icons-core's small default subset.
    implementation(libs.compose.material.icons.extended)

    testImplementation(platform(libs.junit.bom))
    testImplementation(libs.junit.jupiter)
    testRuntimeOnly(libs.junit.platform.launcher)
    testImplementation(kotlin("test"))
    testImplementation(libs.kotlinx.coroutines.test)
    testImplementation(libs.turbine)
    // FakeSubscriptionTierCache etc. shared via :domain's testFixtures.
    testImplementation(testFixtures(project(":domain")))
}
