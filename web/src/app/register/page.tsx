"use client";

import { RegisterPage } from "@/product/pages/RegisterPage";
import { GuestAuthGate } from "@/components/guest-auth-gate";

export default function RegisterRoute() {
  return (
    <GuestAuthGate>
      <RegisterPage />
    </GuestAuthGate>
  );
}
