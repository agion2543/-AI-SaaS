CREATE TABLE IF NOT EXISTS store_products (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  store_id BIGINT UNSIGNED NOT NULL,
  name VARCHAR(120) NOT NULL,
  price BIGINT NOT NULL DEFAULT 0 COMMENT '价格，单位：分',
  description VARCHAR(500) NULL,
  image_url VARCHAR(500) NULL,
  status ENUM('active', 'inactive') NOT NULL DEFAULT 'active',
  sort INT NOT NULL DEFAULT 100,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  deleted_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  INDEX idx_store_products_store_id (store_id),
  INDEX idx_store_products_status (status),
  INDEX idx_store_products_sort (sort),
  INDEX idx_store_products_deleted_at (deleted_at),
  CONSTRAINT fk_store_products_store
    FOREIGN KEY (store_id) REFERENCES stores(id)
    ON UPDATE CASCADE
    ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
