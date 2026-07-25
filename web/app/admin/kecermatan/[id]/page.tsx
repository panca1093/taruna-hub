"use client";

import { use } from "react";
import { KecermatanBuilder } from "@/components/admin/kecermatan-builder";

export default function KecermatanBuilderPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  return <KecermatanBuilder testId={id === "new" ? undefined : id} />;
}
