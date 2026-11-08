UPDATE website_content SET draft = draft - 'program_visibility';
UPDATE website_content SET published = published - 'program_visibility' WHERE published IS NOT NULL;
