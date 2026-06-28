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

describe("notification-state types", () => {
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
