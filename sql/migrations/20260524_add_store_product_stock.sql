ALTER TABLE store_products
  ADD COLUMN stock INT NULL COMMENT '可售库存，NULL 表示不限库存，0 表示售罄' AFTER status;
