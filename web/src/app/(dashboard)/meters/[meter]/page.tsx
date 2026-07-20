"use client";

import { useParams } from "next/navigation";

import { MeterDetailPage } from "@/product/pages/MeterDetailPage";

export default function MeterDetailRoute() {
  const { meter } = useParams<{ meter: string }>();
  return <MeterDetailPage routeMeter={decodeURIComponent(meter)} />;
}
