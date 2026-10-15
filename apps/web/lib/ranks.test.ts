import { describe, expect, it } from "vitest";
import { rankForXp } from "@/lib/ranks";

describe("rankForXp", () => {
  it.each([
    { xp: 0, name: "Egg", level: 1, minXp: 0, nextXp: 100 },
    { xp: 99, name: "Egg", level: 1, minXp: 0, nextXp: 100 },
    { xp: 100, name: "Hatchling", level: 2, minXp: 100, nextXp: 300 },
    { xp: 299, name: "Hatchling", level: 2, minXp: 100, nextXp: 300 },
    { xp: 300, name: "Gopher", level: 3, minXp: 300, nextXp: 700 },
    { xp: 699, name: "Gopher", level: 3, minXp: 300, nextXp: 700 },
    { xp: 700, name: "Pathfinder", level: 4, minXp: 700, nextXp: 1500 },
    { xp: 1499, name: "Pathfinder", level: 4, minXp: 700, nextXp: 1500 },
    { xp: 1500, name: "Concurrency Knight", level: 5, minXp: 1500, nextXp: 3000 },
    { xp: 2999, name: "Concurrency Knight", level: 5, minXp: 1500, nextXp: 3000 },
    { xp: 3000, name: "Go Master", level: 6, minXp: 3000, nextXp: null },
    { xp: -3, name: "Egg", level: 1, minXp: 0, nextXp: 100 },
  ])("maps $xp XP", ({ xp, name, level, minXp, nextXp }) => {
    expect(rankForXp(xp)).toEqual({ name, level, minXp, nextXp });
  });
});
