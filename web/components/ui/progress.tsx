import clsx from "clsx";

type Props = {
  value: number; // 0–100
  className?: string;
  colorClass?: string; // tailwind color for the fill, default accent
  size?: "sm" | "md";
  showLabel?: boolean;
};

export function Progress({ value, className, colorClass, size = "sm", showLabel }: Props) {
  const clamped = Math.min(100, Math.max(0, value));
  const fill = colorClass ?? "bg-accent";
  const h = size === "sm" ? "h-1.5" : "h-2.5";
  return (
    <div className={clsx("flex items-center gap-2", className)}>
      <div className={clsx("flex-1 overflow-hidden rounded-full bg-muted", h)}>
        <div
          className={clsx("h-full rounded-full transition-all duration-500", fill)}
          style={{ width: `${clamped}%` }}
        />
      </div>
      {showLabel && (
        <span className="min-w-[2.5rem] text-right text-xs tabular-nums text-subtle">
          {Math.round(clamped)}%
        </span>
      )}
    </div>
  );
}
