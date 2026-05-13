import clsx from "clsx";
import { HTMLAttributes } from "react";

type Props = HTMLAttributes<HTMLSpanElement>;

export function Badge({ className, ...rest }: Props) {
  return (
    <span
      className={clsx(
        "inline-flex items-center rounded-full px-2.5 py-0.5",
        "text-xs font-medium",
        className,
      )}
      {...rest}
    />
  );
}
