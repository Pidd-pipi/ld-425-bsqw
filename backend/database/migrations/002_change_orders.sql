-- 002_change_orders.sql
-- 施工变更签证闭环：变更单表与项目汇总字段。
-- GORM AutoMigrate 会在应用启动时创建/更新表结构；
-- 本文件记录与实体对应的初始表结构，便于离线审阅与迁移追踪。

CREATE TABLE IF NOT EXISTS change_orders (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    project_id BIGINT UNSIGNED NOT NULL,
    node_id BIGINT UNSIGNED NOT NULL,
    amount DECIMAL(14,2) NOT NULL DEFAULT 0,
    schedule_impact INT NOT NULL DEFAULT 0,
    reason VARCHAR(500) NOT NULL,
    status VARCHAR(32) NOT NULL,
    applicant_id BIGINT UNSIGNED NOT NULL,
    reviewer_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
    review_comment VARCHAR(500),
    reviewed_at DATETIME(3) NULL,
    created_at DATETIME(3) NULL,
    updated_at DATETIME(3) NULL,
    INDEX idx_change_project (project_id),
    INDEX idx_change_project_node (project_id, node_id),
    INDEX idx_change_node (node_id),
    INDEX idx_change_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 同一节点待审批变更的唯一性由应用事务（节点行锁 + 状态校验）保证，
-- 以便在节点历史中保留多笔已批准/已驳回签证。

ALTER TABLE renovation_projects
    ADD COLUMN approved_changes DECIMAL(14,2) NOT NULL DEFAULT 0 AFTER contract_amount,
    ADD COLUMN schedule_delta INT NOT NULL DEFAULT 0 AFTER approved_changes;
