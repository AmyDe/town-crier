plugins {
    alias(libs.plugins.android.library)
    alias(libs.plugins.kotlin.android)
    alias(libs.plugins.kotlin.serialization)
}

android {
    namespace = "uk.towncrierapp.data"
    compileSdk = 35

    defaultConfig {
        minSdk = 26
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_21
        targetCompatibility = JavaVersion.VERSION_21
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
    explicitApi()
}

dependencies {
    implementation(project(":domain"))

    implementation(libs.kotlinx.coroutines.core)
    implementation(libs.kotlinx.serialization.json)
    implementation(libs.okhttp)
    implementation(libs.auth0)
    implementation(libs.androidx.datastore.preferences)

    testImplementation(platform(libs.junit.bom))
    testImplementation(libs.junit.jupiter)
    testRuntimeOnly(libs.junit.platform.launcher)
    testImplementation(kotlin("test"))
    testImplementation(libs.kotlinx.coroutines.test)
    testImplementation(libs.turbine)
    // FakeAuthenticationService, anAuthSession(), ... shared via :domain's testFixtures.
    testImplementation(testFixtures(project(":domain")))
}
