"use client";

import NextLink from "next/link";
import { usePathname, useRouter as useNextRouter } from "next/navigation";
import type { ComponentProps, ReactNode } from "react";

type NavigateOptions = {
  to: string;
  params?: Record<string, string>;
  replace?: boolean;
};

export function useRouter() {
  const router = useNextRouter();

  return {
    navigate: async ({ params = {}, replace = false, to }: NavigateOptions) => {
      const href = Object.entries(params).reduce(
        (path, [key, value]) => path.replace(`$${key}`, encodeURIComponent(value)),
        to,
      );
      if (replace) router.replace(href);
      else router.push(href);
    },
  };
}

type LinkProps = Omit<ComponentProps<typeof NextLink>, "href"> & {
  activeProps?: { className?: string };
  children: ReactNode;
  to: string;
};

export function Link({ activeProps, className, to, ...props }: LinkProps) {
  const pathname = usePathname();
  const active = pathname === to || pathname.startsWith(`${to}/`);
  const resolvedClassName = [className, active ? activeProps?.className : undefined].filter(Boolean).join(" ");
  return <NextLink {...props} className={resolvedClassName} href={to} />;
}
