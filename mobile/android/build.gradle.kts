import org.gradle.api.artifacts.ProjectDependency
import org.jlleitschuh.gradle.ktlint.reporter.ReporterType

plugins {
    alias(libs.plugins.android.application) apply false
    alias(libs.plugins.android.library) apply false
    alias(libs.plugins.kotlin.android) apply false
    alias(libs.plugins.kotlin.jvm) apply false
    alias(libs.plugins.kotlin.compose) apply false
    alias(libs.plugins.kotlin.serialization) apply false
    alias(libs.plugins.ktlint)
    alias(libs.plugins.detekt)
}

// ktlint + detekt are configured once here and applied to every module below,
// mirroring the Go skill's shared-config philosophy: style rules stay off,
// bug-catching rules stay on. See detekt.yml for the deliberate deviations.
subprojects {
    apply(plugin = "org.jlleitschuh.gradle.ktlint")
    apply(plugin = "io.gitlab.arturbosch.detekt")

    extensions.configure<org.jlleitschuh.gradle.ktlint.KtlintExtension> {
        version.set(rootProject.libs.versions.ktlintCli)
        android.set(true)
        outputToConsole.set(true)
        reporters {
            reporter(ReporterType.PLAIN)
        }
    }

    extensions.configure<io.gitlab.arturbosch.detekt.extensions.DetektExtension> {
        buildUponDefaultConfig = true
        config.setFrom(rootProject.file("detekt.yml"))
        source.setFrom(
            "src/main/kotlin",
            "src/test/kotlin",
        )
    }

    tasks.withType<io.gitlab.arturbosch.detekt.Detekt>().configureEach {
        reports {
            html.required.set(true)
            txt.required.set(false)
            sarif.required.set(false)
        }
    }
}

// Structural enforcement that :presentation never depends (directly or
// transitively) on :data — see android-coding-standards skill, module graph.
// This is a Gradle-level guard rather than a JUnit test because the thing
// under test is declared build metadata, not runtime behaviour; it is wired
// into the plain `test` task name below so `./gradlew test` catches drift.
tasks.register("verifyModuleGraph") {
    group = "verification"
    description = "Fails if :presentation depends, directly or transitively, on :data."

    doLast {
        fun declaredProjectDeps(path: String): Set<String> =
            project(path)
                .configurations
                .matching { it.name == "api" || it.name == "implementation" }
                .flatMap { it.dependencies.withType(ProjectDependency::class.java) }
                .map { it.path }
                .toSet()

        val visited = mutableSetOf<String>()
        val queue = ArrayDeque(listOf(":presentation"))
        val reachedData = mutableListOf<String>()

        while (queue.isNotEmpty()) {
            val path = queue.removeFirst()
            if (!visited.add(path)) continue
            if (path == ":data") {
                reachedData += path
                continue
            }
            queue.addAll(declaredProjectDeps(path))
        }

        check(reachedData.isEmpty()) {
            ":presentation must depend only on :domain — found a path reaching :data. " +
                "Remove the offending project() dependency."
        }
    }
}

// Root has no source of its own; declaring a plain "test" task here means
// `./gradlew test` (which Gradle runs across every project matching the
// task name) also runs this guard alongside every module's real unit tests.
tasks.register("test") {
    group = "verification"
    dependsOn("verifyModuleGraph")
}
