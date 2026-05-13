"use client";

import clsx from "clsx";
import { ButtonHTMLAttributes, forwardRef } from "react";

type Variant = "primary" | "secondary" | "danger" | "ghost";

type Props = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: Variant;
};

const styles: Record<Variant, string> = {
  primary:
    "bg-accent hover:bg-accentHover text-white border border-transparent",
  secondary:
    "bg-surface hover:bg-muted text-text border border-border",
  danger:
    "bg-danger/90 hover:bg-danger text-white border border-transparent",
  ghost:
    "bg-transparent hover:bg-muted text-text border border-transparent",
};

export const Button = forwardRef<HTMLButtonElement, Props>(function Button(
  { className, variant = "primary", disabled, ...rest },
  ref,
) {
  return (
    <button
      ref={ref}
      className={clsx(
        "inline-flex items-center justify-center rounded-md px-3 py-2",
        "text-sm font-medium transition-colors focus:outline-none",
        "focus-visible:ring-2 focus-visible:ring-accent/60",
        "disabled:opacity-50 disabled:cursor-not-allowed",
        styles[variant],
        className,
      )}
      disabled={disabled}
      {...rest}
    />
  );
});
