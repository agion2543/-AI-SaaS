package router

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type healthResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Status string            `json:"status"`
		Checks map[string]string `json:"checks"`
	} `json:"data"`
}

func newHealthTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open synthetic health-check database: %v", err)
	}
	return db
}

func healthTestEngine(db *gorm.DB) *gin.Engine {
	engine := gin.New()
	registerHealthRoutes(engine, db)
	return engine
}

func requestHealth(t *testing.T, engine *gin.Engine, path string) (int, healthResponse, string) {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", path, nil)
	engine.ServeHTTP(recorder, request)

	var response healthResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode %s response: %v; body=%s", path, err, recorder.Body.String())
	}
	return recorder.Code, response, recorder.Body.String()
}

func TestReadinessReportsDatabaseReady(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := healthTestEngine(newHealthTestDB(t))

	status, response, _ := requestHealth(t, engine, "/ready")
	if status != 200 || response.Data.Status != "ready" || response.Data.Checks["database"] != "ready" {
		t.Fatalf("unexpected readiness response: http=%d body=%+v", status, response)
	}
}

func TestReadinessReportsDatabaseUnavailableWithoutLeakingDetails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newHealthTestDB(t)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get synthetic SQL database: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("close synthetic SQL database: %v", err)
	}
	engine := healthTestEngine(db)

	status, response, body := requestHealth(t, engine, "/ready")
	if status != 503 || response.Data.Status != "not_ready" || response.Data.Checks["database"] != "unavailable" {
		t.Fatalf("unexpected unavailable readiness response: http=%d body=%+v", status, response)
	}
	if strings.Contains(body, "dsn") || strings.Contains(body, "password") || strings.Contains(body, "sqlite") {
		t.Fatalf("readiness response leaked connection details: %s", body)
	}
}

func TestLivenessDoesNotDependOnDatabaseReadiness(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newHealthTestDB(t)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get synthetic SQL database: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("close synthetic SQL database: %v", err)
	}

	status, response, _ := requestHealth(t, healthTestEngine(db), "/health")
	if status != 200 || response.Data.Status != "ok" {
		t.Fatalf("liveness should report a running process even while DB is unavailable: http=%d body=%+v", status, response)
	}
}
