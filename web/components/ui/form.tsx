"use client";

import {
  FieldError,
  Input,
  Label,
  ListBox,
  Select,
  Switch,
  TextArea,
  TextField,
} from "@heroui/react";
import { ChevronDown } from "lucide-react";
import { cn } from "@/lib/cn";
import { useCommonT } from "@/lib/i18n/dict/common";

export function FormField({
  label,
  description,
  error,
  required,
  className,
  children,
}: {
  label?: string;
  description?: string;
  error?: string;
  required?: boolean;
  className?: string;
  children: React.ReactNode;
}) {
  return (
    <TextField isRequired={required} isInvalid={!!error} className={cn("flex flex-col gap-1.5", className)}>
      {label && <Label className="text-caption font-medium text-foreground">{label}</Label>}
      {children}
      {description && !error && <p className="text-label text-muted">{description}</p>}
      {error && <FieldError className="text-label text-danger">{error}</FieldError>}
    </TextField>
  );
}

export function TextInput({
  label,
  description,
  error,
  required,
  className,
  inputClassName,
  ...props
}: {
  label?: string;
  description?: string;
  error?: string;
  required?: boolean;
  className?: string;
  inputClassName?: string;
} & Omit<React.ComponentProps<typeof Input>, "className">) {
  return (
    <FormField label={label} description={description} error={error} required={required} className={className}>
      <Input fullWidth className={inputClassName} {...props} />
    </FormField>
  );
}

export function TextareaInput({
  label,
  description,
  error,
  required,
  className,
  rows = 4,
  ...props
}: {
  label?: string;
  description?: string;
  error?: string;
  required?: boolean;
  className?: string;
} & Omit<React.ComponentProps<typeof TextArea>, "className">) {
  return (
    <FormField label={label} description={description} error={error} required={required} className={className}>
      <TextArea fullWidth rows={rows} {...props} />
    </FormField>
  );
}

export function SelectInput({
  label,
  description,
  error,
  required,
  className,
  placeholder,
  options,
  selectedKey,
  onSelectionChange,
  isDisabled,
}: {
  label?: string;
  description?: string;
  error?: string;
  required?: boolean;
  className?: string;
  placeholder?: string;
  options: { id: string; label: string }[];
  selectedKey?: string | null;
  onSelectionChange?: (key: string | null) => void;
  isDisabled?: boolean;
}) {
  const commonT = useCommonT();
  return (
    <Select
      isRequired={required}
      isInvalid={!!error}
      isDisabled={isDisabled}
      fullWidth
      placeholder={placeholder ?? commonT.selectPlaceholder}
      selectedKey={selectedKey}
      onSelectionChange={(key) => onSelectionChange?.(key as string | null)}
      className={cn("flex flex-col gap-1.5", className)}
    >
      {label && <Label className="text-caption font-medium text-foreground">{label}</Label>}
      <Select.Trigger className="justify-between">
        <Select.Value />
        <Select.Indicator>
          <ChevronDown className="size-4 text-muted" />
        </Select.Indicator>
      </Select.Trigger>
      <Select.Popover className="min-w-(--trigger-width)">
        <ListBox>
          {options.map((opt) => (
            <ListBox.Item key={opt.id} id={opt.id}>
              {opt.label}
            </ListBox.Item>
          ))}
        </ListBox>
      </Select.Popover>
      {description && !error && <p className="text-label text-muted">{description}</p>}
      {error && <FieldError className="text-label text-danger">{error}</FieldError>}
    </Select>
  );
}

export function SwitchInput({
  label,
  description,
  isSelected,
  onChange,
  isDisabled,
  className,
}: {
  label: string;
  description?: string;
  isSelected: boolean;
  onChange: (checked: boolean) => void;
  isDisabled?: boolean;
  className?: string;
}) {
  return (
    <Switch isSelected={isSelected} onChange={onChange} isDisabled={isDisabled} className={cn("w-full", className)}>
      <Switch.Content className="flex w-full flex-row items-center justify-between gap-3">
        <span className="flex flex-col gap-0.5">
          <span className="text-caption font-medium text-foreground">{label}</span>
          {description && <span className="text-xs text-muted">{description}</span>}
        </span>
        <Switch.Control>
          <Switch.Thumb />
        </Switch.Control>
      </Switch.Content>
    </Switch>
  );
}
