"use client";

import clsx from "clsx";
import { InputHTMLAttributes, forwardRef } from "react";

type Props = InputHTMLAttributes<HTMLInputElement>;

export const Input = forwardRef<HTMLInputElement, Props>(function Input(
  { className, ...rest },
  ref,
) {
  return (
    <input
      ref={ref}
      className={clsx(
        "w-full rounded-md border border-border bg-muted/50 px-3 py-2",
        "text-sm text-text placeholder:text-subtle",
        "focus:outline-none focus:ring-2 focus:ring-accent/50",
        className,
      )}
      {...rest}
    />
  );
});
