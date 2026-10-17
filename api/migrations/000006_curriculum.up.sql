-- P05: programs become tenant-owned and gain subjects and chapters. The old global rows were dev
-- seed only, so they go; master (tenant_id NULL) is deferred.

DELETE FROM programs;

ALTER TABLE programs
    ADD COLUMN tenant_id   uuid NOT NULL REFERENCES tenants (id),
    ADD COLUMN description text NOT NULL DEFAULT '',
    ADD COLUMN position    int  NOT NULL DEFAULT 0,
    DROP CONSTRAINT programs_code_key,
    ADD CONSTRAINT programs_tenant_code_key UNIQUE (tenant_id, code),
    -- Makes (id, tenant_id) referenceable, so a child can never point at another tenant's parent.
    ADD CONSTRAINT programs_id_tenant_key UNIQUE (id, tenant_id);

CREATE TABLE subjects (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id  uuid        NOT NULL REFERENCES tenants (id),
    program_id uuid        NOT NULL,
    name       text        NOT NULL,
    position   int         NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT subjects_id_tenant_key UNIQUE (id, tenant_id),
    -- Composite target: an existing id from another tenant fails the match instead of passing.
    CONSTRAINT subjects_program_tenant_fkey FOREIGN KEY (program_id, tenant_id)
        REFERENCES programs (id, tenant_id) ON DELETE RESTRICT
);

CREATE UNIQUE INDEX subjects_program_name_key ON subjects (program_id, lower(name));

CREATE TABLE chapters (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id  uuid        NOT NULL REFERENCES tenants (id),
    subject_id uuid        NOT NULL,
    name       text        NOT NULL,
    position   int         NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT chapters_subject_tenant_fkey FOREIGN KEY (subject_id, tenant_id)
        REFERENCES subjects (id, tenant_id) ON DELETE RESTRICT
);

CREATE UNIQUE INDEX chapters_subject_name_key ON chapters (subject_id, lower(name));
