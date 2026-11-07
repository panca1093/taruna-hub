"use client";

import { useEffect, useState } from "react";
import { Button } from "@heroui/react";
import { CloudOff, Pencil, Plus, Power, Search, Users } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { RequireRole } from "@/components/require-role";
import { DataTable, type Column } from "@/components/data-table";
import { DetailGrid, DetailItem } from "@/components/admin-detail";
import { EmptyState } from "@/components/empty-state";
import { FilterPills } from "@/components/filter-pills";
import { Icon } from "@/components/icon";
import { NoteBox } from "@/components/page-blocks";
import { StatusChip } from "@/components/status-badge";
import { GradientAvatar } from "@/components/gradient-avatar";
import { FormModal } from "@/components/ui/form-modal";
import { TextInput } from "@/components/ui/form";
import { api, errorText, type AdminUser } from "@/lib/api/client";
import { useUser } from "@/lib/session-context";
import { useUsersT } from "@/lib/i18n/dict/users";
import { AccountFormModal, AccountStatusModal } from "./account-modals";

const PAGE_SIZE = 50;
const STAFF_ROLES: AdminUser["role"][] = ["operator", "admin"];

type StatusFilter = "all" | "active" | "inactive";

export default function UsersPage() {
  const t = useUsersT();
  const sessionUser = useUser();
  const [rows, setRows] = useState<AdminUser[] | null>(null);
  const [total, setTotal] = useState(0);
  const [loadFailed, setLoadFailed] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [moreFailed, setMoreFailed] = useState("");
  const [reloadKey, setReloadKey] = useState(0);
  const [status, setStatus] = useState<StatusFilter>("all");
  const [search, setSearch] = useState("");
  const [query, setQuery] = useState("");
  const [formOpen, setFormOpen] = useState(false);
  const [editTarget, setEditTarget] = useState<AdminUser | null>(null);
  const [statusTarget, setStatusTarget] = useState<AdminUser | null>(null);
  const [detail, setDetail] = useState<AdminUser | null>(null);

  const roleLabel = (role: AdminUser["role"]) => (role === "admin" ? t.roleAdmin : t.roleOperator);
  const activeFilter = status === "all" ? undefined : status === "active";

  useEffect(() => {
    let cancelled = false;
    api.adminUsers
      .list({ role: "staff", q: query, active: activeFilter, limit: PAGE_SIZE })
      .then((page) => {
        if (cancelled) return;
        setRows(page.users);
        setTotal(page.total);
      })
      .catch(() => {
        if (!cancelled) setLoadFailed(true);
      });
    return () => {
      cancelled = true;
    };
  }, [query, activeFilter, reloadKey]);

  const reload = () => {
    setRows(null);
    setLoadFailed(false);
    setMoreFailed("");
    setReloadKey((key) => key + 1);
  };

  const loadMore = async () => {
    setLoadingMore(true);
    setMoreFailed("");
    try {
      const page = await api.adminUsers.list({
        role: "staff",
        q: query,
        active: activeFilter,
        limit: PAGE_SIZE,
        offset: rows?.length ?? 0,
      });
      setRows((prev) => {
        const seen = new Set((prev ?? []).map((row) => row.id));
        return [...(prev ?? []), ...page.users.filter((row) => !seen.has(row.id))];
      });
      setTotal(page.total);
    } catch (err) {
      setMoreFailed(errorText(err, t.loadErrorDescription));
    } finally {
      setLoadingMore(false);
    }
  };

  const replaceRow = (user: AdminUser) => {
    setRows((prev) => prev?.map((row) => (row.id === user.id ? user : row)) ?? prev);
    setDetail((prev) => (prev?.id === user.id ? user : prev));
  };

  const openCreate = () => {
    setEditTarget(null);
    setFormOpen(true);
  };

  const openEdit = (user: AdminUser) => {
    setDetail(null);
    setEditTarget(user);
    setFormOpen(true);
  };

  const statusChip = (user: AdminUser) => (
    <StatusChip tone={user.is_active ? "success" : "neutral"} dot>
      {user.is_active ? t.statusActive : t.statusInactive}
    </StatusChip>
  );

  const columns: Column<AdminUser>[] = [
    {
      key: "name",
      header: t.colName,
      render: (u) => (
        <div className="flex items-center gap-2.5">
          <GradientAvatar user={u} size="sm" />
          <div>
            <p className="font-medium text-foreground">{u.name}</p>
            <p className="text-label text-muted">{u.email ?? "—"}</p>
          </div>
        </div>
      ),
    },
    {
      key: "role",
      header: t.colRole,
      render: (u) => <StatusChip tone={u.role === "admin" ? "accent" : "neutral"}>{roleLabel(u.role)}</StatusChip>,
    },
    { key: "status", header: t.colStatus, render: statusChip },
    { key: "createdAt", header: t.colCreated, render: (u) => new Date(u.created_at).toLocaleDateString() },
    {
      key: "actions",
      header: "",
      headerClassName: "text-right",
      className: "text-right",
      render: (u) => (
        <div className="flex justify-end gap-1" onClick={(e) => e.stopPropagation()}>
          <Button variant="ghost" size="sm" isIconOnly aria-label={t.editAction} onPress={() => openEdit(u)}>
            <Icon icon={Pencil} size="sm" />
          </Button>
          <Button
            variant="ghost"
            size="sm"
            isIconOnly
            className={u.is_active ? "text-danger" : undefined}
            aria-label={u.is_active ? t.deactivateConfirm : t.reactivateConfirm}
            isDisabled={u.id === sessionUser.id}
            onPress={() => setStatusTarget(u)}
          >
            <Icon icon={Power} size="sm" />
          </Button>
        </div>
      ),
    },
  ];

  return (
    <RequireRole roles={["admin"]} title={t.pageTitle} deniedMessage={t.deniedMessage}>
      <AppShell
        title={t.pageTitle}
        description={rows === null ? t.pageDescription : t.accountCount(total)}
        actions={
          <Button variant="primary" size="sm" onPress={openCreate}>
            <Icon icon={Plus} size="sm" /> {t.addAccount}
          </Button>
        }
      >
        <div className="mb-4 flex flex-wrap items-end justify-between gap-3">
          <form
            className="flex w-full max-w-md flex-wrap items-end gap-2"
            onSubmit={(event) => {
              event.preventDefault();
              setRows(null);
              setLoadFailed(false);
              setQuery(search.trim());
            }}
          >
            <TextInput
              className="w-full max-w-sm"
              placeholder={t.searchPlaceholder}
              value={search}
              onChange={(event) => setSearch(event.target.value)}
            />
            <Button variant="secondary" size="sm" type="submit">
              <Icon icon={Search} size="xs" /> {t.searchAction}
            </Button>
            {query !== "" && (
              <Button
                variant="ghost"
                size="sm"
                onPress={() => {
                  setSearch("");
                  setRows(null);
                  setQuery("");
                }}
              >
                {t.searchClear}
              </Button>
            )}
          </form>
          <FilterPills
            ariaLabel={t.filterLabel}
            value={status}
            onChange={(next) => {
              setRows(null);
              setLoadFailed(false);
              setStatus(next as StatusFilter);
            }}
            options={[
              { id: "all", label: t.filterAll },
              { id: "active", label: t.statusActive },
              { id: "inactive", label: t.statusInactive },
            ]}
          />
        </div>

        {loadFailed ? (
          <EmptyState
            icon={CloudOff}
            title={t.loadErrorTitle}
            description={t.loadErrorDescription}
            action={
              <Button variant="primary" size="sm" onPress={reload}>
                {t.retry}
              </Button>
            }
          />
        ) : rows === null ? (
          <p className="py-14 text-center text-caption text-muted">{t.loading}</p>
        ) : rows.length === 0 ? (
          <EmptyState
            icon={Users}
            title={query === "" && status === "all" ? t.emptyTitle : t.noMatchTitle}
            description={query === "" && status === "all" ? t.emptyDescription : t.noMatchDescription}
          />
        ) : (
          <>
            <DataTable columns={columns} rows={rows} rowKey={(u) => u.id} onRowClick={setDetail} />
            {moreFailed !== "" && <p className="mt-3 text-center text-caption text-danger">{moreFailed}</p>}
            {rows.length < total && (
              <div className="mt-4 flex justify-center">
                <Button variant="secondary" size="sm" onPress={loadMore} isDisabled={loadingMore}>
                  {loadingMore ? t.loadingMore : t.loadMore}
                </Button>
              </div>
            )}
          </>
        )}

        <AccountFormModal
          isOpen={formOpen}
          onOpenChange={setFormOpen}
          target={editTarget}
          roles={STAFF_ROLES}
          onSaved={(user, created) => (created ? reload() : replaceRow(user))}
        />
        <AccountStatusModal target={statusTarget} onClose={() => setStatusTarget(null)} onDone={replaceRow} />

        <FormModal
          isOpen={!!detail}
          onOpenChange={(open) => !open && setDetail(null)}
          title={detail?.name ?? ""}
          description={detail?.email ?? ""}
          footer={
            <>
              {detail && (
                <Button variant="secondary" size="sm" onPress={() => openEdit(detail)}>
                  <Icon icon={Pencil} size="xs" /> {t.editAction}
                </Button>
              )}
              <Button variant="ghost" onPress={() => setDetail(null)}>
                {t.close}
              </Button>
            </>
          }
        >
          {detail && (
            <>
              <div className="flex items-center gap-3.5">
                <GradientAvatar user={detail} size="lg" />
                <div>
                  <p className="font-display text-title text-foreground">{detail.name}</p>
                  <p className="text-label text-muted">{detail.email ?? "—"}</p>
                </div>
              </div>
              <DetailGrid className="border-t border-border pt-5">
                <DetailItem label={t.colRole}>
                  <StatusChip tone={detail.role === "admin" ? "accent" : "neutral"}>{roleLabel(detail.role)}</StatusChip>
                </DetailItem>
                <DetailItem label={t.colStatus}>{statusChip(detail)}</DetailItem>
                <DetailItem label={t.detailPhone}>{detail.phone ?? "—"}</DetailItem>
                <DetailItem label={t.detailEmailVerified}>{detail.email_verified ? t.yes : t.no}</DetailItem>
                <DetailItem label={t.colCreated}>{new Date(detail.created_at).toLocaleDateString()}</DetailItem>
              </DetailGrid>
              {detail.id === sessionUser.id && <NoteBox>{t.selfNote}</NoteBox>}
            </>
          )}
        </FormModal>
      </AppShell>
    </RequireRole>
  );
}
