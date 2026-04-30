USE saas_billing;

START TRANSACTION;

CREATE TABLE IF NOT EXISTS merchants (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  name VARCHAR(120) NOT NULL,
  contact_phone VARCHAR(30) NULL,
  contact_email VARCHAR(120) NULL,
  status ENUM('pending', 'active', 'suspended') NOT NULL DEFAULT 'pending',
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  UNIQUE KEY uk_merchants_name (name),
  KEY idx_merchants_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

ALTER TABLE users
  ADD COLUMN IF NOT EXISTS merchant_id BIGINT UNSIGNED NULL AFTER phone,
  ADD COLUMN IF NOT EXISTS role ENUM('super_admin', 'merchant_admin', 'customer') NOT NULL DEFAULT 'customer' AFTER merchant_id;

SET @merchant_fk_exists := (
  SELECT COUNT(1)
  FROM information_schema.TABLE_CONSTRAINTS
  WHERE CONSTRAINT_SCHEMA = DATABASE()
    AND TABLE_NAME = 'users'
    AND CONSTRAINT_NAME = 'fk_users_merchant'
    AND CONSTRAINT_TYPE = 'FOREIGN KEY'
);

SET @merchant_fk_sql := IF(
  @merchant_fk_exists = 0,
  'ALTER TABLE users ADD CONSTRAINT fk_users_merchant FOREIGN KEY (merchant_id) REFERENCES merchants(id) ON UPDATE CASCADE ON DELETE SET NULL',
  'SELECT 1'
);

PREPARE merchant_fk_stmt FROM @merchant_fk_sql;
EXECUTE merchant_fk_stmt;
DEALLOCATE PREPARE merchant_fk_stmt;

SET @merchant_idx_exists := (
  SELECT COUNT(1)
  FROM information_schema.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE()
    AND TABLE_NAME = 'users'
    AND INDEX_NAME = 'idx_users_merchant_id'
);

SET @merchant_idx_sql := IF(
  @merchant_idx_exists = 0,
  'ALTER TABLE users ADD INDEX idx_users_merchant_id (merchant_id)',
  'SELECT 1'
);

PREPARE merchant_idx_stmt FROM @merchant_idx_sql;
EXECUTE merchant_idx_stmt;
DEALLOCATE PREPARE merchant_idx_stmt;

SET @role_idx_exists := (
  SELECT COUNT(1)
  FROM information_schema.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE()
    AND TABLE_NAME = 'users'
    AND INDEX_NAME = 'idx_users_role'
);

SET @role_idx_sql := IF(
  @role_idx_exists = 0,
  'ALTER TABLE users ADD INDEX idx_users_role (role)',
  'SELECT 1'
);

PREPARE role_idx_stmt FROM @role_idx_sql;
EXECUTE role_idx_stmt;
DEALLOCATE PREPARE role_idx_stmt;

INSERT INTO merchants (id, name, contact_phone, contact_email, status, created_at, updated_at)
VALUES (1, '平台方', NULL, NULL, 'active', NOW(3), NOW(3))
ON DUPLICATE KEY UPDATE
  name = VALUES(name),
  contact_phone = VALUES(contact_phone),
  contact_email = VALUES(contact_email),
  status = VALUES(status),
  updated_at = VALUES(updated_at);

UPDATE users
SET role = 'customer',
    merchant_id = NULL
WHERE role <> 'customer'
   OR role IS NULL
   OR merchant_id IS NOT NULL;

/*
  NOTE:
  The current project stores the platform admin account in admin_users/admin_roles,
  not in users. So there is no existing users row for username "admin" to update here.
  The admin account remains controlled by admin_users + admin_roles(super_admin).
  If you later migrate admin identities into users, set that specific row to:
    role = 'super_admin'
    merchant_id = NULL
*/

COMMIT;
