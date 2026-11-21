ALTER TABLE question_versions
    DROP COLUMN content_doc,
    DROP COLUMN explanation_doc;

UPDATE question_versions v
   SET options = COALESCE(
       (SELECT jsonb_agg(o.obj - 'doc' ORDER BY o.ord)
          FROM jsonb_array_elements(v.options) WITH ORDINALITY AS o(obj, ord)),
       '[]'::jsonb)
 WHERE jsonb_array_length(v.options) > 0;
