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
