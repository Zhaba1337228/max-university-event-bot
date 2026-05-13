// Утилиты форматирования. Дата — в локальном TZ браузера.

export function fmtDate(iso?: string): string {
  if (!iso) return "—";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "—";
  return d.toLocaleString("ru-RU", {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export function statusLabel(s: string): string {
  switch (s) {
    case "open":
      return "Открыто";
    case "closed":
      return "Закрыто";
    case "cancelled":
      return "Отменено";
    case "completed":
      return "Завершено";
    case "draft":
      return "Черновик";
    default:
      return s;
  }
}

export function statusBadge(s: string): string {
  switch (s) {
    case "open":
      return "bg-success/15 text-success border border-success/30";
    case "closed":
    case "cancelled":
      return "bg-danger/15 text-danger border border-danger/30";
    case "completed":
      return "bg-accent/15 text-accent border border-accent/30";
    default:
      return "bg-muted text-subtle border border-border";
  }
}
