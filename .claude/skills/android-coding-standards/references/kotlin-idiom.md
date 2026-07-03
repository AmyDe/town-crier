# Kotlin Idiom (reference)

Read when unsure of language-level style. The test for every construct: would this look at home in a kotlinx library? Kotlin read by a Kotlin engineer should feel expression-oriented, immutable-first, and honest about absence and failure — without being clever for its own sake.

## Immutability first

- `val` everywhere; a `var` is a design statement that something genuinely changes, and it should be `private` when it does.
- Public APIs speak read-only types: `List`, `Map`, `Set` — never `MutableList` in a signature. Inside a function, `buildList { }`/`buildMap { }` beat declare-then-mutate.
- Evolve values with `data class` + `copy`, not setters: `zone.copy(name = trimmed)`.

## Null-safety, honestly

- `T?` is the model for absence — use it, and let the compiler force handling via `?.`, `?:`, and smart casts.
- `!!` is banned in production code. If you "know" it's non-null, encode that knowledge: `requireNotNull(x) { "why it can't be null" }` at the boundary where the guarantee is established, then pass the non-null type onwards.
- `lateinit var` only where a framework imposes two-phase init (rare in this codebase — Compose + constructor injection removes almost every case).
- Java interop (OkHttp, Android SDK) returns platform types — pin them to an explicit Kotlin type at the boundary so nullability is decided once, deliberately.

## Sealed hierarchies and exhaustive `when`

Closed sets are sealed interfaces (or enums when there's no payload), consumed with an exhaustive `when` and **no `else` branch** — adding a case must break every consumer at compile time:

```kotlin
sealed interface RedeemOutcome {
    data class Redeemed(val tier: Tier, val expiresAt: Instant) : RedeemOutcome
    data object AlreadyRedeemed : RedeemOutcome
    data object Expired : RedeemOutcome
    data class Rejected(val reason: String) : RedeemOutcome
}

// Presentation layer (user-facing copy is its job — see compose-ui.md on UiText/resources):
val message: UiText = when (outcome) {
    is RedeemOutcome.Redeemed -> UiText.Res(R.string.redeem_success, outcome.tier.displayName)
    RedeemOutcome.AlreadyRedeemed -> UiText.Res(R.string.redeem_already_used)
    RedeemOutcome.Expired -> UiText.Res(R.string.redeem_expired)
    is RedeemOutcome.Rejected -> UiText.Dynamic(outcome.reason)
}
```

Note `when` used as an expression — prefer that over statement-plus-mutation.

## Expression-oriented style

- A cheap, pure, non-throwing, parameterless computation is a **property**, not a function: `val displayName get() = "$name (${radius.metres} m)"`. A parameterless `fun displayName()` is a Java/C# habit — reserve functions for work (side effects, I/O, non-trivial cost, can fail).
- Single-expression syntax (`fun cappedTo(tier: Tier) = copy(radius = …)`) when the body is genuinely one expression that fits on a line or two — don't contort a multi-step body to earn the `=`.
- `if`/`when`/`try` are expressions; use them to initialise a `val` instead of declaring-then-assigning.
- Prefer stdlib collection transforms (`map`, `filter`, `firstOrNull`, `groupBy`, `partition`, `sumOf`) while the chain still reads top-to-bottom as a sentence. The moment a chain needs a comment to explain a step, name the step with a local `val` or drop to a plain loop. Reach for `asSequence()` only on large inputs or long chains — it's an optimisation, not a style.

## Scope functions — sparingly, one deep

| Function | The one job it's for |
|---|---|
| `let` | transform a nullable: `id?.let(::findZone)` |
| `apply` | configure an object you're constructing: `Json { … }`, builders from Java APIs |
| `also` | side-effect in a chain (logging) without breaking it |
| `run`/`with` | grouping several calls on one receiver — fine, same one-deep rule |

Never nest scope functions; never chain two on the same line. If `it` becomes ambiguous or you're renaming lambda parameters to keep track, use a local `val`. Scope-function soup is the most common way generated Kotlin outs itself as non-native.

## Named and default arguments

Default arguments + named call sites replace both the builder pattern and telescoping overload ladders:

```kotlin
fun paged(cursor: Cursor? = null, pageSize: PageSize = PageSize.DEFAULT): Page
// call site: paged(pageSize = PageSize(50))
```

When a call passes two or more literals of the same type, name them. One public function with defaults beats three overloads.

## Extension functions

- Use them to give types you don't own a domain vocabulary (`Instant.toDisplayDate()`), or to keep a helper next to its single consumer as a `private` extension.
- They live in the feature package of their consumer — **never** in a `utils`/`Extensions.kt` dumping ground. A public extension on a stdlib/platform type is API: it needs the same justification as any public function.
- Kotlin has top-level functions and properties; there is no reason for a class whose name ends in `Util`, `Helper`, or `Manager` to exist.

## Errors: two channels, never blurred

1. **Bugs and broken infrastructure → exceptions.** `require`/`check` for contract violations; typed exceptions (e.g. a sealed `ApiException` hierarchy) thrown by the data layer for transport-level failure. Catch them **specifically**, at the layer that can act (usually the ViewModel), and always let `CancellationException` fly — see `coroutines-and-flow.md`.
2. **Expected domain outcomes → sealed results** returned, not thrown (`RedeemOutcome`, `PostcodeLookup`). The caller is forced by `when`-exhaustiveness to handle every case; nothing is handleable-but-forgettable.

Do not use `kotlin.Result` in public signatures and do not wrap suspend calls in `runCatching` — both erase the error type, and `runCatching` also swallows coroutine cancellation. Model the outcome or throw the typed exception; never return `null` to mean "it failed".

## Objects, companions, constants

- Constants: `const val` at top level or in a `companion object` — near their user, UPPER_SNAKE_CASE.
- `object` is for genuinely stateless singletons (a parser, a `Comparator`); an `object` holding mutable state is a hidden global — inject a class instead.
- Prefer top-level factory functions to companion factories unless the factory needs private access to the constructor (the smart-constructor pattern).
