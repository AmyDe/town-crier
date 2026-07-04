package uk.towncrierapp.data.legal

/**
 * Reads a bundled asset's raw text by path (e.g. `"legal/privacy.json"`).
 * The testable seam over `AssetManager.open` — no Android framework or
 * Robolectric needed to fake it (android-coding-standards skill: no
 * Robolectric). The real implementation wraps `Context.getAssets()` and is
 * constructed at `:app`'s composition root.
 */
public fun interface LegalDocumentAssetReader {
    public fun read(assetPath: String): String
}
