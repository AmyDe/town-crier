package uk.towncrierapp.data.api

import kotlinx.serialization.Serializable

// Shared across the ApiClient*Test files — top-level `private` classes still
// occupy the package namespace, so duplicate per-file declarations of the
// same name genuinely collide; one shared internal DTO avoids that.
@Serializable
internal data class TestResponse(val id: String, val name: String)

@Serializable
internal data class TestBody(val title: String)
