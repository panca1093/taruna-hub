"use client";

import { Modal } from "@heroui/react";

export function FormModal({
  isOpen,
  onOpenChange,
  title,
  description,
  size = "md",
  footer,
  children,
}: {
  isOpen: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  description?: string;
  size?: "xs" | "sm" | "md" | "lg" | "xl" | "full";
  footer?: React.ReactNode;
  children: React.ReactNode;
}) {
  // HeroUI's Modal size variants top out at "lg" (max-w-lg) below "full" (100vw).
  // "xl" reuses the "lg" dialog chrome but forces a wider max-width via inline
  // style, which always wins over the variant's CSS class regardless of layers.
  const isWide = size === "xl";
  return (
    <Modal isOpen={isOpen} onOpenChange={onOpenChange}>
      <Modal.Backdrop>
        <Modal.Container size={isWide ? "lg" : size} scroll="inside">
          <Modal.Dialog style={isWide ? { maxWidth: "64rem" } : undefined}>
            <Modal.Header>
              <Modal.Heading className="font-display text-hero text-foreground">{title}</Modal.Heading>
              {description && <p className="mt-2 text-caption leading-relaxed text-muted">{description}</p>}
            </Modal.Header>
            <Modal.Body className="flex flex-col gap-4">{children}</Modal.Body>
            {footer && <Modal.Footer>{footer}</Modal.Footer>}
          </Modal.Dialog>
        </Modal.Container>
      </Modal.Backdrop>
    </Modal>
  );
}
