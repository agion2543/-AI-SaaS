CREATE DATABASE IF NOT EXISTS saas_billing DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
USE saas_billing;

CREATE TABLE IF NOT EXISTS users (
  id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  uuid VARCHAR(36) NOT NULL UNIQUE,
  email VARCHAR(120) NOT NULL UNIQUE,
  phone VARCHAR(30) NOT NULL UNIQUE,
  password_hash VARCHAR(255) NOT NULL,
  display_name VARCHAR(80) NOT NULL,
  status VARCHAR(20) NOT NULL DEFAULT 'active',
  member_level VARCHAR(30) NOT NULL DEFAULT 'free',
  current_package_id BIGINT UNSIGNED NULL,
  expired_at DATETIME NULL,
  remaining_quota INT NOT NULL DEFAULT 0,
  bound_devices TEXT NULL,
  auto_renew TINYINT(1) NOT NULL DEFAULT 0,
  reset_token VARCHAR(120) NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS verification_codes (
  id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  phone VARCHAR(30) NOT NULL,
  scene VARCHAR(40) NOT NULL,
  code VARCHAR(12) NOT NULL,
  user_id BIGINT UNSIGNED NULL,
  expires_at DATETIME NOT NULL,
  consumed_at DATETIME NULL,
  KEY idx_phone_scene (phone, scene)
);

CREATE TABLE IF NOT EXISTS admin_roles (
  id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  name VARCHAR(60) NOT NULL UNIQUE,
  code VARCHAR(60) NOT NULL UNIQUE,
  permissions TEXT NULL
);

CREATE TABLE IF NOT EXISTS admin_users (
  id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  username VARCHAR(60) NOT NULL UNIQUE,
  password_hash VARCHAR(255) NOT NULL,
  display_name VARCHAR(80) NOT NULL,
  status VARCHAR(20) NOT NULL DEFAULT 'active',
  role_id BIGINT UNSIGNED NOT NULL
);

CREATE TABLE IF NOT EXISTS membership_packages (
  id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  name VARCHAR(80) NOT NULL,
  code VARCHAR(40) NOT NULL UNIQUE,
  duration_days INT NOT NULL DEFAULT 0,
  price BIGINT NOT NULL DEFAULT 0,
  original_price BIGINT NOT NULL DEFAULT 0,
  quota INT NOT NULL DEFAULT 0,
  sort INT NOT NULL DEFAULT 0,
  status VARCHAR(20) NOT NULL DEFAULT 'published',
  is_lifetime TINYINT(1) NOT NULL DEFAULT 0,
  description TEXT NULL
);

CREATE TABLE IF NOT EXISTS orders (
  id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  order_no VARCHAR(50) NOT NULL UNIQUE,
  user_id BIGINT UNSIGNED NOT NULL,
  package_id BIGINT UNSIGNED NOT NULL,
  amount BIGINT NOT NULL DEFAULT 0,
  status VARCHAR(20) NOT NULL DEFAULT 'pending',
  payment_channel VARCHAR(40) NOT NULL DEFAULT '',
  auto_renew TINYINT(1) NOT NULL DEFAULT 0,
  paid_at DATETIME NULL,
  transaction_no VARCHAR(80) NOT NULL DEFAULT '',
  subscription_end_at DATETIME NULL
);

CREATE TABLE IF NOT EXISTS payment_records (
  id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  order_id BIGINT UNSIGNED NOT NULL,
  payment_channel VARCHAR(40) NOT NULL,
  transaction_no VARCHAR(80) NOT NULL UNIQUE,
  amount BIGINT NOT NULL DEFAULT 0,
  status VARCHAR(20) NOT NULL DEFAULT 'success',
  raw_payload TEXT NULL
);

CREATE TABLE IF NOT EXISTS card_codes (
  id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  batch_no VARCHAR(50) NOT NULL,
  code VARCHAR(64) NOT NULL UNIQUE,
  package_id BIGINT UNSIGNED NOT NULL,
  quota INT NOT NULL DEFAULT 0,
  status VARCHAR(20) NOT NULL DEFAULT 'unused',
  expired_at DATETIME NULL,
  redeemed_by BIGINT UNSIGNED NULL,
  redeemed_at DATETIME NULL
);

CREATE TABLE IF NOT EXISTS system_configs (
  id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  config_key VARCHAR(80) NOT NULL UNIQUE,
  config_value TEXT NULL,
  is_encrypted TINYINT(1) NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS usage_records (
  id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  user_id BIGINT UNSIGNED NOT NULL,
  scene VARCHAR(80) NOT NULL,
  description VARCHAR(255) NOT NULL,
  quota_used INT NOT NULL DEFAULT 0,
  device_id VARCHAR(100) NOT NULL DEFAULT ''
);

INSERT INTO admin_roles (id, name, code, permissions, created_at, updated_at)
VALUES (1, 'Super Admin', 'super_admin', 'dashboard,users,packages,orders,payments,cards,configs', NOW(), NOW())
ON DUPLICATE KEY UPDATE name = VALUES(name), permissions = VALUES(permissions);

INSERT INTO admin_users (id, username, password_hash, display_name, status, role_id, created_at, updated_at)
VALUES (1, 'admin', '$2a$10$cE8LFVvs5/26l6.4l8hE8.mn1sXSzBHRVnaItSN.g14.tmtjcl/5a', 'System Admin', 'active', 1, NOW(), NOW())
ON DUPLICATE KEY UPDATE display_name = VALUES(display_name), status = VALUES(status), role_id = VALUES(role_id);

INSERT INTO membership_packages (name, code, duration_days, price, original_price, quota, sort, status, is_lifetime, description, created_at, updated_at)
VALUES
('Day Pass', 'day', 1, 990, 1290, 10, 1, 'published', 0, 'Daily membership package', NOW(), NOW()),
('Week Pass', 'week', 7, 4990, 6990, 100, 2, 'published', 0, 'Weekly membership package', NOW(), NOW()),
('Month Pass', 'month', 30, 12900, 15900, 500, 3, 'published', 0, 'Monthly membership package', NOW(), NOW()),
('Year Pass', 'year', 365, 99900, 129900, 6000, 4, 'published', 0, 'Yearly membership package', NOW(), NOW()),
('Lifetime', 'lifetime', 0, 299900, 399900, 999999, 5, 'published', 1, 'Lifetime membership package', NOW(), NOW())
ON DUPLICATE KEY UPDATE price = VALUES(price), quota = VALUES(quota), description = VALUES(description);
