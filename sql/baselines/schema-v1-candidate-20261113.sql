-- NON-PRODUCTION CANDIDATE SNAPSHOT; NOT AN OFFICIAL INSTALL OR MIGRATION MECHANISM.
-- Generated: 2026-11-13. MySQL: 8.0.46-0ubuntu0.24.04.5.
-- Source basis: repository HEAD 15274c8 plus the current local working-tree changes at generation time.
-- Built from the current internal/bootstrap/app.go autoMigrate model list against a blank isolated MySQL database;
-- the existing sql/migrations/20261113_add_refund_reconciliation_audits.sql was then applied for its required triggers.
-- Schema only: application seedInitialData was not run; no synthetic or real row data is included.
-- This is a candidate/reference snapshot, not a release, not a validated historical-old-database upgrade baseline,
-- and not yet the authoritative fresh-install or migration runner. Legacy SQL upgrades remain unsupported until an
-- actual historical schema baseline is available. Review and explicitly bless/version before operational use.
-- Round-trip verification: imported into a second clean isolated MySQL instance; tables, columns, indexes,
-- foreign keys and triggers matched source catalog checks.


/*!40101 SET @OLD_CHARACTER_SET_CLIENT=@@CHARACTER_SET_CLIENT */;
/*!40101 SET @OLD_CHARACTER_SET_RESULTS=@@CHARACTER_SET_RESULTS */;
/*!40101 SET @OLD_COLLATION_CONNECTION=@@COLLATION_CONNECTION */;
/*!50503 SET NAMES utf8mb4 */;
/*!40103 SET @OLD_TIME_ZONE=@@TIME_ZONE */;
/*!40103 SET TIME_ZONE='+00:00' */;
/*!40014 SET @OLD_UNIQUE_CHECKS=@@UNIQUE_CHECKS, UNIQUE_CHECKS=0 */;
/*!40014 SET @OLD_FOREIGN_KEY_CHECKS=@@FOREIGN_KEY_CHECKS, FOREIGN_KEY_CHECKS=0 */;
/*!40101 SET @OLD_SQL_MODE=@@SQL_MODE, SQL_MODE='NO_AUTO_VALUE_ON_ZERO' */;
/*!40111 SET @OLD_SQL_NOTES=@@SQL_NOTES, SQL_NOTES=0 */;
DROP TABLE IF EXISTS `admin_roles`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `admin_roles` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `name` varchar(60) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `code` varchar(60) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `permissions` text COLLATE utf8mb4_unicode_ci,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_admin_roles_name` (`name`),
  UNIQUE KEY `idx_admin_roles_code` (`code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
DROP TABLE IF EXISTS `admin_users`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `admin_users` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `username` varchar(60) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `password_hash` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `display_name` varchar(80) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `status` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT 'active',
  `role_id` bigint unsigned DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_admin_users_username` (`username`),
  KEY `fk_admin_users_role` (`role_id`),
  CONSTRAINT `fk_admin_users_role` FOREIGN KEY (`role_id`) REFERENCES `admin_roles` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
DROP TABLE IF EXISTS `audit_logs`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `audit_logs` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `actor_id` bigint unsigned DEFAULT NULL,
  `actor_type` varchar(30) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `actor_name` varchar(120) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `action` varchar(80) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `target_type` varchar(60) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `target_id` bigint unsigned DEFAULT NULL,
  `target_name` varchar(160) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `merchant_id` bigint unsigned DEFAULT NULL,
  `ip` varchar(80) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `user_agent` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `detail` json DEFAULT NULL,
  `review_status` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT 'pending',
  `review_remark` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `reviewed_by` bigint unsigned DEFAULT NULL,
  `reviewed_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_audit_logs_reviewed_by` (`reviewed_by`),
  KEY `idx_audit_logs_actor_id` (`actor_id`),
  KEY `idx_audit_logs_actor_type` (`actor_type`),
  KEY `idx_audit_logs_action` (`action`),
  KEY `idx_audit_logs_target_type` (`target_type`),
  KEY `idx_audit_logs_target_id` (`target_id`),
  KEY `idx_audit_logs_merchant_id` (`merchant_id`),
  KEY `idx_audit_logs_review_status` (`review_status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
DROP TABLE IF EXISTS `card_codes`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `card_codes` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `batch_no` varchar(50) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `code` varchar(64) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `package_id` bigint unsigned DEFAULT NULL,
  `quota` bigint DEFAULT NULL,
  `status` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT 'unused',
  `expired_at` datetime(3) DEFAULT NULL,
  `redeemed_by` bigint unsigned DEFAULT NULL,
  `redeemed_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_card_codes_code` (`code`),
  KEY `idx_card_codes_batch_no` (`batch_no`),
  KEY `fk_card_codes_package` (`package_id`),
  CONSTRAINT `fk_card_codes_package` FOREIGN KEY (`package_id`) REFERENCES `membership_packages` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
DROP TABLE IF EXISTS `customer_leads`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `customer_leads` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `merchant_id` bigint unsigned NOT NULL,
  `store_id` bigint unsigned NOT NULL,
  `customer_name` varchar(60) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `customer_phone` varchar(30) COLLATE utf8mb4_unicode_ci NOT NULL,
  `message` varchar(500) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `follow_up_note` varchar(500) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `source` varchar(40) COLLATE utf8mb4_unicode_ci DEFAULT 'store_qr',
  `status` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT 'new',
  PRIMARY KEY (`id`),
  KEY `idx_customer_leads_merchant_id` (`merchant_id`),
  KEY `idx_customer_leads_store_id` (`store_id`),
  KEY `idx_customer_leads_customer_phone` (`customer_phone`),
  KEY `idx_customer_leads_status` (`status`),
  CONSTRAINT `fk_customer_leads_merchant` FOREIGN KEY (`merchant_id`) REFERENCES `merchants` (`id`),
  CONSTRAINT `fk_customer_leads_store` FOREIGN KEY (`store_id`) REFERENCES `stores` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
DROP TABLE IF EXISTS `email_verification_codes`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `email_verification_codes` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `email` varchar(120) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `scene` varchar(40) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `code` varchar(12) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `user_id` bigint unsigned DEFAULT NULL,
  `expires_at` datetime(3) DEFAULT NULL,
  `consumed_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_email_scene` (`email`,`scene`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
DROP TABLE IF EXISTS `membership_packages`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `membership_packages` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `name` varchar(80) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `code` varchar(40) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `duration_days` bigint DEFAULT NULL,
  `price` bigint DEFAULT NULL,
  `original_price` bigint DEFAULT NULL,
  `quota` bigint DEFAULT NULL,
  `sort` bigint DEFAULT '0',
  `status` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT 'published',
  `is_lifetime` tinyint(1) DEFAULT '0',
  `description` text COLLATE utf8mb4_unicode_ci,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_membership_packages_code` (`code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
DROP TABLE IF EXISTS `merchant_ai_usage_logs`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `merchant_ai_usage_logs` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `merchant_id` bigint unsigned NOT NULL,
  `user_id` bigint unsigned DEFAULT NULL,
  `scenario` varchar(120) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `model` varchar(120) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `provider` varchar(80) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `fallback` tinyint(1) DEFAULT '0',
  `success` tinyint(1) DEFAULT '1',
  `error` varchar(500) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `duration_ms` bigint DEFAULT '0',
  `prompt_tokens` bigint DEFAULT '0',
  `completion_tokens` bigint DEFAULT '0',
  `total_tokens` bigint DEFAULT '0',
  `estimated_cost_cents` bigint DEFAULT '0',
  `used_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_merchant_ai_usage_logs_merchant_id` (`merchant_id`),
  KEY `idx_merchant_ai_usage_logs_user_id` (`user_id`),
  KEY `idx_merchant_ai_usage_logs_used_at` (`used_at`),
  CONSTRAINT `fk_merchant_ai_usage_logs_merchant` FOREIGN KEY (`merchant_id`) REFERENCES `merchants` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
DROP TABLE IF EXISTS `merchant_follow_ups`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `merchant_follow_ups` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `merchant_id` bigint unsigned NOT NULL,
  `type` varchar(30) COLLATE utf8mb4_unicode_ci DEFAULT 'risk',
  `priority` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT 'normal',
  `status` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT 'open',
  `content` varchar(1000) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `source` varchar(40) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `source_id` bigint unsigned DEFAULT NULL,
  `order_id` bigint unsigned DEFAULT NULL,
  `order_no` varchar(60) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `next_follow_at` datetime(3) DEFAULT NULL,
  `operator_id` bigint unsigned DEFAULT NULL,
  `operator_role` varchar(30) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_merchant_follow_ups_order_no` (`order_no`),
  KEY `idx_merchant_follow_ups_operator_id` (`operator_id`),
  KEY `idx_merchant_follow_ups_source` (`source`),
  KEY `idx_merchant_follow_ups_source_id` (`source_id`),
  KEY `idx_merchant_follow_ups_merchant_id` (`merchant_id`),
  KEY `idx_merchant_follow_ups_type` (`type`),
  KEY `idx_merchant_follow_ups_priority` (`priority`),
  KEY `idx_merchant_follow_ups_status` (`status`),
  KEY `idx_merchant_follow_ups_order_id` (`order_id`),
  CONSTRAINT `fk_merchant_follow_ups_merchant` FOREIGN KEY (`merchant_id`) REFERENCES `merchants` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
DROP TABLE IF EXISTS `merchant_group_buy_link_audits`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `merchant_group_buy_link_audits` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `link_id` bigint unsigned NOT NULL,
  `merchant_id` bigint unsigned NOT NULL,
  `store_id` bigint unsigned NOT NULL,
  `channel` varchar(24) COLLATE utf8mb4_unicode_ci NOT NULL,
  `action` varchar(40) COLLATE utf8mb4_unicode_ci NOT NULL,
  `rule_id` varchar(32) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `actor_type` varchar(30) COLLATE utf8mb4_unicode_ci NOT NULL,
  `actor_id` bigint unsigned DEFAULT NULL,
  `actor_name` varchar(120) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `reason` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `before_state` json DEFAULT NULL,
  `after_state` json DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_merchant_group_buy_link_audits_channel` (`channel`),
  KEY `idx_merchant_group_buy_link_audits_action` (`action`),
  KEY `idx_merchant_group_buy_link_audits_rule_id` (`rule_id`),
  KEY `idx_merchant_group_buy_link_audits_actor_type` (`actor_type`),
  KEY `idx_merchant_group_buy_link_audits_actor_id` (`actor_id`),
  KEY `idx_merchant_group_buy_link_audits_link_id` (`link_id`),
  KEY `idx_merchant_group_buy_link_audits_merchant_id` (`merchant_id`),
  KEY `idx_merchant_group_buy_link_audits_store_id` (`store_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
DROP TABLE IF EXISTS `merchant_group_buy_links`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `merchant_group_buy_links` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `merchant_id` bigint unsigned NOT NULL,
  `store_id` bigint unsigned NOT NULL,
  `channel` varchar(24) COLLATE utf8mb4_unicode_ci NOT NULL,
  `destination_url` varchar(1000) COLLATE utf8mb4_unicode_ci NOT NULL,
  `code` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL,
  `enabled` tinyint(1) NOT NULL,
  `click_count` bigint unsigned NOT NULL DEFAULT '0',
  `last_clicked_at` datetime(3) DEFAULT NULL,
  `review_status` varchar(20) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'pending',
  `review_method` varchar(16) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `review_rule_id` varchar(32) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `review_reason` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `reviewed_by` bigint unsigned DEFAULT NULL,
  `reviewed_at` datetime(3) DEFAULT NULL,
  `admin_suspended` tinyint(1) NOT NULL DEFAULT '0',
  `suspension_reason` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `suspended_by` bigint unsigned DEFAULT NULL,
  `suspended_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_merchant_group_buy_links_code` (`code`),
  UNIQUE KEY `idx_merchant_store_channel` (`merchant_id`,`store_id`,`channel`),
  KEY `idx_merchant_group_buy_links_review_method` (`review_method`),
  KEY `idx_merchant_group_buy_links_reviewed_by` (`reviewed_by`),
  KEY `idx_merchant_group_buy_links_suspended_by` (`suspended_by`),
  KEY `idx_merchant_group_buy_links_merchant_id` (`merchant_id`),
  KEY `idx_merchant_group_buy_links_store_id` (`store_id`),
  KEY `idx_merchant_group_buy_links_enabled` (`enabled`),
  KEY `idx_merchant_group_buy_links_review_status` (`review_status`),
  KEY `idx_merchant_group_buy_links_review_rule_id` (`review_rule_id`),
  KEY `idx_merchant_group_buy_links_admin_suspended` (`admin_suspended`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
DROP TABLE IF EXISTS `merchant_payment_configs`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `merchant_payment_configs` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `merchant_id` bigint unsigned DEFAULT NULL,
  `channel` varchar(30) COLLATE utf8mb4_unicode_ci DEFAULT 'alipay',
  `mode` varchar(30) COLLATE utf8mb4_unicode_ci DEFAULT 'direct',
  `account_name` varchar(120) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `account_no` varchar(120) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `alipay_qr_code` varchar(1000) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `wechat_qr_code` varchar(1000) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `app_id` varchar(120) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `status` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT 'disabled',
  `audit_status` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT 'pending',
  `audit_remark` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `contact_phone` varchar(30) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `remark` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_merchant_payment_configs_merchant_id` (`merchant_id`),
  KEY `idx_merchant_payment_configs_status` (`status`),
  KEY `idx_merchant_payment_configs_audit_status` (`audit_status`),
  CONSTRAINT `fk_merchant_payment_configs_merchant` FOREIGN KEY (`merchant_id`) REFERENCES `merchants` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
DROP TABLE IF EXISTS `merchant_plans`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `merchant_plans` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `name` varchar(80) COLLATE utf8mb4_unicode_ci NOT NULL,
  `price_cents` bigint NOT NULL,
  `duration_days` bigint NOT NULL,
  `sort` bigint DEFAULT '0',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
DROP TABLE IF EXISTS `merchant_settlements`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `merchant_settlements` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `merchant_id` bigint unsigned NOT NULL,
  `settlement_period_start` datetime(3) DEFAULT NULL,
  `settlement_period_end` datetime(3) DEFAULT NULL,
  `order_count` bigint DEFAULT '0',
  `total_amount_cents` bigint DEFAULT '0',
  `refund_amount_cents` bigint DEFAULT '0',
  `net_amount_cents` bigint DEFAULT '0',
  `status` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT 'pending',
  `paid_at` datetime(3) DEFAULT NULL,
  `remark` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_merchant_settlements_merchant_id` (`merchant_id`),
  KEY `idx_merchant_settlements_status` (`status`),
  CONSTRAINT `fk_merchant_settlements_merchant` FOREIGN KEY (`merchant_id`) REFERENCES `merchants` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
DROP TABLE IF EXISTS `merchants`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `merchants` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `name` varchar(120) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `contact_phone` varchar(30) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `contact_email` varchar(120) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `status` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT 'pending',
  `subscription_plan` varchar(40) COLLATE utf8mb4_unicode_ci DEFAULT 'none',
  `subscription_status` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT 'inactive',
  `subscription_expired_at` datetime(3) DEFAULT NULL,
  `subscription_note` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `subscription_plan_id` bigint unsigned DEFAULT NULL,
  `subscription_expire_at` datetime(3) DEFAULT NULL,
  `subscription_pause_reason` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `subscription_paused_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_merchants_name` (`name`),
  KEY `idx_merchants_subscription_status` (`subscription_status`),
  KEY `idx_merchants_subscription_plan_id` (`subscription_plan_id`),
  CONSTRAINT `fk_merchants_merchant_plan` FOREIGN KEY (`subscription_plan_id`) REFERENCES `merchant_plans` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
DROP TABLE IF EXISTS `orders`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `orders` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `order_no` varchar(50) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `customer_access_token_hash` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `order_type` varchar(40) COLLATE utf8mb4_unicode_ci DEFAULT 'user_membership',
  `user_id` bigint unsigned DEFAULT NULL,
  `merchant_id` bigint unsigned DEFAULT NULL,
  `settlement_id` bigint unsigned DEFAULT NULL,
  `store_id` bigint unsigned DEFAULT NULL,
  `customer_phone` varchar(30) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `package_id` bigint unsigned DEFAULT NULL,
  `merchant_plan_id` bigint unsigned DEFAULT NULL,
  `merchant_plan_name_snapshot` varchar(80) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `merchant_plan_price_cents_snapshot` bigint DEFAULT NULL,
  `merchant_plan_duration_days_snapshot` bigint DEFAULT NULL,
  `amount` bigint DEFAULT NULL,
  `total_amount` bigint DEFAULT NULL,
  `discount_amount` bigint DEFAULT '0',
  `refunded_amount` bigint DEFAULT '0',
  `refund_status` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT 'none',
  `promotion_id` bigint unsigned DEFAULT NULL,
  `referral_coupon_id` bigint unsigned DEFAULT NULL,
  `coupon_no` varchar(60) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `items` json DEFAULT NULL,
  `customer_note` varchar(500) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `merchant_note` varchar(500) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `customer_rating` bigint DEFAULT '0',
  `customer_review` varchar(1000) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `reviewed_at` datetime(3) DEFAULT NULL,
  `share_code` varchar(40) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `operation_logs` text COLLATE utf8mb4_unicode_ci,
  `status` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT 'pending',
  `payment_channel` varchar(40) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `auto_renew` tinyint(1) DEFAULT '0',
  `paid_at` datetime(3) DEFAULT NULL,
  `transaction_no` varchar(80) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `subscription_end_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_orders_order_no` (`order_no`),
  KEY `idx_orders_order_type` (`order_type`),
  KEY `idx_orders_settlement_id` (`settlement_id`),
  KEY `idx_orders_store_id` (`store_id`),
  KEY `idx_orders_refund_status` (`refund_status`),
  KEY `idx_orders_promotion_id` (`promotion_id`),
  KEY `idx_orders_coupon_id` (`referral_coupon_id`),
  KEY `idx_orders_coupon_no` (`coupon_no`),
  KEY `idx_orders_merchant_id` (`merchant_id`),
  KEY `idx_orders_merchant_plan_id` (`merchant_plan_id`),
  KEY `idx_orders_share_code` (`share_code`),
  KEY `fk_orders_package` (`package_id`),
  KEY `fk_orders_user` (`user_id`),
  CONSTRAINT `fk_orders_merchant` FOREIGN KEY (`merchant_id`) REFERENCES `merchants` (`id`),
  CONSTRAINT `fk_orders_merchant_plan` FOREIGN KEY (`merchant_plan_id`) REFERENCES `merchant_plans` (`id`),
  CONSTRAINT `fk_orders_package` FOREIGN KEY (`package_id`) REFERENCES `membership_packages` (`id`),
  CONSTRAINT `fk_orders_promotion` FOREIGN KEY (`promotion_id`) REFERENCES `promotions` (`id`),
  CONSTRAINT `fk_orders_settlement` FOREIGN KEY (`settlement_id`) REFERENCES `merchant_settlements` (`id`),
  CONSTRAINT `fk_orders_store` FOREIGN KEY (`store_id`) REFERENCES `stores` (`id`),
  CONSTRAINT `fk_orders_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
DROP TABLE IF EXISTS `payment_records`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `payment_records` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `order_id` bigint unsigned DEFAULT NULL,
  `payment_channel` varchar(40) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `transaction_no` varchar(80) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `amount` bigint DEFAULT NULL,
  `status` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT 'success',
  `raw_payload` text COLLATE utf8mb4_unicode_ci,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_payment_records_transaction_no` (`transaction_no`),
  KEY `fk_payment_records_order` (`order_id`),
  CONSTRAINT `fk_payment_records_order` FOREIGN KEY (`order_id`) REFERENCES `orders` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
DROP TABLE IF EXISTS `promotions`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `promotions` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `merchant_id` bigint unsigned NOT NULL,
  `store_id` bigint unsigned DEFAULT NULL,
  `title` varchar(120) COLLATE utf8mb4_unicode_ci NOT NULL,
  `description` varchar(500) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `type` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT 'amount',
  `threshold` bigint DEFAULT '0',
  `discount` bigint DEFAULT '0',
  `discount_rate` bigint DEFAULT '0',
  `status` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT 'draft',
  `valid_from` datetime(3) DEFAULT NULL,
  `valid_to` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_promotions_merchant_id` (`merchant_id`),
  KEY `idx_promotions_store_id` (`store_id`),
  KEY `idx_promotions_status` (`status`),
  CONSTRAINT `fk_promotions_merchant` FOREIGN KEY (`merchant_id`) REFERENCES `merchants` (`id`),
  CONSTRAINT `fk_promotions_store` FOREIGN KEY (`store_id`) REFERENCES `stores` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
DROP TABLE IF EXISTS `referral_coupons`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `referral_coupons` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `merchant_id` bigint unsigned NOT NULL,
  `store_id` bigint unsigned NOT NULL,
  `share_campaign_id` bigint unsigned DEFAULT NULL,
  `order_id` bigint unsigned DEFAULT NULL,
  `coupon_no` varchar(60) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `owner_phone` varchar(30) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `owner_type` varchar(30) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `title` varchar(120) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `amount` bigint NOT NULL DEFAULT '0',
  `threshold` bigint NOT NULL DEFAULT '0',
  `status` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT 'unused',
  `valid_from` datetime(3) DEFAULT NULL,
  `valid_to` datetime(3) DEFAULT NULL,
  `used_at` datetime(3) DEFAULT NULL,
  `remark` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_referral_coupons_coupon_no` (`coupon_no`),
  KEY `idx_referral_coupons_merchant_id` (`merchant_id`),
  KEY `idx_referral_coupons_store_id` (`store_id`),
  KEY `idx_referral_coupons_share_campaign_id` (`share_campaign_id`),
  KEY `idx_referral_coupons_order_id` (`order_id`),
  KEY `idx_referral_coupons_owner_phone` (`owner_phone`),
  KEY `idx_referral_coupons_owner_type` (`owner_type`),
  KEY `idx_referral_coupons_status` (`status`),
  CONSTRAINT `fk_referral_coupons_merchant` FOREIGN KEY (`merchant_id`) REFERENCES `merchants` (`id`),
  CONSTRAINT `fk_referral_coupons_share_campaign` FOREIGN KEY (`share_campaign_id`) REFERENCES `share_campaigns` (`id`),
  CONSTRAINT `fk_referral_coupons_store` FOREIGN KEY (`store_id`) REFERENCES `stores` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
DROP TABLE IF EXISTS `refund_reconciliation_audits`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `refund_reconciliation_audits` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `refund_id` bigint unsigned NOT NULL,
  `refund_no` varchar(60) COLLATE utf8mb4_unicode_ci NOT NULL,
  `order_id` bigint unsigned NOT NULL,
  `actor_id` bigint unsigned NOT NULL,
  `actor_role` varchar(30) COLLATE utf8mb4_unicode_ci NOT NULL,
  `old_status` varchar(20) COLLATE utf8mb4_unicode_ci NOT NULL,
  `new_status` varchar(20) COLLATE utf8mb4_unicode_ci NOT NULL,
  `amount` bigint NOT NULL,
  `provider_reference` varchar(120) COLLATE utf8mb4_unicode_ci NOT NULL,
  `reason` varchar(1000) COLLATE utf8mb4_unicode_ci NOT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_refund_reconciliation_transition` (`refund_id`,`old_status`,`new_status`),
  KEY `idx_refund_reconciliation_audits_refund_id` (`refund_id`),
  KEY `idx_refund_reconciliation_audits_order_id` (`order_id`),
  KEY `idx_refund_reconciliation_audits_actor_id` (`actor_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
/*!50003 SET @saved_cs_client      = @@character_set_client */ ;
/*!50003 SET @saved_cs_results     = @@character_set_results */ ;
/*!50003 SET @saved_col_connection = @@collation_connection */ ;
/*!50003 SET character_set_client  = utf8mb4 */ ;
/*!50003 SET character_set_results = utf8mb4 */ ;
/*!50003 SET collation_connection  = utf8mb4_0900_ai_ci */ ;
/*!50003 SET @saved_sql_mode       = @@sql_mode */ ;
/*!50003 SET sql_mode              = 'ONLY_FULL_GROUP_BY,STRICT_TRANS_TABLES,NO_ZERO_IN_DATE,NO_ZERO_DATE,ERROR_FOR_DIVISION_BY_ZERO,NO_ENGINE_SUBSTITUTION' */ ;
DELIMITER ;;
/*!50003 CREATE*/ /*!50017 DEFINER=`root`@`localhost`*/ /*!50003 TRIGGER `trg_refund_reconciliation_audits_no_update` BEFORE UPDATE ON `refund_reconciliation_audits` FOR EACH ROW BEGIN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'refund reconciliation audit is append-only';
END */;;
DELIMITER ;
/*!50003 SET sql_mode              = @saved_sql_mode */ ;
/*!50003 SET character_set_client  = @saved_cs_client */ ;
/*!50003 SET character_set_results = @saved_cs_results */ ;
/*!50003 SET collation_connection  = @saved_col_connection */ ;
/*!50003 SET @saved_cs_client      = @@character_set_client */ ;
/*!50003 SET @saved_cs_results     = @@character_set_results */ ;
/*!50003 SET @saved_col_connection = @@collation_connection */ ;
/*!50003 SET character_set_client  = utf8mb4 */ ;
/*!50003 SET character_set_results = utf8mb4 */ ;
/*!50003 SET collation_connection  = utf8mb4_0900_ai_ci */ ;
/*!50003 SET @saved_sql_mode       = @@sql_mode */ ;
/*!50003 SET sql_mode              = 'ONLY_FULL_GROUP_BY,STRICT_TRANS_TABLES,NO_ZERO_IN_DATE,NO_ZERO_DATE,ERROR_FOR_DIVISION_BY_ZERO,NO_ENGINE_SUBSTITUTION' */ ;
DELIMITER ;;
/*!50003 CREATE*/ /*!50017 DEFINER=`root`@`localhost`*/ /*!50003 TRIGGER `trg_refund_reconciliation_audits_no_delete` BEFORE DELETE ON `refund_reconciliation_audits` FOR EACH ROW BEGIN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'refund reconciliation audit is append-only';
END */;;
DELIMITER ;
/*!50003 SET sql_mode              = @saved_sql_mode */ ;
/*!50003 SET character_set_client  = @saved_cs_client */ ;
/*!50003 SET character_set_results = @saved_cs_results */ ;
/*!50003 SET collation_connection  = @saved_col_connection */ ;
DROP TABLE IF EXISTS `refund_records`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `refund_records` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `order_id` bigint unsigned DEFAULT NULL,
  `merchant_id` bigint unsigned DEFAULT NULL,
  `store_id` bigint unsigned DEFAULT NULL,
  `refund_no` varchar(60) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `idempotency_key` varchar(120) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `request_fingerprint` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `transaction_no` varchar(80) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `amount` bigint NOT NULL,
  `reason` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `operator_role` varchar(30) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `operator_id` bigint unsigned DEFAULT NULL,
  `status` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT 'success',
  `raw_payload` text COLLATE utf8mb4_unicode_ci,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_refund_order_idempotency` (`order_id`,`idempotency_key`),
  UNIQUE KEY `idx_refund_records_refund_no` (`refund_no`),
  KEY `idx_refund_records_merchant_id` (`merchant_id`),
  KEY `idx_refund_records_store_id` (`store_id`),
  KEY `idx_refund_records_transaction_no` (`transaction_no`),
  KEY `idx_refund_records_status` (`status`),
  KEY `idx_refund_records_order_id` (`order_id`),
  CONSTRAINT `fk_refund_records_order` FOREIGN KEY (`order_id`) REFERENCES `orders` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
DROP TABLE IF EXISTS `share_activity_configs`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `share_activity_configs` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `merchant_id` bigint unsigned NOT NULL,
  `enabled` tinyint(1) DEFAULT '0',
  `poster_title` varchar(120) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `poster_copy` varchar(500) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `friend_coupon_amount` bigint NOT NULL DEFAULT '500',
  `friend_coupon_threshold` bigint NOT NULL DEFAULT '3000',
  `referrer_coupon_amount` bigint NOT NULL DEFAULT '500',
  `referrer_coupon_threshold` bigint NOT NULL DEFAULT '3000',
  `valid_days` bigint NOT NULL DEFAULT '30',
  `status` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT 'active',
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_share_activity_configs_merchant_id` (`merchant_id`),
  KEY `idx_share_activity_configs_enabled` (`enabled`),
  KEY `idx_share_activity_configs_status` (`status`),
  CONSTRAINT `fk_share_activity_configs_merchant` FOREIGN KEY (`merchant_id`) REFERENCES `merchants` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
DROP TABLE IF EXISTS `share_campaigns`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `share_campaigns` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `merchant_id` bigint unsigned NOT NULL,
  `store_id` bigint unsigned NOT NULL,
  `order_id` bigint unsigned DEFAULT NULL,
  `share_code` varchar(40) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `customer_phone` varchar(30) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `poster_title` varchar(120) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `poster_copy` varchar(500) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `scan_count` bigint DEFAULT '0',
  `lead_count` bigint DEFAULT '0',
  `conversion_count` bigint DEFAULT '0',
  `conversion_amount` bigint DEFAULT '0',
  `status` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT 'active',
  `last_scanned_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_share_campaigns_share_code` (`share_code`),
  KEY `idx_share_campaigns_merchant_id` (`merchant_id`),
  KEY `idx_share_campaigns_store_id` (`store_id`),
  KEY `idx_share_campaigns_order_id` (`order_id`),
  KEY `idx_share_campaigns_customer_phone` (`customer_phone`),
  KEY `idx_share_campaigns_status` (`status`),
  CONSTRAINT `fk_share_campaigns_merchant` FOREIGN KEY (`merchant_id`) REFERENCES `merchants` (`id`),
  CONSTRAINT `fk_share_campaigns_order` FOREIGN KEY (`order_id`) REFERENCES `orders` (`id`),
  CONSTRAINT `fk_share_campaigns_store` FOREIGN KEY (`store_id`) REFERENCES `stores` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
DROP TABLE IF EXISTS `store_products`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `store_products` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `store_id` bigint unsigned NOT NULL,
  `name` varchar(120) COLLATE utf8mb4_unicode_ci NOT NULL,
  `price` bigint NOT NULL DEFAULT '0',
  `description` varchar(500) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `image_url` varchar(500) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `category` varchar(80) COLLATE utf8mb4_unicode_ci DEFAULT '默认分类',
  `status` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT 'active',
  `stock` bigint DEFAULT NULL,
  `sort` bigint DEFAULT '100',
  PRIMARY KEY (`id`),
  KEY `idx_store_products_category` (`category`),
  KEY `idx_store_products_status` (`status`),
  KEY `idx_store_products_sort` (`sort`),
  KEY `idx_store_products_store_id` (`store_id`),
  CONSTRAINT `fk_store_products_store` FOREIGN KEY (`store_id`) REFERENCES `stores` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
DROP TABLE IF EXISTS `stores`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `stores` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `merchant_id` bigint unsigned NOT NULL,
  `name` varchar(120) COLLATE utf8mb4_unicode_ci NOT NULL,
  `address` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `contact_phone` varchar(30) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `status` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT 'active',
  `is_open` tinyint(1) DEFAULT '1',
  `business_hours` varchar(120) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `pause_reason` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `order_mode` varchar(30) COLLATE utf8mb4_unicode_ci DEFAULT 'pay_first',
  `auto_accept` tinyint(1) DEFAULT '0',
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_merchant_store_name` (`merchant_id`,`name`),
  KEY `idx_stores_merchant_id` (`merchant_id`),
  KEY `idx_stores_status` (`status`),
  KEY `idx_stores_is_open` (`is_open`),
  KEY `idx_stores_order_mode` (`order_mode`),
  KEY `idx_stores_auto_accept` (`auto_accept`),
  CONSTRAINT `fk_stores_merchant` FOREIGN KEY (`merchant_id`) REFERENCES `merchants` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
DROP TABLE IF EXISTS `system_configs`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `system_configs` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `config_key` varchar(80) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `config_value` text COLLATE utf8mb4_unicode_ci,
  `is_encrypted` tinyint(1) DEFAULT '0',
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_system_configs_config_key` (`config_key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
DROP TABLE IF EXISTS `usage_records`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `usage_records` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `user_id` bigint unsigned DEFAULT NULL,
  `scene` varchar(80) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `description` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `quota_used` bigint DEFAULT '0',
  `device_id` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
DROP TABLE IF EXISTS `users`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `users` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `uuid` char(36) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `email` varchar(120) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `phone` varchar(30) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `merchant_id` bigint unsigned DEFAULT NULL,
  `role` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT 'customer',
  `password_hash` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `display_name` varchar(80) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `status` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT 'active',
  `member_level` varchar(30) COLLATE utf8mb4_unicode_ci DEFAULT 'free',
  `current_package_id` bigint unsigned DEFAULT NULL,
  `expired_at` datetime(3) DEFAULT NULL,
  `remaining_quota` bigint DEFAULT '0',
  `bound_devices` text COLLATE utf8mb4_unicode_ci,
  `auto_renew` tinyint(1) DEFAULT '0',
  `reset_token` varchar(120) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_users_phone` (`phone`),
  UNIQUE KEY `idx_users_uuid` (`uuid`),
  UNIQUE KEY `idx_users_email` (`email`),
  KEY `idx_users_merchant_id` (`merchant_id`),
  KEY `idx_users_role` (`role`),
  CONSTRAINT `fk_users_merchant` FOREIGN KEY (`merchant_id`) REFERENCES `merchants` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
DROP TABLE IF EXISTS `verification_codes`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `verification_codes` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `phone` varchar(30) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `scene` varchar(40) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `code` varchar(12) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `user_id` bigint unsigned DEFAULT NULL,
  `expires_at` datetime(3) DEFAULT NULL,
  `consumed_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_phone_scene` (`phone`,`scene`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
/*!40103 SET TIME_ZONE=@OLD_TIME_ZONE */;

/*!40101 SET SQL_MODE=@OLD_SQL_MODE */;
/*!40014 SET FOREIGN_KEY_CHECKS=@OLD_FOREIGN_KEY_CHECKS */;
/*!40014 SET UNIQUE_CHECKS=@OLD_UNIQUE_CHECKS */;
/*!40101 SET CHARACTER_SET_CLIENT=@OLD_CHARACTER_SET_CLIENT */;
/*!40101 SET CHARACTER_SET_RESULTS=@OLD_CHARACTER_SET_RESULTS */;
/*!40101 SET COLLATION_CONNECTION=@OLD_COLLATION_CONNECTION */;
/*!40111 SET SQL_NOTES=@OLD_SQL_NOTES */;
