-- P09: kecermatan item packages. A package is a named element pool; every change writes a new frozen
-- revision so a test (P10) can pin the exact pool it was built from. tenant_id stays NOT NULL: master
-- packages are out of the MVP (D17).

ALTER TABLE assets DROP CONSTRAINT assets_purpose_check;
ALTER TABLE assets ADD CONSTRAINT assets_purpose_check
    CHECK (purpose IN ('question_image', 'payment_proof', 'item_image'));

CREATE TABLE item_packages (
    id               uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id        uuid        NOT NULL REFERENCES tenants (id) ON DELETE RESTRICT,
    type             text        NOT NULL
        CONSTRAINT item_packages_type_check CHECK (type IN ('symbol', 'image', 'emoji', 'shape')),
    name             text        NOT NULL
        CONSTRAINT item_packages_name_check CHECK (char_length(btrim(name)) BETWEEN 1 AND 100),
    current_revision integer     NOT NULL DEFAULT 1 CHECK (current_revision >= 1),
    created_by       uuid,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT item_packages_tenant_id_key UNIQUE (tenant_id, id),
    CONSTRAINT item_packages_creator_tenant_fkey FOREIGN KEY (created_by, tenant_id)
        REFERENCES users (id, tenant_id) ON DELETE RESTRICT
);

CREATE UNIQUE INDEX item_packages_name_uniq ON item_packages (tenant_id, lower(btrim(name)));

CREATE TABLE item_package_revisions (
    package_id    uuid        NOT NULL REFERENCES item_packages (id) ON DELETE CASCADE,
    revision      integer     NOT NULL CHECK (revision >= 1),
    element_count integer     NOT NULL CHECK (element_count >= 0),
    created_by    uuid,
    created_at    timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (package_id, revision)
);

-- element_id is stable across revisions, so "remove this element" names the same thing in every one.
-- key is the dedup identity: the symbol itself, the emoji or shape id, or the image's SHA-256.
CREATE TABLE item_package_elements (
    package_id uuid    NOT NULL,
    revision   integer NOT NULL,
    element_id uuid    NOT NULL,
    position   integer NOT NULL CHECK (position >= 0),
    kind       text    NOT NULL CHECK (kind IN ('symbol', 'image', 'emoji', 'shape')),
    key        text    NOT NULL CHECK (key <> ''),
    value      text    NOT NULL DEFAULT '',
    label      text    NOT NULL DEFAULT '',
    asset_id   uuid,
    PRIMARY KEY (package_id, revision, element_id),
    CONSTRAINT item_package_elements_key_uniq UNIQUE (package_id, revision, key),
    CONSTRAINT item_package_elements_revision_fkey FOREIGN KEY (package_id, revision)
        REFERENCES item_package_revisions (package_id, revision) ON DELETE CASCADE,
    CONSTRAINT item_package_elements_asset_fkey FOREIGN KEY (asset_id) REFERENCES assets (id) ON DELETE RESTRICT,
    CONSTRAINT item_package_elements_asset_check CHECK ((kind = 'image') = (asset_id IS NOT NULL))
);

CREATE FUNCTION item_package_frozen() RETURNS trigger
    LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION '% is frozen once written', TG_TABLE_NAME USING ERRCODE = '23514';
END;
$$;

CREATE TRIGGER item_package_revisions_frozen
    BEFORE UPDATE ON item_package_revisions
    FOR EACH ROW EXECUTE FUNCTION item_package_frozen();

CREATE TRIGGER item_package_elements_frozen
    BEFORE UPDATE ON item_package_elements
    FOR EACH ROW EXECUTE FUNCTION item_package_frozen();

-- The seam P10 plugs into: one row per test that pins a package revision and the pool size it needs
-- (options_per_soal x item_length, FR-ADM-KEC-10). test_id has no foreign key yet because the kecermatan
-- test table does not exist; P10 adds it. The RESTRICT keys are what make a used package undeletable.
CREATE TABLE kecermatan_test_packages (
    test_id        uuid        NOT NULL,
    tenant_id      uuid        NOT NULL,
    package_id     uuid        NOT NULL,
    revision       integer     NOT NULL,
    required_count integer     NOT NULL CHECK (required_count >= 1),
    created_at     timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (test_id, package_id),
    CONSTRAINT kecermatan_test_packages_package_fkey FOREIGN KEY (tenant_id, package_id)
        REFERENCES item_packages (tenant_id, id) ON DELETE RESTRICT,
    CONSTRAINT kecermatan_test_packages_revision_fkey FOREIGN KEY (package_id, revision)
        REFERENCES item_package_revisions (package_id, revision) ON DELETE RESTRICT
);

CREATE INDEX kecermatan_test_packages_package_idx ON kecermatan_test_packages (package_id);
