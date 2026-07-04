plugins {
    alias(libs.plugins.kotlin.jvm)
    `java-test-fixtures`
}

kotlin {
    explicitApi()
    jvmToolchain(
        libs.versions.javaToolchain
            .get()
            .toInt(),
    )
}

dependencies {
    // Language-level concurrency vocabulary only — no android.*, no HTTP, no
    // serialization. See android-coding-standards skill: architecture-and-modules.md.
    implementation(libs.kotlinx.coroutines.core)

    testImplementation(platform(libs.junit.bom))
    testImplementation(libs.junit.jupiter)
    testRuntimeOnly(libs.junit.platform.launcher)
    testImplementation(kotlin("test"))
    testImplementation(libs.kotlinx.coroutines.test)
    testImplementation(libs.turbine)

    // Shared fakes/fixtures for domain ports (FakeAuthenticationService, aAuthSession(), ...)
    // consumed by both :data and :presentation tests — testing.md's documented
    // cross-module fake-sharing mechanism (java-test-fixtures, not a "testutils" module).
    testFixturesImplementation(kotlin("test"))
}

tasks.test {
    useJUnitPlatform()
}
