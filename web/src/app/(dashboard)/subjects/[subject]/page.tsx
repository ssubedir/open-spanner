"use client";

import { useParams } from "next/navigation";

import { SubjectDetailPage } from "@/product/pages/SubjectDetailPage";

export default function SubjectDetailRoute() {
  const { subject } = useParams<{ subject: string }>();
  return <SubjectDetailPage routeSubject={decodeURIComponent(subject)} />;
}
