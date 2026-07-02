# Workflow, Naming & Best Practices (reference)

Read when running lint/format/build commands, configuring SwiftLint or swift-format, or naming a type/protocol. The core (`SKILL.md`) carries the forbidden list; this file is the operational, naming, and best-practice detail.

## Workflow

### 1. Verification
To check the codebase for style and standards:

```bash
swiftlint lint --strict
swift test
```

### 2. Auto-Formatting
To automatically fix formatting issues:

```bash
swift-format format --in-place --recursive .
```

### 3. Setup Enforcements
To enforce standards in a project, use the bundled assets.

#### Apply .swiftlint.yml
Copy the standard `.swiftlint.yml` to the project root.

```bash
cp .claude/skills/ios-coding-standards/assets/.swiftlint.yml ./mobile/ios/
```

The bundled `.swiftlint.yml` treats force cast, force try, and force unwrap as errors.

## Guidelines

### Naming Conventions
- **Types:** PascalCase (structs, enums, classes, protocols).
- **Properties/Functions:** camelCase.
- **Protocols:**
    - Capabilities: `...able` (e.g., `Codable`, `Searchable`).
    - Services/Repositories: `...Service`, `...Repository` (e.g., `PlanningApplicationRepository`).
    - No `I` prefix (that is a C# convention). Use a `Protocol` suffix only if there is a genuine name collision with a concrete type.

### Best Practices
- **No Force Unwraps:** `!` is forbidden outside of `XCTest` assertions. Force unwraps crash the app at runtime and bypass Swift's safety guarantees.
- **Final Classes:** Classes should be `final` by default. Only remove `final` when inheritance is explicitly designed and documented.
- **Access Control:** `private` by default. Expose only what is strictly necessary — a smaller public surface means fewer accidental breaking changes and easier refactoring.
- **View Logic:** Views render State from the ViewModel and forward user intents. They should contain zero business logic (no `if/else` on domain rules). Conditional rendering based on ViewModel state (e.g., showing a loading spinner) is fine.
