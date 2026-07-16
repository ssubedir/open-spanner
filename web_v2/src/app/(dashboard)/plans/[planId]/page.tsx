"use client";

import { useParams } from "next/navigation";

import { PlanDetailPage } from "@/product/pages/PlanDetailPage";

export default function PlanDetailRoute() {
  const { planId } = useParams<{ planId: string }>();
  return <PlanDetailPage planId={decodeURIComponent(planId)} />;
}
