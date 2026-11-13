package schema

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
)

const (
	ManualReviewV2Version  = "schema-v2-subscription-manual-review-20261113"
	ManualReviewV2Checksum = "48ef3ec703a314a213107af20ad415e80dc4aa27cdccaf3349cdc9ab53ea5e26"
)

type incrementalMigrationDefinition struct {
	version  string
	checksum string
	tables   map[string]string
	triggers map[string]triggerDefinition
}

func loadManualReviewV2() (incrementalMigrationDefinition, error) {
	definition := incrementalMigrationDefinition{
		version: ManualReviewV2Version,
		tables: map[string]string{
			"merchant_subscription_manual_reviews": `CREATE TABLE ` + "`merchant_subscription_manual_reviews`" + ` (
				` + "`id`" + ` bigint unsigned NOT NULL AUTO_INCREMENT,
				` + "`review_key`" + ` varchar(100) COLLATE utf8mb4_unicode_ci NOT NULL,
				` + "`order_id`" + ` bigint unsigned NOT NULL,
				` + "`order_no`" + ` varchar(50) COLLATE utf8mb4_unicode_ci NOT NULL,
				` + "`merchant_id`" + ` bigint unsigned NOT NULL,
				` + "`actor_id`" + ` bigint unsigned NOT NULL,
				` + "`actor_role`" + ` varchar(30) COLLATE utf8mb4_unicode_ci NOT NULL,
				` + "`plan_name`" + ` varchar(80) COLLATE utf8mb4_unicode_ci NOT NULL,
				` + "`amount_cents`" + ` bigint NOT NULL,
				` + "`duration_days`" + ` bigint NOT NULL,
				` + "`payment_occurred_attested`" + ` tinyint(1) NOT NULL,
				` + "`evidence_reference`" + ` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
				` + "`reason`" + ` varchar(1000) COLLATE utf8mb4_unicode_ci NOT NULL,
				` + "`request_fingerprint`" + ` char(64) COLLATE utf8mb4_unicode_ci NOT NULL,
				` + "`review_status`" + ` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL,
				` + "`created_at`" + ` datetime(3) NOT NULL,
				` + "`updated_at`" + ` datetime(3) NOT NULL,
				PRIMARY KEY (` + "`id`" + `),
				UNIQUE KEY ` + "`uk_subscription_manual_reviews_review_key`" + ` (` + "`review_key`" + `),
				UNIQUE KEY ` + "`uk_subscription_manual_reviews_order_id`" + ` (` + "`order_id`" + `),
				KEY ` + "`idx_subscription_manual_reviews_merchant_id`" + ` (` + "`merchant_id`" + `),
				KEY ` + "`idx_subscription_manual_reviews_actor_id`" + ` (` + "`actor_id`" + `),
				CONSTRAINT ` + "`fk_subscription_manual_reviews_order`" + ` FOREIGN KEY (` + "`order_id`" + `) REFERENCES ` + "`orders`" + ` (` + "`id`" + `) ON UPDATE RESTRICT ON DELETE RESTRICT,
				CONSTRAINT ` + "`fk_subscription_manual_reviews_merchant`" + ` FOREIGN KEY (` + "`merchant_id`" + `) REFERENCES ` + "`merchants`" + ` (` + "`id`" + `) ON UPDATE RESTRICT ON DELETE RESTRICT
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
			"merchant_subscription_manual_review_audits": `CREATE TABLE ` + "`merchant_subscription_manual_review_audits`" + ` (
				` + "`id`" + ` bigint unsigned NOT NULL AUTO_INCREMENT,
				` + "`review_id`" + ` bigint unsigned NOT NULL,
				` + "`order_id`" + ` bigint unsigned NOT NULL,
				` + "`merchant_id`" + ` bigint unsigned NOT NULL,
				` + "`actor_id`" + ` bigint unsigned NOT NULL,
				` + "`actor_role`" + ` varchar(30) COLLATE utf8mb4_unicode_ci NOT NULL,
				` + "`evidence_reference`" + ` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
				` + "`reason`" + ` varchar(1000) COLLATE utf8mb4_unicode_ci NOT NULL,
				` + "`plan_name`" + ` varchar(80) COLLATE utf8mb4_unicode_ci NOT NULL,
				` + "`amount_cents`" + ` bigint NOT NULL,
				` + "`duration_days`" + ` bigint NOT NULL,
				` + "`payment_occurred_attested`" + ` tinyint(1) NOT NULL,
				` + "`order_status_before`" + ` varchar(20) COLLATE utf8mb4_unicode_ci NOT NULL,
				` + "`order_status_after`" + ` varchar(20) COLLATE utf8mb4_unicode_ci NOT NULL,
				` + "`order_paid_at_before`" + ` datetime(3) DEFAULT NULL,
				` + "`order_paid_at_after`" + ` datetime(3) DEFAULT NULL,
				` + "`order_updated_at_before`" + ` datetime(3) NOT NULL,
				` + "`order_updated_at_after`" + ` datetime(3) NOT NULL,
				` + "`subscription_status_before`" + ` varchar(20) COLLATE utf8mb4_unicode_ci NOT NULL,
				` + "`subscription_status_after`" + ` varchar(20) COLLATE utf8mb4_unicode_ci NOT NULL,
				` + "`subscription_plan_before`" + ` varchar(40) COLLATE utf8mb4_unicode_ci NOT NULL,
				` + "`subscription_plan_after`" + ` varchar(40) COLLATE utf8mb4_unicode_ci NOT NULL,
				` + "`subscription_expire_at_before`" + ` datetime(3) DEFAULT NULL,
				` + "`subscription_expire_at_after`" + ` datetime(3) NOT NULL,
				` + "`reviewed_at`" + ` datetime(3) NOT NULL,
				` + "`created_at`" + ` datetime(3) NOT NULL,
				PRIMARY KEY (` + "`id`" + `),
				UNIQUE KEY ` + "`uk_subscription_manual_review_audits_review_id`" + ` (` + "`review_id`" + `),
				KEY ` + "`idx_subscription_manual_review_audits_order_id`" + ` (` + "`order_id`" + `),
				KEY ` + "`idx_subscription_manual_review_audits_merchant_id`" + ` (` + "`merchant_id`" + `),
				KEY ` + "`idx_subscription_manual_review_audits_actor_id`" + ` (` + "`actor_id`" + `),
				CONSTRAINT ` + "`fk_subscription_manual_review_audits_review`" + ` FOREIGN KEY (` + "`review_id`" + `) REFERENCES ` + "`merchant_subscription_manual_reviews`" + ` (` + "`id`" + `) ON UPDATE RESTRICT ON DELETE RESTRICT,
				CONSTRAINT ` + "`fk_subscription_manual_review_audits_order`" + ` FOREIGN KEY (` + "`order_id`" + `) REFERENCES ` + "`orders`" + ` (` + "`id`" + `) ON UPDATE RESTRICT ON DELETE RESTRICT,
				CONSTRAINT ` + "`fk_subscription_manual_review_audits_merchant`" + ` FOREIGN KEY (` + "`merchant_id`" + `) REFERENCES ` + "`merchants`" + ` (` + "`id`" + `) ON UPDATE RESTRICT ON DELETE RESTRICT
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		},
		triggers: make(map[string]triggerDefinition),
	}
	for name, trigger := range map[string]triggerDefinition{
		"trg_subscription_manual_reviews_no_update": {
			name: "trg_subscription_manual_reviews_no_update", table: "merchant_subscription_manual_reviews", timing: "BEFORE", event: "UPDATE",
			body: "SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'merchant subscription manual review is append-only';",
		},
		"trg_subscription_manual_reviews_no_delete": {
			name: "trg_subscription_manual_reviews_no_delete", table: "merchant_subscription_manual_reviews", timing: "BEFORE", event: "DELETE",
			body: "SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'merchant subscription manual review is append-only';",
		},
		"trg_subscription_manual_review_audits_no_update": {
			name: "trg_subscription_manual_review_audits_no_update", table: "merchant_subscription_manual_review_audits", timing: "BEFORE", event: "UPDATE",
			body: "SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'merchant subscription manual review audit is append-only';",
		},
		"trg_subscription_manual_review_audits_no_delete": {
			name: "trg_subscription_manual_review_audits_no_delete", table: "merchant_subscription_manual_review_audits", timing: "BEFORE", event: "DELETE",
			body: "SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'merchant subscription manual review audit is append-only';",
		},
	} {
		trigger.stmt = fmt.Sprintf("CREATE TRIGGER `%s` %s %s ON `%s` FOR EACH ROW BEGIN\n%s\nEND", trigger.name, trigger.timing, trigger.event, trigger.table, trigger.body)
		definition.triggers[name] = trigger
	}

	if err := validateManualReviewV2Definition(&definition); err != nil {
		return incrementalMigrationDefinition{}, err
	}
	return definition, nil
}

func validateManualReviewV2Definition(definition *incrementalMigrationDefinition) error {
	if definition.version != ManualReviewV2Version || len(definition.tables) != 2 || len(definition.triggers) != 4 {
		return fmt.Errorf("subscription manual review v2 definition inventory mismatch")
	}
	for table := range definition.tables {
		if table != "merchant_subscription_manual_reviews" && table != "merchant_subscription_manual_review_audits" {
			return fmt.Errorf("unexpected subscription manual review v2 table %q", table)
		}
	}
	for _, trigger := range definition.triggers {
		if trigger.timing != "BEFORE" || (trigger.event != "UPDATE" && trigger.event != "DELETE") || !strings.Contains(strings.ToUpper(trigger.body), "SIGNAL SQLSTATE '45000'") || !strings.Contains(trigger.body, "append-only") {
			return fmt.Errorf("subscription manual review v2 trigger %q is not append-only", trigger.name)
		}
	}
	material := []string{definition.version}
	tableNames := make([]string, 0, len(definition.tables))
	for name := range definition.tables {
		tableNames = append(tableNames, name)
	}
	sort.Strings(tableNames)
	for _, name := range tableNames {
		material = append(material, definition.tables[name])
	}
	triggerNames := make([]string, 0, len(definition.triggers))
	for name := range definition.triggers {
		triggerNames = append(triggerNames, name)
	}
	sort.Strings(triggerNames)
	for _, name := range triggerNames {
		material = append(material, definition.triggers[name].stmt)
	}
	sum := sha256.Sum256([]byte(strings.Join(material, "\n")))
	checksum := fmt.Sprintf("%x", sum)
	if checksum != ManualReviewV2Checksum {
		return fmt.Errorf("subscription manual review v2 checksum mismatch: definition %s, expected %s", checksum, ManualReviewV2Checksum)
	}
	definition.checksum = checksum
	return nil
}
