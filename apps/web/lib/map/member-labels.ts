// Labels can disappear in a crowded map; the geographic circles never move.
export type MemberLabel = { id: string; x: number; y: number; width: number; height: number };
type Rect = { left: number; top: number; right: number; bottom: number };
export const memberLabelOffset = 16;

export function memberLocationStatus(status: string): string {
  switch (status) {
    case 'tracking': return '';
    case 'stale': return 'Location delayed';
    case 'offline': return 'Offline';
    case 'spectating': return 'Spectating';
    case 'left': return 'Left';
    default: return 'Last known location';
  }
}

export function visibleMemberLabels(
  labels: MemberLabel[],
  viewport: { width: number; height: number; top?: number; bottom?: number },
  selectedId?: string | null,
): string[] {
  const intersects = (a: Rect, b: Rect) =>
    a.left < b.right && a.right > b.left && a.top < b.bottom && a.bottom > b.top;
  const circles = labels.map(({ x, y }) => ({ left: x - 12, right: x + 12, top: y - 12, bottom: y + 12 }));
  const occupied: Rect[] = [];
  const visible: string[] = [];
  const ordered = [...labels].sort((a, b) => {
    if (a.id === selectedId) return -1;
    if (b.id === selectedId) return 1;
    return a.id < b.id ? -1 : a.id > b.id ? 1 : 0;
  });
  for (const label of ordered) {
    if (![label.x, label.y, label.width, label.height].every(Number.isFinite)) continue;
    const rect = { left: label.x - label.width / 2 - 3, right: label.x + label.width / 2 + 3,
      top: label.y - memberLabelOffset - label.height - 3, bottom: label.y - memberLabelOffset + 3 };
    if (rect.left < 4 || rect.right > viewport.width - 4 || rect.top < (viewport.top ?? 0) + 4 ||
      rect.bottom > viewport.height - (viewport.bottom ?? 0) - 4) continue;
    if (occupied.some((other) => intersects(rect, other)) || circles.some((circle) => intersects(rect, circle))) continue;
    occupied.push(rect);
    visible.push(label.id);
  }
  return visible;
}
