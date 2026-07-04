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
    flavorDimensions += "environment"
    productFlavors {
        create("dev") {
            dimension = "environment"
            applicationIdSuffix = ".dev"
            buildConfigField("String", "API_BASE_URL", "\"https://api-dev.towncrierapp.uk\"")
        }
        create("prod") {
            dimension = "environment"
            buildConfigField("String", "API_BASE_URL", "\"https://api.towncrierapp.uk\"")
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

    testImplementation(platform(libs.junit.bom))
    testImplementation(libs.junit.jupiter)
    testRuntimeOnly(libs.junit.platform.launcher)
    testImplementation(kotlin("test"))
    testImplementation(libs.kotlinx.coroutines.test)
    testImplementation(libs.turbine)
    // FakeSubscriptionTierCache etc. shared via :domain's testFixtures.
    testImplementation(testFixtures(project(":domain")))
}
