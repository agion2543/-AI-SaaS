package service

import (
	"testing"

	"go-web-gin-health/internal/config"
	"go-web-gin-health/internal/model"
	"go-web-gin-health/internal/utils"
)

func TestListSystemConfigsDecryptsLegacyAndVersionedCiphertext(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	if err := db.AutoMigrate(&model.SystemConfig{}); err != nil {
		t.Fatalf("migrate synthetic system-config schema: %v", err)
	}

	// This exact 32-byte key matches the historical fixed-nonce fixture
	// exercised independently in utils tests.
	secret := "0123456789abcdef0123456789abcdef"
	const legacyPlaintext = "legacy payment gateway fixture"
	const legacyCiphertext = "9cyAWyCzuGuUfE7Y3XBcQCPysbh2sUoNTfe7bLudg2UEJeDlzLNgS+lWxtqIXA=="
	const versionedPlaintext = "synthetic versioned gateway secret"
	versionedCiphertext, err := utils.EncryptString(secret, versionedPlaintext)
	if err != nil {
		t.Fatalf("encrypt synthetic versioned config: %v", err)
	}

	seed := []model.SystemConfig{
		{ConfigKey: "synthetic.legacy_secret", ConfigValue: legacyCiphertext, IsEncrypted: true},
		{ConfigKey: "synthetic.versioned_secret", ConfigValue: versionedCiphertext, IsEncrypted: true},
		{ConfigKey: "synthetic.public_value", ConfigValue: "synthetic public config", IsEncrypted: false},
	}
	if err := db.Create(&seed).Error; err != nil {
		t.Fatalf("seed synthetic system configs: %v", err)
	}

	service := NewAdminService(&config.Config{AESSecret: secret}, db)
	configs, err := service.ListSystemConfigs()
	if err != nil {
		t.Fatalf("list synthetic system configs: %v", err)
	}
	values := make(map[string]model.SystemConfig, len(configs))
	for _, item := range configs {
		values[item.ConfigKey] = item
	}
	for key, want := range map[string]string{
		"synthetic.legacy_secret":    legacyPlaintext,
		"synthetic.versioned_secret": versionedPlaintext,
		"synthetic.public_value":     "synthetic public config",
	} {
		item, ok := values[key]
		if !ok {
			t.Fatalf("listed config %q was missing", key)
		}
		if item.ConfigValue != want {
			t.Errorf("listed config %q = %q, want %q", key, item.ConfigValue, want)
		}
	}
}

func TestListSystemConfigsFailsClosedOnWrongAESSecret(t *testing.T) {
	db := newOrderBoundaryTestDB(t)
	if err := db.AutoMigrate(&model.SystemConfig{}); err != nil {
		t.Fatalf("migrate synthetic system-config schema: %v", err)
	}
	storedWithKey := "0123456789abcdef0123456789abcdef"
	ciphertext, err := utils.EncryptString(storedWithKey, "synthetic private config value")
	if err != nil {
		t.Fatalf("encrypt synthetic config: %v", err)
	}
	if err := db.Create(&model.SystemConfig{
		ConfigKey:   "synthetic.private_config",
		ConfigValue: ciphertext,
		IsEncrypted: true,
	}).Error; err != nil {
		t.Fatalf("seed synthetic encrypted config: %v", err)
	}

	service := NewAdminService(&config.Config{AESSecret: "fedcba0987654321fedcba0987654321"}, db)
	configs, err := service.ListSystemConfigs()
	if err == nil {
		t.Fatal("listing encrypted config with the wrong AES_SECRET must fail")
	}
	if configs != nil {
		t.Fatal("failed encrypted-config listing must not return ciphertext or partial records")
	}
}
