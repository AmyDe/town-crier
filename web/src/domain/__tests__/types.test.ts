import { describe, it, expect } from "vitest";
import {
  type ApplicationUid,
  type WatchZoneId,
  type AuthorityId,
  type LatestUnreadEvent,
  type NotificationStateSnapshot,
  asApplicationUid,
  asWatchZoneId,
  asAuthorityId,
  isApplicationStatus,
  isSubscriptionTier,
  isNotificationEventType,
} from "../types";
import * as types from "../types";

describe("branded type constructors", () => {
  it("creates an ApplicationUid from a string", () => {
    const uid: ApplicationUid = asApplicationUid("APP-001");
    expect(uid).toBe("APP-001");
  });

  it("creates a WatchZoneId from a string", () => {
    const id: WatchZoneId = asWatchZoneId("zone-123");
    expect(id).toBe("zone-123");
  });

  it("creates an AuthorityId from a number", () => {
    const id: AuthorityId = asAuthorityId(42);
    expect(id).toBe(42);
  });
});

describe("type guards", () => {
  describe("isApplicationStatus", () => {
    it("returns true for valid statuses (PlanIt vocabulary)", () => {
      expect(isApplicationStatus("Undecided")).toBe(true);
      expect(isApplicationStatus("Permitted")).toBe(true);
      expect(isApplicationStatus("Conditions")).toBe(true);
      expect(isApplicationStatus("Rejected")).toBe(true);
      expect(isApplicationStatus("Withdrawn")).toBe(true);
      expect(isApplicationStatus("Appealed")).toBe(true);
      expect(isApplicationStatus("Unresolved")).toBe(true);
      expect(isApplicationStatus("Referred")).toBe(true);
      expect(isApplicationStatus("Not Available")).toBe(true);
    });

    it("returns false for legacy/invalid statuses", () => {
      // Legacy vocabulary that PlanIt does not actually use
      expect(isApplicationStatus("Approved")).toBe(false);
      expect(isApplicationStatus("Refused")).toBe(false);
      expect(isApplicationStatus("invalid")).toBe(false);
      expect(isApplicationStatus("")).toBe(false);
      expect(isApplicationStatus(42)).toBe(false);
    });
  });

  describe("isSubscriptionTier", () => {
    it("returns true for valid tiers", () => {
      expect(isSubscriptionTier("Free")).toBe(true);
      expect(isSubscriptionTier("Personal")).toBe(true);
      expect(isSubscriptionTier("Pro")).toBe(true);
    });

    it("returns false for invalid tiers", () => {
      expect(isSubscriptionTier("Premium")).toBe(false);
      expect(isSubscriptionTier("")).toBe(false);
    });
  });

});

describe("notification-state types", () => {
  it("isNotificationEventType returns true for the wire-format names", () => {
    expect(isNotificationEventType("NewApplication")).toBe(true);
    expect(isNotificationEventType("DecisionUpdate")).toBe(true);
  });

  it("isNotificationEventType returns false for unknown values", () => {
    expect(isNotificationEventType("Other")).toBe(false);
    expect(isNotificationEventType("")).toBe(false);
    expect(isNotificationEventType(0)).toBe(false);
    expect(isNotificationEventType(null)).toBe(false);
  });

  it("LatestUnreadEvent surfaces type, decision and createdAt", () => {
    const event: LatestUnreadEvent = {
      type: "DecisionUpdate",
      decision: "Permitted",
      createdAt: "2026-05-04T12:00:00Z",
    };
    expect(event.type).toBe("DecisionUpdate");
    expect(event.decision).toBe("Permitted");
    expect(event.createdAt).toBe("2026-05-04T12:00:00Z");
  });

  it("NotificationStateSnapshot surfaces lastReadAt, version and totalUnreadCount", () => {
    const state: NotificationStateSnapshot = {
      lastReadAt: "2026-05-04T12:00:00Z",
      version: 3,
      totalUnreadCount: 7,
    };
    expect(state.lastReadAt).toBe("2026-05-04T12:00:00Z");
    expect(state.version).toBe(3);
    expect(state.totalUnreadCount).toBe(7);
  });
});

describe("Groups removal", () => {
  it("does not export Group-related symbols", () => {
    const exportedNames = Object.keys(types);
    const groupSymbols = exportedNames.filter(
      (name) => /group|invitation/i.test(name)
    );
    expect(groupSymbols).toEqual([]);
  });
});
