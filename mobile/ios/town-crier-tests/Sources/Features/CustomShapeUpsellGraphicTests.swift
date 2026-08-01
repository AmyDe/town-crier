import SwiftUI
import Testing

@testable import TownCrierPresentation

@Suite("CustomShapeUpsellGraphic")
@MainActor
struct CustomShapeUpsellGraphicTests {

  @Test func init_rendersWithoutCrashing() {
    let sut = CustomShapeUpsellGraphic()
    _ = sut.body
  }
}

@Suite("CustomShapePolygonGeometry")
struct CustomShapePolygonGeometryTests {

  @Test func vertices_returnsOnePointPerRadiusMultiplier() {
    let rect = CGRect(x: 0, y: 0, width: 100, height: 100)

    let vertices = CustomShapePolygonGeometry.vertices(in: rect)

    #expect(vertices.count == CustomShapePolygonGeometry.radiusMultipliers.count)
  }

  @Test func vertices_isDeterministicForTheSameRect() {
    let rect = CGRect(x: 0, y: 0, width: 100, height: 100)

    let first = CustomShapePolygonGeometry.vertices(in: rect)
    let second = CustomShapePolygonGeometry.vertices(in: rect)

    #expect(first == second)
  }

  @Test func vertices_areContainedWithinTheRectsBoundingRadius() {
    let rect = CGRect(x: 0, y: 0, width: 100, height: 100)
    let center = CGPoint(x: rect.midX, y: rect.midY)
    let maxRadius = min(rect.width, rect.height) / 2

    let vertices = CustomShapePolygonGeometry.vertices(in: rect)

    for vertex in vertices {
      let distance = (vertex - center).magnitude
      #expect(distance <= maxRadius + 0.001)
    }
  }

  @Test func vertices_areNotAllIdentical() {
    let rect = CGRect(x: 0, y: 0, width: 100, height: 100)

    let vertices = CustomShapePolygonGeometry.vertices(in: rect)
    let distinctVertices = Set(vertices.map { "\($0.x),\($0.y)" })

    #expect(distinctVertices.count == vertices.count)
  }

  @Test func vertices_scalesWithASmallerRect() {
    let smallRect = CGRect(x: 0, y: 0, width: 10, height: 10)
    let center = CGPoint(x: smallRect.midX, y: smallRect.midY)
    let maxRadius = min(smallRect.width, smallRect.height) / 2

    let vertices = CustomShapePolygonGeometry.vertices(in: smallRect)

    for vertex in vertices {
      let distance = (vertex - center).magnitude
      #expect(distance <= maxRadius + 0.001)
    }
  }
}

extension CGPoint {
  private static func - (lhs: CGPoint, rhs: CGPoint) -> CGVector {
    CGVector(dx: lhs.x - rhs.x, dy: lhs.y - rhs.y)
  }
}

extension CGVector {
  private var magnitude: CGFloat {
    (dx * dx + dy * dy).squareRoot()
  }
}
