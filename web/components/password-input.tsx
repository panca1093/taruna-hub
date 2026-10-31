"use client";

import { useState } from "react";
import { Button, Input } from "@heroui/react";
import { Eye, EyeOff } from "lucide-react";
import { FormField } from "@/components/ui/form";
import { useAuthT } from "@/lib/i18n/dict/auth";

export function PasswordInput({ label, error, ...props }: {
  label: string;
  error?: string;
} & Omit<React.ComponentProps<typeof Input>, "type" | "className">) {
  const [visible, setVisible] = useState(false);
  const t = useAuthT();
  return (
    <FormField label={label} error={error} required={props.required}>
      <div className="relative">
        <Input {...props} fullWidth type={visible ? "text" : "password"} className="pr-12" />
        <Button type="button" variant="ghost" isIconOnly size="sm" className="absolute right-1 top-1 size-10 text-muted" aria-label={visible ? t.hidePassword : t.showPassword} aria-pressed={visible} onPress={() => setVisible((value) => !value)}>
          {visible ? <EyeOff className="size-4" /> : <Eye className="size-4" />}
        </Button>
      </div>
    </FormField>
  );
}
