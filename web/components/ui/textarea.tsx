"use client";

import clsx from "clsx";
import { TextareaHTMLAttributes, forwardRef } from "react";

type Props = TextareaHTMLAttributes<HTMLTextAreaElement>;

export const Textarea = forwardRef<HTMLTextAreaElement, Props>(function Textarea(
  { className, ...rest },
  ref,
) {
  return (
    <textarea
      ref={ref}
      className={clsx(
        "w-full rounded-md border border-border bg-muted/70 px-3 py-2.5",
        "text-sm text-text placeholder:text-subtle",
        "focus:outline-none focus:ring-2 focus:ring-accent/50 focus:border-accent/40",
        "min-h-[140px] resize-vertical",
        className,
      )}
      {...rest}
    />
  );
});
