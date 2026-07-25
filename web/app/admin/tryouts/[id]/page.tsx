"use client";

import { use } from "react";
import { TryoutBuilder } from "@/components/admin/tryout-builder";

export default function TryoutBuilderPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  return <TryoutBuilder tryoutId={id === "new" ? undefined : id} />;
}
