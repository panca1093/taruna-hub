-- Lossy by design: the text columns keep text, formulas, bold/italic and simple lists, but tables become " | " lines
-- and shapes and uploaded images disappear. Documents are dropped; so are the per-option docs below.
ALTER TABLE question_versions
    DROP COLUMN content_doc,
    DROP COLUMN explanation_doc;

UPDATE question_versions v
   SET options = COALESCE(
       (SELECT jsonb_agg(o.obj - 'doc' ORDER BY o.ord)
          FROM jsonb_array_elements(v.options) WITH ORDINALITY AS o(obj, ord)),
       '[]'::jsonb)
 WHERE jsonb_array_length(v.options) > 0;
