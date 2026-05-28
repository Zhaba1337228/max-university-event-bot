import { IconArrowLeft, IconArrowRight } from "@/components/ui/icons";
import { Button } from "@/components/ui/button";

interface PaginationProps {
  offset: number;
  pageSize: number;
  total: number;
  loading?: boolean;
  onPage: (offset: number) => void;
}

export function Pagination({ offset, pageSize, total, loading, onPage }: PaginationProps) {
  const currentPage = Math.floor(offset / pageSize) + 1;
  const totalPages = Math.max(1, Math.ceil(total / pageSize));

  // Build page numbers to show: always first, last, current ±1, with "…" gaps
  function getPages(): (number | "…")[] {
    if (totalPages <= 7) return Array.from({ length: totalPages }, (_, i) => i + 1);
    const pages = new Set<number>([1, totalPages, currentPage]);
    if (currentPage > 1) pages.add(currentPage - 1);
    if (currentPage < totalPages) pages.add(currentPage + 1);
    const sorted = Array.from(pages).sort((a, b) => a - b);
    const result: (number | "…")[] = [];
    for (let i = 0; i < sorted.length; i++) {
      if (i > 0 && sorted[i] - sorted[i - 1] > 1) result.push("…");
      result.push(sorted[i]);
    }
    return result;
  }

  const from = total === 0 ? 0 : offset + 1;
  const to = Math.min(offset + pageSize, total);

  return (
    <div className="mt-5 flex flex-col items-center gap-3 border-t border-border/60 pt-4 sm:flex-row sm:justify-between">
      {/* Counter */}
      <span className="text-xs text-subtle order-2 sm:order-1">
        {total === 0 ? "Нет записей" : `${from}–${to} из ${total}`}
      </span>

      {/* Pages */}
      <div className="flex items-center gap-1 order-1 sm:order-2">
        <Button
          variant="secondary"
          disabled={currentPage === 1 || loading}
          onClick={() => onPage(offset - pageSize)}
          className="h-8 w-8 p-0"
          aria-label="Назад"
        >
          <IconArrowLeft size={14} />
        </Button>

        {getPages().map((p, i) =>
          p === "…" ? (
            <span key={`gap-${i}`} className="px-1 text-xs text-subtle select-none">…</span>
          ) : (
            <button
              key={p}
              type="button"
              disabled={loading}
              onClick={() => onPage((p - 1) * pageSize)}
              className={`h-8 min-w-[2rem] rounded-md px-2 text-xs font-medium transition-colors ${
                p === currentPage
                  ? "bg-accent text-white shadow-sm"
                  : "bg-muted/60 text-subtle hover:bg-muted hover:text-text"
              }`}
            >
              {p}
            </button>
          )
        )}

        <Button
          variant="secondary"
          disabled={currentPage === totalPages || loading}
          onClick={() => onPage(offset + pageSize)}
          className="h-8 w-8 p-0"
          aria-label="Вперёд"
        >
          <IconArrowRight size={14} />
        </Button>
      </div>

      {/* Page X of Y */}
      <span className="text-xs text-subtle order-3">
        Стр. {currentPage} из {totalPages}
      </span>
    </div>
  );
}
