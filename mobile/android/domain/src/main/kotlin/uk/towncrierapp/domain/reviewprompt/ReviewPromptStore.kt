package uk.towncrierapp.domain.reviewprompt

/**
 * Persists the local [ReviewPromptState] for the review-prompt feature
 * (GH #628). The contract is a whole-state load/save: the store
 * reconstructs the full value on [load] and writes it back on [save]. All
 * storage is device-local — there is no server, network, or PII involved.
 * Port of iOS `ReviewPromptStore`.
 */
public interface ReviewPromptStore {
    /** Returns the persisted state, or a default-initialised state on first run. */
    public suspend fun load(): ReviewPromptState

    /** Persists the supplied state. */
    public suspend fun save(state: ReviewPromptState)
}
