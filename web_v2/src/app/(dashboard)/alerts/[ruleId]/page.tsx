"use client";

import { useParams } from "next/navigation";

import { AlertDetailPage } from "@/product/pages/AlertDetailPage";

export default function AlertDetailRoute() {
  const { ruleId } = useParams<{ ruleId: string }>();
  return <AlertDetailPage ruleId={decodeURIComponent(ruleId)} />;
}
