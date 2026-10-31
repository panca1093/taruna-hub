import { Suspense } from "react";
import { AuthLogin } from "@/components/auth-login";

export default function PortalPage() {
  return <Suspense><AuthLogin staff /></Suspense>;
}
