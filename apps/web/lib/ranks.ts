export type Rank = { name: string; level: number; minXp: number; nextXp: number | null };

const rankSteps = [
  { name: "Egg", minXp: 0 },
  { name: "Hatchling", minXp: 100 },
  { name: "Gopher", minXp: 300 },
  { name: "Pathfinder", minXp: 700 },
  { name: "Concurrency Knight", minXp: 1500 },
  { name: "Go Master", minXp: 3000 },
] as const;

export function rankForXp(totalXp: number): Rank {
  const xp = Math.max(0, Math.floor(totalXp));
  const index = rankSteps.reduce((current, rank, i) => xp >= rank.minXp ? i : current, 0);
  const current = rankSteps[index];
  return { name: current.name, level: index + 1, minXp: current.minXp, nextXp: rankSteps[index + 1]?.minXp ?? null };
}
