package schema

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"

	"go-web-gin-health/sql/baselines"
)

const (
	BaselineVersion  = "schema-v1-candidate-20261113"
	BaselineChecksum = "d4277d6999a8c5fade3edda12e6d7d9bfbf7874c34a0b0406d03573b68609006"
	ledgerTable      = "app_schema_migrations"
)

type triggerDefinition struct {
	name   string
	table  string
	timing string
	event  string
	body   string
	stmt   string
}

type baselineDefinition struct {
	checksum string
	tables   map[string]string
	triggers map[string]triggerDefinition
}

var (
	createTablePattern = regexp.MustCompile(`(?is)CREATE TABLE \x60([^\x60]+)\x60 \((.*?)\) ENGINE=[^;]+;`)
	triggerPattern     = regexp.MustCompile("(?is)/\\*!50003 CREATE\\*/\\s*/\\*!50017 DEFINER=.*?\\*/\\s*/\\*!50003 TRIGGER `([^`]+)` (BEFORE|AFTER) (INSERT|UPDATE|DELETE) ON `([^`]+)` FOR EACH ROW BEGIN\\s*(.*?)\\s*END \\*/;;")
)

func loadBaseline() (baselineDefinition, error) {
	sum := sha256.Sum256(baselines.CandidateSchemaV1)
	checksum := fmt.Sprintf("%x", sum)
	if checksum != BaselineChecksum {
		return baselineDefinition{}, fmt.Errorf("candidate baseline checksum mismatch: embedded %s, expected %s", checksum, BaselineChecksum)
	}

	definition := baselineDefinition{
		checksum: checksum,
		tables:   make(map[string]string),
		triggers: make(map[string]triggerDefinition),
	}
	for _, match := range createTablePattern.FindAllSubmatch(baselines.CandidateSchemaV1, -1) {
		name := string(match[1])
		stmt := strings.TrimSpace(string(match[0]))
		if name == "" || stmt == "" {
			return baselineDefinition{}, fmt.Errorf("candidate baseline contains an invalid CREATE TABLE")
		}
		if _, exists := definition.tables[name]; exists {
			return baselineDefinition{}, fmt.Errorf("candidate baseline repeats table %q", name)
		}
		definition.tables[name] = stmt
	}
	for _, match := range triggerPattern.FindAllSubmatch(baselines.CandidateSchemaV1, -1) {
		trigger := triggerDefinition{
			name:   string(match[1]),
			timing: strings.ToUpper(string(match[2])),
			event:  strings.ToUpper(string(match[3])),
			table:  string(match[4]),
			body:   strings.TrimSpace(string(match[5])),
		}
		if trigger.name == "" || trigger.table == "" || trigger.body == "" {
			return baselineDefinition{}, fmt.Errorf("candidate baseline contains an invalid trigger")
		}
		if _, exists := definition.triggers[trigger.name]; exists {
			return baselineDefinition{}, fmt.Errorf("candidate baseline repeats trigger %q", trigger.name)
		}
		trigger.stmt = fmt.Sprintf("CREATE TRIGGER `%s` %s %s ON `%s` FOR EACH ROW BEGIN\n%s\nEND", trigger.name, trigger.timing, trigger.event, trigger.table, trigger.body)
		definition.triggers[trigger.name] = trigger
	}

	if len(definition.tables) != 29 || len(definition.triggers) != 2 {
		return baselineDefinition{}, fmt.Errorf("candidate baseline object count mismatch: got %d tables and %d triggers, want 29 and 2", len(definition.tables), len(definition.triggers))
	}
	if _, ok := definition.tables["refund_reconciliation_audits"]; !ok {
		return baselineDefinition{}, fmt.Errorf("candidate baseline is missing refund_reconciliation_audits")
	}
	for name, wantEvent := range map[string]string{
		"trg_refund_reconciliation_audits_no_update": "UPDATE",
		"trg_refund_reconciliation_audits_no_delete": "DELETE",
	} {
		trigger, ok := definition.triggers[name]
		if !ok || trigger.table != "refund_reconciliation_audits" || trigger.timing != "BEFORE" || trigger.event != wantEvent || !strings.Contains(strings.ToUpper(trigger.body), "SIGNAL SQLSTATE '45000'") || !strings.Contains(trigger.body, "refund reconciliation audit is append-only") {
			return baselineDefinition{}, fmt.Errorf("candidate baseline is missing or has altered append-only trigger %q", name)
		}
	}
	return definition, nil
}
