package service

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	mysqlDriver "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"go-web-gin-health/internal/model"
)

const mysqlOTPTestDSNEnv = "MYSQL_OTP_IT_DSN"

func TestMySQLOTPConcurrentIssueAndConsume(t *testing.T) {
	db := newMySQLOTPIntegrationDB(t)

	for _, test := range []struct {
		name  string
		send  func() error
		count func() (int64, error)
	}{
		{
			name: "first SMS send",
			send: func() error {
				return issueSMSVerificationCode(db, true, testOTPSecret, "13800138991", "register", nil)
			},
			count: func() (int64, error) {
				var count int64
				err := db.Model(&model.VerificationCode{}).Where("phone = ?", "13800138991").Count(&count).Error
				return count, err
			},
		},
		{
			name: "first email send",
			send: func() error {
				return issueEmailVerificationCode(db, true, testOTPSecret, "otp-first-send@example.test", "bind_email", nil)
			},
			count: func() (int64, error) {
				var count int64
				err := db.Model(&model.EmailVerificationCode{}).Where("email = ?", "otp-first-send@example.test").Count(&count).Error
				return count, err
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			const workers = 16
			results := concurrentOTPCalls(workers, test.send)
			successes := countSuccessfulOTPCalls(results)
			if successes == 0 {
				t.Fatalf("all concurrent first sends failed; errors: %v", results)
			}
			count, err := test.count()
			if err != nil {
				t.Fatal(err)
			}
			if count != 1 {
				t.Fatalf("concurrent first sends created %d OTP rows; want exactly one", count)
			}
		})
	}

	t.Run("SMS consumption is single use", func(t *testing.T) {
		phone := "13800138992"
		digest, err := hashSMSCode(testOTPSecret, phone, "login", "246810")
		if err != nil {
			t.Fatal(err)
		}
		if err := db.Create(&model.VerificationCode{
			Phone: phone, Scene: "login", Code: legacyOTPRedacted, CodeHash: digest,
			ExpiresAt: time.Now().Add(otpLifetime),
		}).Error; err != nil {
			t.Fatal(err)
		}
		results := concurrentOTPCalls(16, func() error {
			return consumeSMSVerificationCode(db, testOTPSecret, phone, "login", "246810", nil)
		})
		assertOnlyOneOTPConsumerWins(t, results, errSMSInvalidCode)
	})

	t.Run("email OTP consumption is single use", func(t *testing.T) {
		email := "otp-consume@example.test"
		digest, err := hashEmailCode(testOTPSecret, email, "bind_email", "135790")
		if err != nil {
			t.Fatal(err)
		}
		if err := db.Create(&model.EmailVerificationCode{
			Email: email, Scene: "bind_email", Code: legacyOTPRedacted, CodeHash: digest,
			ExpiresAt: time.Now().Add(otpLifetime),
		}).Error; err != nil {
			t.Fatal(err)
		}
		results := concurrentOTPCalls(16, func() error {
			return consumeEmailVerificationCode(db, testOTPSecret, email, "bind_email", "135790", nil)
		})
		assertOnlyOneOTPConsumerWins(t, results, errEmailInvalidCode)
	})
}

func concurrentOTPCalls(workers int, call func() error) []error {
	start := make(chan struct{})
	results := make(chan error, workers)
	var group sync.WaitGroup
	group.Add(workers)
	for index := 0; index < workers; index++ {
		go func() {
			defer group.Done()
			<-start
			results <- call()
		}()
	}
	close(start)
	group.Wait()
	close(results)
	all := make([]error, 0, workers)
	for err := range results {
		all = append(all, err)
	}
	return all
}

func countSuccessfulOTPCalls(results []error) int {
	successes := 0
	for _, err := range results {
		if err == nil {
			successes++
		}
	}
	return successes
}

func assertOnlyOneOTPConsumerWins(t *testing.T, results []error, invalidError error) {
	t.Helper()
	successes := 0
	for _, err := range results {
		if err == nil {
			successes++
		} else if !errors.Is(err, invalidError) {
			t.Errorf("unexpected concurrent OTP consume error: %v", err)
		}
	}
	if successes != 1 {
		t.Fatalf("successful OTP consumers=%d, want exactly one; errors=%v", successes, results)
	}
}

func newMySQLOTPIntegrationDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv(mysqlOTPTestDSNEnv)
	if dsn == "" {
		t.Skip("set MYSQL_OTP_IT_DSN to a disposable local MySQL Unix-socket database")
	}
	socketPath, database := parseMySQLOTPTestDSN(t, dsn)
	if !filepath.IsAbs(socketPath) || !strings.HasPrefix(filepath.Clean(socketPath), "/tmp/cue-otp-mysql-") {
		t.Fatalf("refusing MySQL OTP test socket outside its dedicated temporary path: %q", socketPath)
	}
	if !strings.HasPrefix(database, "cue_otp_p1_") {
		t.Fatalf("refusing database without disposable prefix cue_otp_p1_: %q", database)
	}

	db, err := gorm.Open(mysqlDriver.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open isolated MySQL OTP test database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get MySQL OTP test pool: %v", err)
	}
	sqlDB.SetMaxOpenConns(32)
	t.Cleanup(func() { _ = sqlDB.Close() })

	var skipNetworking int
	var isolation, version, dataDir, socket string
	if err := db.Raw("SELECT @@skip_networking, @@transaction_isolation, VERSION(), @@datadir, @@socket").Row().Scan(&skipNetworking, &isolation, &version, &dataDir, &socket); err != nil {
		t.Fatalf("inspect isolated MySQL OTP test server: %v", err)
	}
	if skipNetworking != 1 || !strings.HasPrefix(version, "8.") || !strings.EqualFold(isolation, "REPEATABLE-READ") || filepath.Clean(socket) != filepath.Clean(socketPath) {
		t.Fatalf("refusing unexpected MySQL test server settings: skip_networking=%d isolation=%q version=%q socket=%q", skipNetworking, isolation, version, socket)
	}
	if !strings.HasPrefix(filepath.Clean(dataDir), filepath.Dir(filepath.Clean(socketPath))+string(os.PathSeparator)) {
		t.Fatalf("refusing MySQL datadir outside its temporary socket directory: %q", dataDir)
	}
	if err := db.AutoMigrate(&model.VerificationCode{}, &model.EmailVerificationCode{}); err != nil {
		t.Fatalf("migrate temporary synthetic OTP tables: %v", err)
	}
	return db
}

func parseMySQLOTPTestDSN(t *testing.T, dsn string) (string, string) {
	t.Helper()
	if strings.Contains(dsn, "@tcp(") || strings.Contains(dsn, "@(") {
		t.Fatalf("refusing non-Unix-socket MySQL OTP test DSN: %q", dsn)
	}
	start := strings.Index(dsn, "@unix(")
	if start < 0 {
		t.Fatalf("refusing MySQL OTP test DSN without a Unix socket: %q", dsn)
	}
	start += len("@unix(")
	end := strings.Index(dsn[start:], ")/")
	if end < 0 {
		t.Fatalf("malformed MySQL OTP test DSN")
	}
	socketPath := dsn[start : start+end]
	databaseStart := start + end + 2
	database := strings.SplitN(dsn[databaseStart:], "?", 2)[0]
	if database == "" {
		t.Fatalf("missing disposable database name in MySQL OTP test DSN")
	}
	return socketPath, database
}
