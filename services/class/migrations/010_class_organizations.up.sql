-- 010: link a class to an organization (dashboard plan ORG-01, Phase 5).
-- NULL means a personal class; non-NULL makes organization roles (OrgAdmin/OrgTeacher/
-- OrgStudent) inherit onto this class through authz.Can.
ALTER TABLE classes ADD COLUMN IF NOT EXISTS org_id BIGINT NULL;

CREATE INDEX IF NOT EXISTS classes_org_id_idx ON classes(org_id);
