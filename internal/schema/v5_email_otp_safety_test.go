package schema

import (
	"strings"
	"testing"
)

func TestLoadEmailOTPSafetyV5IsChecksumPinned(t *testing.T) {
	definition, err := loadEmailOTPSafetyV5()
	if err != nil {
		t.Fatalf("load email OTP safety v5 definition: %v", err)
	}
	if definition.version != EmailOTPSafetyV5Version || definition.checksum != EmailOTPSafetyV5Checksum {
		t.Fatalf("loaded v5 identity=%q/%q, want %q/%q", definition.version, definition.checksum, EmailOTPSafetyV5Version, EmailOTPSafetyV5Checksum)
	}
	if !strings.Contains(definition.ddl, "`email_verification_codes`") || !strings.Contains(definition.ddl, "`code_hash` char(64)") || !strings.Contains(definition.ddl, "`failed_attempts` int unsigned") {
		t.Fatalf("unexpected email OTP safety v5 DDL: %s", definition.ddl)
	}
	if !strings.Contains(definition.scrub, "REPEAT('0',64)") || !strings.Contains(definition.scrub, "`code`='REDACTED'") || !strings.Contains(definition.scrub, "`consumed_at`") {
		t.Fatalf("v5 must overwrite legacy plaintext email OTPs and invalidate them: %s", definition.scrub)
	}
}

func TestEmailOTPSafetyV5PreservesExistingMigrationIdentities(t *testing.T) {
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
	v5, err := loadEmailOTPSafetyV5()
	if err != nil {
		t.Fatal(err)
	}
	if base.checksum != BaselineChecksum || v2.checksum != ManualReviewV2Checksum || v3.checksum != AdminAuthV3Checksum || v4.checksum != OTPSafetyV4Checksum || v5.checksum != EmailOTPSafetyV5Checksum {
		t.Fatalf("migration checksums changed unexpectedly: v1=%s v2=%s v3=%s v4=%s v5=%s", base.checksum, v2.checksum, v3.checksum, v4.checksum, v5.checksum)
	}
}
