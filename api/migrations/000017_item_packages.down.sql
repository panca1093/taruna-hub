DROP TABLE IF EXISTS kecermatan_test_packages;
DROP TABLE IF EXISTS item_package_elements;
DROP TABLE IF EXISTS item_package_revisions;
DROP TABLE IF EXISTS item_packages;
DROP FUNCTION IF EXISTS item_package_frozen();
DELETE FROM assets WHERE purpose = 'item_image';
ALTER TABLE assets DROP CONSTRAINT assets_purpose_check;
ALTER TABLE assets ADD CONSTRAINT assets_purpose_check CHECK (purpose IN ('question_image', 'payment_proof'));
