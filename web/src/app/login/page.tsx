"use client";

import { LoginPage } from "@/product/pages/LoginPage";
import { GuestAuthGate } from "@/components/guest-auth-gate";

export default function LoginRoute() {
  return (
    <GuestAuthGate>
      <LoginPage />
    </GuestAuthGate>
  );
}
