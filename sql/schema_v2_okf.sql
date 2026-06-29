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
    `created_at` DATETIME NOT NULL,
    `updated_at` DATETIME NOT NULL,
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
    `link_count` INT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'Outgoing edges (denormalized for O(1) stats)',
    `backlink_count` INT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'Incoming edges (denormalized for O(1) stats)',
    `extra_json` JSON COMMENT 'Unknown frontmatter keys preserved per OKF §9',
    `content_hash` CHAR(64) NOT NULL DEFAULT '' COMMENT 'SHA-256 of the markdown body, for change detection',
    `created_at` DATETIME NOT NULL,
    `updated_at` DATETIME NOT NULL,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_bundle_rel_path` (`bundle_id`, `rel_path`(255)),
    KEY `idx_bundle_type` (`bundle_id`, `type`),
    KEY `idx_bundle_file` (`bundle_id`, `file_id`),
    KEY `idx_content_hash` (`content_hash`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='OKF materialized node index';

-- disk_okf_edge: one row per directed link between two OKF nodes. Written
-- atomically by WriteMarkdown's materialize step. dst_node_id is NULL when the
-- link target does not exist in the bundle (dead link); dst_exists mirrors
-- that as a boolean for cheap broken-link filtering. public_dir_id clusters a
-- bundle's graph slice on the composite indexes so 1-hop and reverse traversal
-- stay index-only.
CREATE TABLE IF NOT EXISTS `disk_okf_edge` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `public_dir_id` BIGINT UNSIGNED NOT NULL COMMENT 'Owner public directory (denormalized from bundle)',
    `src_node_id` BIGINT UNSIGNED NOT NULL COMMENT 'Source node (disk_okf_node.id)',
    `dst_node_id` BIGINT UNSIGNED NULL COMMENT 'Destination node (NULL when target missing)',
    `dst_rel_path` VARCHAR(1024) NOT NULL COMMENT 'Destination bundle-relative path as written in source',
    `dst_exists` TINYINT(1) NOT NULL DEFAULT 0 COMMENT '1 when dst_node_id is set, 0 for dead link',
    `link_text` VARCHAR(512) NOT NULL DEFAULT '' COMMENT 'Anchor text of the link',
    `src_line` INT NOT NULL DEFAULT 0 COMMENT '1-indexed line number of the link in the source body',
    `link_kind` VARCHAR(16) NOT NULL DEFAULT 'bundle' COMMENT 'bundle | external | anchor',
    `anchor` VARCHAR(128) NOT NULL DEFAULT '' COMMENT 'In-page anchor (for LinkKind=anchor)',
    `created_at` DATETIME NOT NULL,
    `updated_at` DATETIME NOT NULL,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_edge_src_line` (`src_node_id`, `dst_rel_path`, `src_line`),
    KEY `idx_edge_src` (`public_dir_id`, `src_node_id`),
    KEY `idx_edge_dst` (`public_dir_id`, `dst_node_id`),
    KEY `idx_edge_broken` (`public_dir_id`, `dst_exists`),
    KEY `idx_edge_relpath` (`public_dir_id`, `dst_rel_path`(255))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='OKF materialized edge graph';
