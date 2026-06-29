-- AgentDisk OKF v0.1 schema (P1)
--
-- This schema extends AgentDisk with Open Knowledge Format (OKF) bundle
-- registration and a materialized node index. It is independent of the
-- existing tables in schema.sql and can be rolled back on its own.
--
-- See: https://github.com/GoogleCloudPlatform/knowledge-catalog/blob/main/okf/SPEC.md

-- disk_okf_bundle: one row per registered OKF bundle. A bundle maps 1:1 to a
-- public directory; the unique constraint enforces "one bundle per directory".
CREATE TABLE IF NOT EXISTS `disk_okf_bundle` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `public_directory_id` BIGINT UNSIGNED NOT NULL COMMENT 'Owner public directory (disk_public_directory.id)',
    `okf_version` VARCHAR(16) NOT NULL DEFAULT '0.1' COMMENT 'OKF spec version declared by index.md',
    `root_index_file_id` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'disk_file.id of index.md (0 until first write)',
    `title` VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'Bundle title (from index.md frontmatter)',
    `description` TEXT COMMENT 'Bundle description (from index.md frontmatter)',
    `status` VARCHAR(16) NOT NULL DEFAULT 'active' COMMENT 'active | archived',
    `node_count` INT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'Denormalized count of materialized nodes',
    `edge_count` INT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'Reserved for future link-graph edge count',
    `created_at` DATETIME(3) NOT NULL,
    `updated_at` DATETIME(3) NOT NULL,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_public_directory_id` (`public_directory_id`),
    KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='OKF bundle registration';

-- disk_okf_node: one row per materialized OKF markdown document within a
-- bundle. The (bundle_id, rel_path) pair is unique so re-writing the same
-- relative path upserts the index entry.
CREATE TABLE IF NOT EXISTS `disk_okf_node` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `bundle_id` BIGINT UNSIGNED NOT NULL COMMENT 'Owner bundle (disk_okf_bundle.id)',
    `file_id` BIGINT UNSIGNED NOT NULL COMMENT 'disk_file.id backing this node',
    `rel_path` VARCHAR(1024) NOT NULL COMMENT 'Bundle-relative path, e.g. "concepts/gemma.md"',
    `type` VARCHAR(64) NOT NULL DEFAULT '' COMMENT 'OKF type (frontmatter "type")',
    `title` VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'Node title (frontmatter "title")',
    `description` TEXT COMMENT 'Node description (frontmatter "description")',
    `tags_json` JSON COMMENT 'Serialized []string of frontmatter "tags"',
    `timestamp` VARCHAR(32) NOT NULL DEFAULT '' COMMENT 'ISO 8601 timestamp (frontmatter "timestamp")',
    `has_broken_link` TINYINT(1) NOT NULL DEFAULT 0 COMMENT 'True if the body references a missing node',
    `extra_json` JSON COMMENT 'Unknown frontmatter keys preserved per OKF §9',
    `content_hash` CHAR(64) NOT NULL DEFAULT '' COMMENT 'SHA-256 of the markdown body, for change detection',
    `created_at` DATETIME(3) NOT NULL,
    `updated_at` DATETIME(3) NOT NULL,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_bundle_rel_path` (`bundle_id`, `rel_path`(255)),
    KEY `idx_bundle_type` (`bundle_id`, `type`),
    KEY `idx_bundle_file` (`bundle_id`, `file_id`),
    KEY `idx_content_hash` (`content_hash`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='OKF materialized node index';
