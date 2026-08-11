package uk.towncrierapp.domain.watchzones

import org.junit.jupiter.api.Test
import kotlin.math.PI
import kotlin.math.abs
import kotlin.math.cos
import kotlin.math.sin
import kotlin.test.assertEquals
import kotlin.test.assertIs
import kotlin.test.assertTrue

/**
 * Mirrors `api-go/internal/watchzones/zone.go`'s `NewBoundary`/`Boundary`
 * validation and geometry, and iOS `WatchZoneBoundaryTests` (GH#1072 Phase 1,
 * tc-v6fo0.1).
 */
class WatchZoneBoundaryTest {
    // MARK: - Closure

    @Test
    fun `an open ring is auto-closed by appending a copy of the first vertex`() {
        val triangle =
            listOf(
                Coordinate(latitude = 51.50, longitude = -0.10),
                Coordinate(latitude = 51.51, longitude = -0.09),
                Coordinate(latitude = 51.50, longitude = -0.09),
            )

        val result = assertIs<WatchZoneBoundaryResult.Valid>(WatchZoneBoundary.of(triangle))

        assertEquals(triangle.size + 1, result.boundary.vertices.size)
        assertEquals(result.boundary.vertices.first(), result.boundary.vertices.last())
    }

    @Test
    fun `an already-closed ring is not doubled`() {
        val closed =
            listOf(
                Coordinate(latitude = 51.50, longitude = -0.10),
                Coordinate(latitude = 51.51, longitude = -0.09),
                Coordinate(latitude = 51.50, longitude = -0.09),
                Coordinate(latitude = 51.50, longitude = -0.10),
            )

        val result = assertIs<WatchZoneBoundaryResult.Valid>(WatchZoneBoundary.of(closed))

        assertEquals(closed.size, result.boundary.vertices.size)
    }

    // MARK: - Vertex count

    @Test
    fun `fewer than 3 distinct vertices is rejected`() {
        val vertices =
            listOf(
                Coordinate(latitude = 51.50, longitude = -0.10),
                Coordinate(latitude = 51.51, longitude = -0.11),
            )

        assertEquals(WatchZoneBoundaryResult.InvalidVertexCount, WatchZoneBoundary.of(vertices))
    }

    @Test
    fun `more than 50 distinct vertices is rejected`() {
        val vertices =
            (0 until 51).map { index ->
                Coordinate(latitude = 51.5, longitude = -0.5 + index * 0.001)
            }

        assertEquals(WatchZoneBoundaryResult.InvalidVertexCount, WatchZoneBoundary.of(vertices))
    }

    @Test
    fun `exactly 50 distinct vertices is accepted`() {
        val centreLat = 51.5074
        val centreLon = -0.1278
        val radiusDeg = 0.05
        val count = 50
        val vertices =
            (0 until count).map { index ->
                val angle = 2 * PI * index / count
                Coordinate(
                    latitude = centreLat + radiusDeg * sin(angle),
                    longitude = centreLon + radiusDeg * cos(angle),
                )
            }

        val result = assertIs<WatchZoneBoundaryResult.Valid>(WatchZoneBoundary.of(vertices))

        assertEquals(count + 1, result.boundary.vertices.size)
    }

    @Test
    fun `a duplicate interior vertex is rejected`() {
        val vertices =
            listOf(
                Coordinate(latitude = 51.50, longitude = -0.10),
                Coordinate(latitude = 51.51, longitude = -0.09),
                Coordinate(latitude = 51.51, longitude = -0.09),
                Coordinate(latitude = 51.50, longitude = -0.08),
            )

        assertEquals(WatchZoneBoundaryResult.DuplicateVertex, WatchZoneBoundary.of(vertices))
    }

    // MARK: - Self-intersection

    @Test
    fun `a self-intersecting bowtie ring is rejected`() {
        val bowtie =
            listOf(
                Coordinate(latitude = 51.50, longitude = -0.10),
                Coordinate(latitude = 51.51, longitude = -0.09),
                Coordinate(latitude = 51.50, longitude = -0.09),
                Coordinate(latitude = 51.51, longitude = -0.10),
            )

        assertEquals(WatchZoneBoundaryResult.SelfIntersecting, WatchZoneBoundary.of(bowtie))
    }

    @Test
    fun `a zero-area collinear ring is rejected`() {
        val collinear =
            listOf(
                Coordinate(latitude = 51.50, longitude = -0.10),
                Coordinate(latitude = 51.50, longitude = -0.09),
                Coordinate(latitude = 51.50, longitude = -0.08),
            )

        assertEquals(WatchZoneBoundaryResult.SelfIntersecting, WatchZoneBoundary.of(collinear))
    }

    // MARK: - Out of bounds

    @Test
    fun `a vertex outside the UK bounding box is rejected`() {
        val vertices =
            listOf(
                Coordinate(latitude = 51.5, longitude = -0.1),
                Coordinate(latitude = 51.51, longitude = -0.11),
                Coordinate(latitude = 51.5, longitude = 40.0),
            )

        assertEquals(WatchZoneBoundaryResult.OutOfBounds, WatchZoneBoundary.of(vertices))
    }

    // MARK: - Centroid

    @Test
    fun `centroid of a known rectangle is its geometric centre`() {
        val rectangle =
            listOf(
                Coordinate(latitude = 51.0, longitude = -1.0),
                Coordinate(latitude = 52.0, longitude = -1.0),
                Coordinate(latitude = 52.0, longitude = 0.0),
                Coordinate(latitude = 51.0, longitude = 0.0),
            )
        val result = assertIs<WatchZoneBoundaryResult.Valid>(WatchZoneBoundary.of(rectangle))

        val centroid = result.boundary.centroid

        assertTrue(abs(centroid.latitude - 51.5) < 1e-9)
        assertTrue(abs(centroid.longitude - (-0.5)) < 1e-9)
    }

    @Test
    fun `centroid is the area-weighted centre not the vertex average`() {
        // A staircase/L-shape: (lat,lon) (0,0),(0,6),(2,6),(2,2),(4,2),(4,0).
        // True polygon centroid is (1.5, 2.5); the naive vertex average is
        // (2.0, 2.667) — different enough to fail a naive-average
        // implementation. Shifted into UK bounds by a +51 latitude, -7
        // longitude offset (the shape spans 6 degrees of longitude, so the
        // offset must keep both ends within the UK longitude bounds, not
        // just the starting vertex).
        val baseLat = 51.0
        val baseLon = -7.0
        val shape =
            listOf(
                Coordinate(latitude = baseLat, longitude = baseLon),
                Coordinate(latitude = baseLat, longitude = baseLon + 6),
                Coordinate(latitude = baseLat + 2, longitude = baseLon + 6),
                Coordinate(latitude = baseLat + 2, longitude = baseLon + 2),
                Coordinate(latitude = baseLat + 4, longitude = baseLon + 2),
                Coordinate(latitude = baseLat + 4, longitude = baseLon),
            )
        val result = assertIs<WatchZoneBoundaryResult.Valid>(WatchZoneBoundary.of(shape))

        val centroid = result.boundary.centroid

        assertTrue(abs(centroid.latitude - (baseLat + 1.5)) < 1e-9)
        assertTrue(abs(centroid.longitude - (baseLon + 2.5)) < 1e-9)
    }

    // MARK: - Enclosing radius

    @Test
    fun `enclosing radius of a known square matches the precomputed distance`() {
        val centreLat = 51.5074
        val centreLon = -0.1278
        val deltaLat = 0.0089831
        val deltaLon = 0.0144328
        val square =
            listOf(
                Coordinate(latitude = centreLat + deltaLat, longitude = centreLon + deltaLon),
                Coordinate(latitude = centreLat + deltaLat, longitude = centreLon - deltaLon),
                Coordinate(latitude = centreLat - deltaLat, longitude = centreLon - deltaLon),
                Coordinate(latitude = centreLat - deltaLat, longitude = centreLon + deltaLon),
            )
        val result = assertIs<WatchZoneBoundaryResult.Valid>(WatchZoneBoundary.of(square))

        val radius = result.boundary.enclosingRadiusMetres

        assertTrue(abs(radius - 1_412.694_247_415_168) < 0.5)
    }
}
