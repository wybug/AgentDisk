-- AgentDisk Database Schema
-- MySQL 8.0+

CREATE DATABASE IF NOT EXISTS `agentdisk` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
USE `agentdisk`;

-- 1. 用户云盘配额表
CREATE TABLE `user_disk` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `user_id` VARCHAR(64) NOT NULL COMMENT '用户ID',
    `total_quota` BIGINT NOT NULL DEFAULT 10737418240 COMMENT '总配额(字节), 默认10GB',
    `used_quota` BIGINT NOT NULL DEFAULT 0 COMMENT '已用配额(字节)',
    `root_folder` VARCHAR(128) DEFAULT NULL COMMENT '根目录路径',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='用户云盘配额';

-- 2. 目录结构表
CREATE TABLE `disk_folder` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `user_id` VARCHAR(64) NOT NULL COMMENT '所属用户ID',
    `parent_id` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '父目录ID, 0为根目录',
    `folder_name` VARCHAR(255) NOT NULL COMMENT '目录名',
    `full_path` VARCHAR(1024) NOT NULL COMMENT '完整路径',
    `sort_order` INT NOT NULL DEFAULT 0 COMMENT '排序权重',
    `is_deleted` TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否已删除',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_user_id` (`user_id`),
    KEY `idx_parent_id` (`parent_id`),
    KEY `idx_user_parent` (`user_id`, `parent_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='目录结构';

-- 3. 文件元数据表
CREATE TABLE `disk_file` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `user_id` VARCHAR(64) NOT NULL COMMENT '所属用户ID',
    `folder_id` BIGINT UNSIGNED NOT NULL COMMENT '所属目录ID',
    `file_name` VARCHAR(255) NOT NULL COMMENT '文件名',
    `file_size` BIGINT NOT NULL DEFAULT 0 COMMENT '文件大小(字节)',
    `file_type` VARCHAR(32) DEFAULT NULL COMMENT '文件扩展名',
    `oss_key` VARCHAR(1024) NOT NULL COMMENT 'OSS存储Key',
    `md5` VARCHAR(32) DEFAULT NULL COMMENT '文件MD5',
    `version` INT NOT NULL DEFAULT 1 COMMENT '当前版本号',
    `is_deleted` TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否已删除',
    `source_agent` VARCHAR(64) DEFAULT NULL COMMENT '来源智能体ID',
    `source_agent_group` VARCHAR(64) DEFAULT NULL COMMENT '来源智能体组ID',
    `is_artifact` TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否为智能体产物',
    `tags` VARCHAR(1024) DEFAULT NULL COMMENT '标签(逗号分隔)',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_user_id` (`user_id`),
    KEY `idx_folder_id` (`folder_id`),
    KEY `idx_user_folder` (`user_id`, `folder_id`),
    KEY `idx_is_deleted` (`is_deleted`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='文件元数据';

-- 4. 智能体权限表
CREATE TABLE `disk_permission` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `user_id` VARCHAR(64) NOT NULL COMMENT '资源所有者ID',
    `agent_id` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '被授权智能体ID',
    `agent_group_id` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '被授权智能体组ID',
    `resource_id` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '资源ID（路径授权时为0）',
    `res_type` VARCHAR(16) NOT NULL DEFAULT '' COMMENT '资源类型: file/folder',
    `resource_path` VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '路径模式，支持通配符',
    `permission` VARCHAR(16) NOT NULL COMMENT '权限: owner/read/write/delete',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_user_id` (`user_id`),
    KEY `idx_agent` (`agent_id`, `agent_group_id`),
    KEY `idx_resource` (`resource_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='智能体权限';

-- 5. 版本快照表
CREATE TABLE `disk_file_version` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `file_id` BIGINT UNSIGNED NOT NULL COMMENT '文件ID',
    `user_id` VARCHAR(64) NOT NULL COMMENT '操作者ID',
    `version` INT NOT NULL COMMENT '版本号',
    `oss_key` VARCHAR(1024) NOT NULL COMMENT 'OSS存储Key',
    `file_size` BIGINT NOT NULL COMMENT '文件大小',
    `md5` VARCHAR(32) DEFAULT NULL COMMENT '文件MD5',
    `snapshot_by` VARCHAR(64) DEFAULT NULL COMMENT '快照创建者',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_file_id` (`file_id`),
    KEY `idx_user_id` (`user_id`),
    KEY `idx_file_version` (`file_id`, `version`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='版本快照';

-- 6. 回收站表
CREATE TABLE `disk_recycle_bin` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `user_id` VARCHAR(64) NOT NULL COMMENT '所属用户ID',
    `resource_id` BIGINT UNSIGNED NOT NULL COMMENT '原资源ID',
    `res_type` VARCHAR(16) NOT NULL COMMENT '资源类型: file/folder',
    `res_name` VARCHAR(255) NOT NULL COMMENT '资源名称',
    `original_path` VARCHAR(1024) DEFAULT NULL COMMENT '原始路径',
    `deleted_by` VARCHAR(64) DEFAULT NULL COMMENT '删除操作者',
    `expire_at` DATETIME NOT NULL COMMENT '过期时间(自动清除)',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_user_id` (`user_id`),
    KEY `idx_expire_at` (`expire_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='回收站';

-- 7. 标签字典表
CREATE TABLE `disk_tag` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `user_id` VARCHAR(64) NOT NULL COMMENT '所属用户ID',
    `tag_name` VARCHAR(64) NOT NULL COMMENT '标签名',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_user_id` (`user_id`),
    UNIQUE KEY `uk_user_tag` (`user_id`, `tag_name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='标签字典';

-- 标签-文件关联表
CREATE TABLE `disk_tag_relation` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `tag_id` BIGINT UNSIGNED NOT NULL COMMENT '标签ID',
    `file_id` BIGINT UNSIGNED NOT NULL COMMENT '文件ID',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_tag_id` (`tag_id`),
    KEY `idx_file_id` (`file_id`),
    UNIQUE KEY `uk_tag_file` (`tag_id`, `file_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='标签文件关联';

-- 8. 外链分享表
CREATE TABLE `disk_share` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `user_id` VARCHAR(64) NOT NULL COMMENT '分享者ID',
    `resource_id` BIGINT UNSIGNED NOT NULL COMMENT '资源ID',
    `res_type` VARCHAR(16) NOT NULL COMMENT '资源类型: file/folder',
    `share_code` VARCHAR(32) NOT NULL COMMENT '分享码',
    `extract_code` VARCHAR(8) DEFAULT NULL COMMENT '提取码',
    `max_visit` INT NOT NULL DEFAULT -1 COMMENT '最大访问次数, -1不限',
    `visit_count` INT NOT NULL DEFAULT 0 COMMENT '已访问次数',
    `expire_at` DATETIME NOT NULL COMMENT '过期时间',
    `is_active` TINYINT(1) NOT NULL DEFAULT 1 COMMENT '是否有效',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_share_code` (`share_code`),
    KEY `idx_user_id` (`user_id`),
    KEY `idx_expire_at` (`expire_at`),
    KEY `idx_is_active` (`is_active`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='外链分享';

-- 分享访问日志
CREATE TABLE `disk_share_access_log` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `share_id` BIGINT UNSIGNED NOT NULL COMMENT '分享ID',
    `visitor_ip` VARCHAR(45) DEFAULT NULL COMMENT '访问IP',
    `user_agent` VARCHAR(512) DEFAULT NULL COMMENT '浏览器UA',
    `action` VARCHAR(32) DEFAULT NULL COMMENT '访问行为',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_share_id` (`share_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='分享访问日志';

-- 审计日志表
CREATE TABLE `disk_audit_log` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `user_id` VARCHAR(64) NOT NULL COMMENT '操作者ID',
    `action` VARCHAR(32) NOT NULL COMMENT '操作类型',
    `resource_type` VARCHAR(16) DEFAULT NULL COMMENT '资源类型',
    `resource_id` BIGINT UNSIGNED DEFAULT NULL COMMENT '资源ID',
    `detail` TEXT DEFAULT NULL COMMENT '操作详情',
    `ip` VARCHAR(45) DEFAULT NULL COMMENT '操作IP',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_user_id` (`user_id`),
    KEY `idx_action` (`action`),
    KEY `idx_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='审计日志';
