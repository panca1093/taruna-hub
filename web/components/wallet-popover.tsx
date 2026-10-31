"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Button, Modal } from "@heroui/react";
import { ArrowRight, Wallet as WalletIcon } from "lucide-react";
import { Icon } from "@/components/icon";
import { cn } from "@/lib/cn";
import { api, type Wallet, type WalletEntry } from "@/lib/api/client";
import { formatIDR, formatSignedIDR } from "@/lib/format-money";
import { useStudentWalletT } from "@/lib/i18n/dict/student-wallet";
import { useWalletTopupT } from "@/lib/i18n/dict/wallet-topup";

const RECENT = 4;

export function WalletPopover({ className }: { className?: string }) {
  const t = useWalletTopupT();
  const ledgerT = useStudentWalletT();
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [wallet, setWallet] = useState<Wallet | null>(null);
  const [entries, setEntries] = useState<WalletEntry[] | null>(null);
  const [failed, setFailed] = useState(false);

  const load = useCallback(() => {
    Promise.all([api.wallet.get(), api.wallet.entries({ limit: RECENT })])
      .then(([head, page]) => {
        setWallet(head);
        setEntries(page.entries);
        setFailed(false);
      })
      .catch(() => setFailed(true));
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const onOpenChange = (next: boolean) => {
    setOpen(next);
    if (next) load();
  };

  const go = (href: string) => {
    setOpen(false);
    router.push(href);
  };

  return (
    <>
      <Button
        variant="ghost"
        size="sm"
        className={cn("gap-2", className)}
        aria-haspopup="dialog"
        aria-label={t.popoverTitle}
        onPress={() => onOpenChange(true)}
      >
        <Icon icon={WalletIcon} size="sm" />
        <span className="text-small font-medium tabular-nums">{wallet === null ? "—" : formatIDR(wallet.balance)}</span>
      </Button>

      <Modal isOpen={open} onOpenChange={onOpenChange}>
        <Modal.Backdrop>
          <Modal.Container size="sm" scroll="inside">
            <Modal.Dialog>
              <Modal.Header>
                <Modal.Heading className="font-display text-hero text-foreground">{t.popoverTitle}</Modal.Heading>
                <p className="mt-2 text-caption leading-relaxed text-muted">{t.popoverDescription}</p>
                <Modal.CloseTrigger aria-label={t.popoverClose} />
              </Modal.Header>
              <Modal.Body className="flex flex-col gap-6">
                {failed ? (
                  <div className="space-y-2">
                    <p className="text-small text-danger">{t.popoverLoadFailed}</p>
                    <Button size="sm" variant="secondary" onPress={load}>
                      {t.popoverRetry}
                    </Button>
                  </div>
                ) : (
                  <div>
                    <p className="font-display text-metric-xl text-foreground">
                      {wallet === null ? "—" : formatIDR(wallet.balance)}
                    </p>
                    <p className="mt-2 text-caption text-muted">{t.popoverAvailable}</p>
                  </div>
                )}
                <div className="flex flex-wrap gap-2.5">
                  <Button variant="primary" onPress={() => go("/student/wallet/topup")}>
                    {t.popoverTopUp}
                    <Icon icon={ArrowRight} size="sm" />
                  </Button>
                  <Button variant="secondary" onPress={() => go("/student/wallet")}>
                    {t.popoverHistory}
                    <Icon icon={ArrowRight} size="sm" />
                  </Button>
                </div>
                <div>
                  <div className="flex items-start justify-between gap-4 border-b border-border pb-4">
                    <div>
                      <p className="text-caption font-medium text-foreground">{t.popoverRecent}</p>
                      <p className="mt-1 text-label text-muted">{t.popoverRecentHint}</p>
                    </div>
                    <Icon icon={WalletIcon} size="sm" className="text-rose" />
                  </div>
                  {entries === null ? (
                    <p className="py-4 text-caption text-muted">{failed ? "" : t.loading}</p>
                  ) : entries.length === 0 ? (
                    <p className="py-4 text-caption text-muted">{t.popoverRecentEmpty}</p>
                  ) : (
                    <ul>
                      {entries.map((entry) => {
                        const credit = !entry.amount.startsWith("-");
                        return (
                          <li
                            key={entry.id}
                            className="flex items-center justify-between gap-4 border-b border-border py-4 text-caption last:border-b-0"
                          >
                            <div className="min-w-0">
                              <p className="truncate font-medium text-foreground">
                                {entry.reason === "" ? (ledgerT.typeLabel[entry.kind] ?? entry.kind) : entry.reason}
                              </p>
                              <p className="mt-1 text-label text-muted">{new Date(entry.created_at).toLocaleDateString()}</p>
                            </div>
                            <span
                              className={cn("font-display text-title whitespace-nowrap", credit ? "text-success" : "text-foreground")}
                            >
                              {formatSignedIDR(entry.amount)}
                            </span>
                          </li>
                        );
                      })}
                    </ul>
                  )}
                </div>
              </Modal.Body>
            </Modal.Dialog>
          </Modal.Container>
        </Modal.Backdrop>
      </Modal>
    </>
  );
}
