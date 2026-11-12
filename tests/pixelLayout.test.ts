import { describe, it, expect } from "vitest";
import {
  LAYOUT,
  pixelUrl,
  ringY,
  roadKeys,
  stageFile,
  wallKind,
} from "../src/ui/scene/pixel/layout";
import {
  facing,
  loop,
  pathLength,
  pointAt,
  type Pt,
} from "../src/ui/scene/pixel/path";
import { BUILDING_IDS, type BuildingId } from "../src/engine/types";

const zero = Object.fromEntries(BUILDING_IDS.map((id) => [id, 0])) as Record<
  BuildingId,
  number
>;

describe("pixel layout", () => {
  it("pixelUrl resolves an exported file and throws for a missing one", () => {
    expect(pixelUrl("buildings/main-3.png")).toMatch(/main-3/);
    expect(() => pixelUrl("buildings/nope.png")).toThrow(
      /Missing pixel art asset: buildings\/nope.png/,
    );
  });

  it("stageFile falls back to the nearest stage: the statue only has stage 3", () => {
    expect(stageFile("statue", 1)).toBe(stageFile("statue", 3));
    expect(stageFile("main", 2)).toBe("buildings/main-2.png");
  });

  it("roadKeys: gate and rally always, building roads by level or construction, side roads by iron and farm", () => {
    expect(roadKeys(zero, []).sort()).toEqual(["gate", "rally"]);
    expect(roadKeys(zero, ["academy"])).toContain("academy");
    const keys = roadKeys({ ...zero, iron: 1, farm: 2, wood: 1, main: 3 }, []);
    expect(keys).toEqual(
      expect.arrayContaining(["side_w", "side_e", "wood", "main"]),
    );
    expect(keys).not.toContain("clay");
    for (const k of roadKeys(
      { ...Object.fromEntries(BUILDING_IDS.map((id) => [id, 1])) } as Record<
        BuildingId,
        number
      >,
      [],
    )) {
      expect(LAYOUT.roads[k], k).toBeDefined();
    }
  });

  it("a side road always brings the road that connects it to the plaza", () => {
    expect(roadKeys({ ...zero, farm: 1 }, [])).toEqual(
      expect.arrayContaining(["side_e", "academy"]),
    );
    expect(roadKeys({ ...zero, iron: 1 }, [])).toEqual(
      expect.arrayContaining(["side_w", "storage"]),
    );
  });

  it("wallKind: 0 → none, 1–2 → palisade, 3 → stone", () => {
    expect([0, 1, 2, 3].map((v) => wallKind(v as 0 | 1 | 2 | 3))).toEqual([
      null,
      "palisade",
      "palisade",
      "stone",
    ]);
  });

  it("the doorway of both gates holds a walker standing in the gate on the wall base, so the gate front leaves it uncovered", () => {
    // Opaque columns of a walker sprite around its anchor, and the part of its height that must show between the open doors.
    const half = 7;
    const body = 20;
    const [x, foot] = [LAYOUT.gates.s[0], LAYOUT.ring.cy + LAYOUT.ring.ry];
    for (const kind of ["palisade", "stone"] as const) {
      const g = LAYOUT.walls[kind].gate;
      const left = Math.floor(g.x - g.w / 2);
      const top = Math.floor(g.y - g.foot);
      const holds = g.doorway.some(
        ([x0, y0, x1, y1]) =>
          left + x0 <= x - half &&
          x + half <= left + x1 &&
          top + y0 <= foot - body &&
          foot <= top + y1,
      );
      expect(holds, `${kind} doorway ${JSON.stringify(g.doorway)}`).toBe(true);
    }
  });

  it("the mower stands inside the wheat field along his whole pass, and his blade stays over it", () => {
    const wheat = LAYOUT.scenery.find(
      (s) => s.file === "scenery/plot-wheat.png",
    )!;
    const [w] = LAYOUT.files[wheat.file]!;
    // The plot's ground is a 2:1 diamond whose bottom corner is the sprite's anchor.
    const inside = ([x, y]: Pt) =>
      Math.abs(x - wheat.x) / (w / 2) +
        Math.abs(y - (wheat.y - w / 4)) / (w / 4) <=
      1;
    const s = LAYOUT.villagers.mow;
    const [, y] = LAYOUT.mow[0];
    expect(LAYOUT.mow[1][1]).toBe(y);
    expect(LAYOUT.mow[0][0]).toBeLessThan(LAYOUT.mow[1][0]);
    for (const p of LAYOUT.mow) expect(inside(p), JSON.stringify(p)).toBe(true);
    // The sprite is cut to the reach of the blade, which swings ahead of him to the east.
    expect(
      LAYOUT.mow[1][0] + s.w - s.anchor[0],
      "blade at the end of the pass",
    ).toBeLessThanOrEqual(wheat.x + w / 2);
    expect(s.file.east).toBeDefined();
  });

  it("the mower swaps between mowing and walking back in place: both sprites share their size and anchor", () => {
    const { mow, scythe_walk: walk } = LAYOUT.villagers;
    expect([walk.w, walk.h, walk.frames, walk.anchor]).toEqual([
      mow.w,
      mow.h,
      mow.frames,
      mow.anchor,
    ]);
    expect(walk.file.west).toBeDefined();
  });

  it("village people walk in four directions; the guard has its own sprite, smaller than the soldier of the map armies", () => {
    for (const k of ["walk", "carry_log", "carry_sack", "guard"] as const) {
      expect(Object.keys(LAYOUT.villagers[k].file).sort(), k).toEqual([
        "east",
        "north",
        "south",
        "west",
      ]);
    }
    expect(LAYOUT.villagers.guard.file.south).not.toBe(
      LAYOUT.villagers.spear.file.south,
    );
  });

  it("ringY gives cy ± ry on the vertical axis and cy outside the ellipse", () => {
    const { cx, cy, ry, rx } = LAYOUT.ring;
    expect(ringY(cx, "north")).toBeCloseTo(cy - ry);
    expect(ringY(cx, "south")).toBeCloseTo(cy + ry);
    expect(ringY(cx + rx + 10, "south")).toBe(cy);
  });
});

describe("path geometry", () => {
  const line: Pt[] = [
    [0, 0],
    [3, 4],
    [3, 10],
  ];

  it("pathLength and pointAt walk the polyline linearly, clamping the distance", () => {
    expect(pathLength(line)).toBe(11);
    expect(pointAt(line, 5)).toEqual([3, 4]);
    expect(pointAt(line, 8)).toEqual([3, 7]);
    expect(pointAt(line, -2)).toEqual([0, 0]);
    expect(pointAt(line, 99)).toEqual([3, 10]);
  });

  it("facing: right → east, down → south, 45° diagonal → south", () => {
    expect(facing([0, 0], [5, 0])).toBe("east");
    expect(facing([0, 0], [-5, 1])).toBe("west");
    expect(facing([0, 0], [0, 5])).toBe("south");
    expect(facing([0, 0], [0, -5])).toBe("north");
    expect(facing([0, 0], [5, 5])).toBe("south");
  });

  it("loop goes there and back without repeating the ends", () => {
    expect(
      loop([
        [0, 0],
        [1, 1],
        [2, 2],
      ]),
    ).toEqual([
      [0, 0],
      [1, 1],
      [2, 2],
      [1, 1],
    ]);
  });
});
