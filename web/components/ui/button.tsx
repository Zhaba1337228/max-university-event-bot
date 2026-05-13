"use client";

import clsx from "clsx";
import { ButtonHTMLAttributes, forwardRef } from "react";

type Variant = "primary" | "secondary" | "danger" | "ghost";
type Size = "md" | "lg";

type Props = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: Variant;
  size?: Size;
};

const styles: Record<Variant, string> = {
  primary:
    "bg-accent hover:bg-accentHover text-white border border-transparent shadow-card",
  secondary:
    "bg-muted hover:bg-surfaceAlt text-text border border-border",
  danger:
    "bg-danger/90 hover:bg-danger text-white border border-transparent shadow-card",
  ghost:
    "bg-transparent hover:bg-muted text-text border border-transparent",
};

const sizes: Record<Size, string> = {
  md: "px-3 py-2 text-sm",
  lg: "px-4 py-2.5 text-base",
};

export const Button = forwardRef<HTMLButtonElement, Props>(function Button(
  { className, variant = "primary", size = "md", disabled, ...rest },
  ref,
) {
  return (
    <button
      ref={ref}
      className={clsx(
        "inline-flex items-center justify-center rounded-md font-medium transition-colors",
        "focus:outline-none focus-visible:ring-2 focus-visible:ring-accent/60",
        "disabled:opacity-50 disabled:cursor-not-allowed",
        sizes[size],
        styles[variant],
        className,
      )}
      disabled={disabled}
      {...rest}
    />
  );
});
