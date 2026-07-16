"use client";

import { useSelector } from "@tanstack/react-store";
import { Loader2 } from "lucide-react";
import { useRouter } from "next/navigation";
import { useEffect } from "react";

import { appStore, appStoreActions } from "@/product/app-store";

export function GuestAuthGate({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const auth = useSelector(appStore, (state) => state.auth);

  useEffect(() => {
    void appStoreActions.ensureAuthUser().then((user) => {
      if (user) router.replace("/overview");
    });
  }, [router]);

  if (!auth.checked || auth.loading) {
    return (
      <div className="grid min-h-screen place-items-center bg-[#f7f7f6]" aria-label="Loading session">
        <Loader2 className="size-7 animate-spin text-primary" />
      </div>
    );
  }

  if (auth.session) return null;
  return children;
}
