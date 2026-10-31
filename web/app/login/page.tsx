import { Suspense } from "react";
import { AuthLogin } from "@/components/auth-login";

export default function LoginPage() {
  return <Suspense><AuthLogin /></Suspense>;
}
