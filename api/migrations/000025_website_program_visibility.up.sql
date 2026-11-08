-- P11b follow-up: one show/hide flag per fixed program slot, shared by both locales. Existing content keeps
-- every slot shown; the draft and the published snapshot are backfilled without touching revisions or history.
UPDATE website_content
   SET draft = draft || jsonb_build_object('program_visibility', jsonb_build_object('tni', true, 'polri', true, 'kedinasan', true))
 WHERE NOT draft ? 'program_visibility';

UPDATE website_content
   SET published = published || jsonb_build_object('program_visibility', jsonb_build_object('tni', true, 'polri', true, 'kedinasan', true))
 WHERE published IS NOT NULL AND NOT published ? 'program_visibility';
