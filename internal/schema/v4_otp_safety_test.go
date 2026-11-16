package schema

import (
	"strings"
	"testing"
)

func TestLoadOTPSafetyV4IsChecksumPinned(t *testing.T) {
	definition, err := loadOTPSafetyV4()
	if err != nil {
		t.Fatalf("load OTP safety v4 definition: %v", err)
	}
	if definition.version != OTPSafetyV4Version || definition.checksum != OTPSafetyV4Checksum {
		t.Fatalf("loaded v4 identity=%q/%q, want %q/%q", definition.version, definition.checksum, OTPSafetyV4Version, OTPSafetyV4Checksum)
	}
	if !strings.Contains(definition.ddl, "`verification_codes`") || !strings.Contains(definition.ddl, "`code_hash` char(64)") || !strings.Contains(definition.ddl, "`failed_attempts` int unsigned") {
		t.Fatalf("unexpected OTP safety v4 DDL: %s", definition.ddl)
	}
	if !strings.Contains(definition.scrub, "REPEAT('0',64)") || !strings.Contains(definition.scrub, "`code`='REDACTED'") || !strings.Contains(definition.scrub, "`consumed_at`") {
		t.Fatalf("v4 must overwrite legacy plaintext OTPs and invalidate them: %s", definition.scrub)
	}
}

func TestOTPSafetyV4PreservesExistingMigrationIdentities(t *testing.T) {
	base, err := loadBaseline()
	if err != nil {
		t.Fatal(err)
	}
	v2, err := loadManualReviewV2()
	if err != nil {
		t.Fatal(err)
	}
	v3, err := loadAdminAuthV3()
	if err != nil {
		t.Fatal(err)
	}
	v4, err := loadOTPSafetyV4()
	if err != nil {
		t.Fatal(err)
	}
	if base.checksum != BaselineChecksum || v2.checksum != ManualReviewV2Checksum || v3.checksum != AdminAuthV3Checksum || v4.checksum != OTPSafetyV4Checksum {
		t.Fatalf("migration checksums changed unexpectedly: v1=%s v2=%s v3=%s v4=%s", base.checksum, v2.checksum, v3.checksum, v4.checksum)
	}
}
