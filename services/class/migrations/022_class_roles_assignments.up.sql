CREATE TABLE IF NOT EXISTS class_role_definitions (
 class_id BIGINT NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
 key TEXT NOT NULL,
 name VARCHAR(80) NOT NULL,
 description VARCHAR(500) NOT NULL DEFAULT '',
 allow_actions JSONB NOT NULL DEFAULT '[]',
 deny_actions JSONB NOT NULL DEFAULT '[]',
 PRIMARY KEY(class_id,key)
);
CREATE TABLE IF NOT EXISTS class_member_access (
 class_id BIGINT NOT NULL,
 user_id BIGINT NOT NULL,
 role_keys JSONB NOT NULL DEFAULT '[]',
 PRIMARY KEY(class_id,user_id),
 FOREIGN KEY(class_id,user_id) REFERENCES class_members(class_id,user_id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS class_assignments (
 id BIGSERIAL PRIMARY KEY,
 class_id BIGINT NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
 title VARCHAR(200) NOT NULL,
 instructions TEXT NOT NULL DEFAULT '',
 due_at TIMESTAMPTZ,
 status TEXT NOT NULL CHECK(status IN ('draft','published')),
 created_by BIGINT NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS class_assignments_class_idx ON class_assignments(class_id,created_at DESC);
CREATE TABLE IF NOT EXISTS class_assignment_files (
 id BIGSERIAL PRIMARY KEY,
 assignment_id BIGINT NOT NULL REFERENCES class_assignments(id) ON DELETE CASCADE,
 name VARCHAR(255) NOT NULL,
 content_type TEXT NOT NULL,
 data BYTEA NOT NULL CHECK(octet_length(data) BETWEEN 1 AND 20971520),
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS class_assignment_files_parent_idx ON class_assignment_files(assignment_id);
