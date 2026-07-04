plugins {
    alias(libs.plugins.android.library)
    alias(libs.plugins.kotlin.android)
    alias(libs.plugins.kotlin.compose)
}

android {
    namespace = "uk.towncrierapp.presentation"
    compileSdk = 35

    defaultConfig {
        minSdk = 26
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_21
        targetCompatibility = JavaVersion.VERSION_21
    }

    buildFeatures {
        compose = true
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
    // No explicitApi() here: explicit-API mode fights Compose ergonomics
    // (android-coding-standards skill, workflow-and-naming.md). `internal`
    // stays a review discipline in this module.
}

dependencies {
    // :domain ONLY — never :data. Enforced structurally (this module simply
    // never declares a :data dependency) and guarded by the root project's
    // verifyModuleGraph task, which walks this module's declared project
    // dependencies and fails the build if :data is reachable.
    implementation(project(":domain"))

    implementation(libs.kotlinx.coroutines.core)

    implementation(platform(libs.compose.bom))
    implementation(libs.compose.ui)
    implementation(libs.compose.ui.graphics)
    implementation(libs.compose.ui.tooling.preview)
    implementation(libs.compose.material3)
    implementation(libs.compose.material.icons.core)
    // The status-glyph set GH#775 needs (Schedule, Cancel, Undo, HelpOutline,
    // ...) isn't in core's small default subset.
    implementation(libs.compose.material.icons.extended)
    implementation(libs.androidx.lifecycle.viewmodel.compose)
    implementation(libs.androidx.lifecycle.runtime.compose)
    // Chrome Custom Tabs for the application-detail "View on Council Portal" button (GH#775).
    implementation(libs.androidx.browser)
    // rememberLauncherForActivityResult for the onboarding wizard's POST_NOTIFICATIONS
    // request (tc-7ttz) - needs a composable scope, so it lives in OnboardingRoute
    // rather than :app's wiring layer.
    implementation(libs.androidx.activity.compose)
    debugImplementation(libs.compose.ui.tooling)

    testImplementation(platform(libs.junit.bom))
    testImplementation(libs.junit.jupiter)
    testRuntimeOnly(libs.junit.platform.launcher)
    testImplementation(kotlin("test"))
    testImplementation(libs.kotlinx.coroutines.test)
    testImplementation(libs.turbine)
    // FakeAuthenticationService, anAuthSession(), ... shared via :domain's testFixtures.
    testImplementation(testFixtures(project(":domain")))
}
