-- P22: who did what to which account or grant (D14: accounts are deactivated, never deleted).
CREATE TABLE audit_events (
    id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   uuid        NOT NULL REFERENCES tenants (id) ON DELETE RESTRICT,
    actor_id    uuid        NOT NULL,
    actor_role  text        NOT NULL CHECK (actor_role IN ('admin', 'operator')),
    action      text        NOT NULL CHECK (action IN (
        'user.create', 'user.update', 'user.deactivate', 'user.reactivate', 'grant.manual')),
    target_type text        NOT NULL CHECK (target_type IN ('user', 'enrollment')),
    target_id   uuid        NOT NULL,
    reason      text        NOT NULL DEFAULT '',
    detail      jsonb       NOT NULL DEFAULT '{}',
    created_at  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT audit_events_actor_fkey FOREIGN KEY (actor_id, tenant_id)
        REFERENCES users (id, tenant_id) ON DELETE RESTRICT
);

CREATE INDEX audit_events_tenant_idx ON audit_events (tenant_id, created_at DESC, id DESC);
CREATE INDEX audit_events_target_idx ON audit_events (tenant_id, target_id, created_at DESC);
