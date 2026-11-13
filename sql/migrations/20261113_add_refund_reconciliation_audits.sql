-- Manual operator reconciliation audit. Each unknown reservation can have at most
-- one terminal transition; audit rows are immutable at the database layer.
CREATE TABLE IF NOT EXISTS refund_reconciliation_audits (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  refund_id BIGINT UNSIGNED NOT NULL,
  refund_no VARCHAR(60) NOT NULL,
  order_id BIGINT UNSIGNED NOT NULL,
  actor_id BIGINT UNSIGNED NOT NULL,
  actor_role VARCHAR(30) NOT NULL,
  old_status VARCHAR(20) NOT NULL,
  new_status VARCHAR(20) NOT NULL,
  amount BIGINT NOT NULL,
  provider_reference VARCHAR(120) NOT NULL,
  reason VARCHAR(1000) NOT NULL,
  created_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY idx_refund_reconciliation_transition (refund_id, old_status, new_status),
  KEY idx_refund_reconciliation_audits_refund_id (refund_id),
  KEY idx_refund_reconciliation_audits_order_id (order_id),
  KEY idx_refund_reconciliation_audits_actor_id (actor_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

DROP TRIGGER IF EXISTS trg_refund_reconciliation_audits_no_update;
DROP TRIGGER IF EXISTS trg_refund_reconciliation_audits_no_delete;
DELIMITER //
CREATE TRIGGER trg_refund_reconciliation_audits_no_update
BEFORE UPDATE ON refund_reconciliation_audits
FOR EACH ROW
BEGIN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'refund reconciliation audit is append-only';
END//
CREATE TRIGGER trg_refund_reconciliation_audits_no_delete
BEFORE DELETE ON refund_reconciliation_audits
FOR EACH ROW
BEGIN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'refund reconciliation audit is append-only';
END//
DELIMITER ;
