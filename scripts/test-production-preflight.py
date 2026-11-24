#!/usr/bin/env python3
"""Regression tests for preflight-production-deploy.py using synthetic temp files only."""

from __future__ import annotations

import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parent.parent
PREFLIGHT = REPO_ROOT / "scripts" / "preflight-production-deploy.py"
SYNTHETIC_APP_PASSWORD = "synthetic-app-password-only"
SYNTHETIC_ROOT_PASSWORD = "synthetic-root-password-only"
SYNTHETIC_MISMATCH_PASSWORD = "synthetic-mismatch-password-only"


class ProductionPreflightTests(unittest.TestCase):
    def setUp(self) -> None:
        self.temp_dir = tempfile.TemporaryDirectory(prefix="cue-production-preflight-test-")
        self.addCleanup(self.temp_dir.cleanup)
        self.env_file = Path(self.temp_dir.name) / ".env.production"
        self.environment = {
            "PATH": os.defpath,
            "LC_ALL": "C",
            "MYSQL_ROOT_PASSWORD": SYNTHETIC_ROOT_PASSWORD,
            "MYSQL_APP_PASSWORD": SYNTHETIC_APP_PASSWORD,
        }

    def run_preflight(self, text: str, environment: dict[str, str] | None = None) -> subprocess.CompletedProcess[str]:
        self.env_file.write_text(text, encoding="utf-8")
        return subprocess.run(
            [sys.executable, str(PREFLIGHT), "--env-file", str(self.env_file)],
            cwd=REPO_ROOT,
            env=self.environment if environment is None else environment,
            text=True,
            capture_output=True,
            check=False,
        )

    @staticmethod
    def valid_env(dsn: str | None = None) -> str:
        return "\n".join(
            (
                "APP_ENV=production",
                "APP_URL=https://api.synthetic.invalid",
                "FRONTEND_URL=https://app.synthetic.invalid",
                "ALIPAY_NOTIFY_URL=https://api.synthetic.invalid/callback/alipay",
                "ALIPAY_RETURN_URL=https://app.synthetic.invalid/payment/return",
                f"JWT_SECRET={'j' * 40}",
                f"AES_SECRET={'a' * 32}",
                "INITIAL_ADMIN_USERNAME=synthetic-admin",
                f"INITIAL_ADMIN_PASSWORD={'p' * 20}",
                "MYSQL_DSN="
                + (
                    dsn
                    if dsn is not None
                    else f"saas_user:{SYNTHETIC_APP_PASSWORD}@tcp(mysql:3306)/saas_billing?charset=utf8mb4&parseTime=True&loc=Local"
                ),
                "",
            )
        )

    def assert_no_synthetic_passwords_printed(self, result: subprocess.CompletedProcess[str]) -> None:
        output = result.stdout + result.stderr
        for value in (SYNTHETIC_APP_PASSWORD, SYNTHETIC_ROOT_PASSWORD, SYNTHETIC_MISMATCH_PASSWORD):
            self.assertNotIn(value, output)

    def test_valid_synthetic_configuration_passes(self) -> None:
        result = self.run_preflight(self.valid_env())
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("preflight passed", result.stdout)
        self.assert_no_synthetic_passwords_printed(result)

    def test_missing_required_environment_file_value_fails(self) -> None:
        text = self.valid_env().replace("FRONTEND_URL=https://app.synthetic.invalid\n", "")
        result = self.run_preflight(text)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("FRONTEND_URL", result.stderr)
        self.assert_no_synthetic_passwords_printed(result)

    def test_missing_compose_password_fails(self) -> None:
        environment = dict(self.environment)
        del environment["MYSQL_ROOT_PASSWORD"]
        result = self.run_preflight(self.valid_env(), environment)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("MYSQL_ROOT_PASSWORD", result.stderr)
        self.assert_no_synthetic_passwords_printed(result)

    def test_missing_app_password_fails(self) -> None:
        environment = dict(self.environment)
        del environment["MYSQL_APP_PASSWORD"]
        result = self.run_preflight(self.valid_env(), environment)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("MYSQL_APP_PASSWORD", result.stderr)
        self.assert_no_synthetic_passwords_printed(result)

    def test_wrong_compose_host_fails(self) -> None:
        dsn = f"saas_user:{SYNTHETIC_APP_PASSWORD}@tcp(other-db:3306)/saas_billing?charset=utf8mb4"
        result = self.run_preflight(self.valid_env(dsn))
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("mysql:3306/saas_billing", result.stderr)
        self.assert_no_synthetic_passwords_printed(result)

    def test_wrong_database_fails(self) -> None:
        dsn = f"saas_user:{SYNTHETIC_APP_PASSWORD}@tcp(mysql:3306)/another_db?charset=utf8mb4"
        result = self.run_preflight(self.valid_env(dsn))
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("mysql:3306/saas_billing", result.stderr)
        self.assert_no_synthetic_passwords_printed(result)

    def test_mismatched_dsn_password_fails_without_revealing_values(self) -> None:
        dsn = f"saas_user:{SYNTHETIC_MISMATCH_PASSWORD}@tcp(mysql:3306)/saas_billing?charset=utf8mb4"
        result = self.run_preflight(self.valid_env(dsn))
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("does not match MYSQL_APP_PASSWORD", result.stderr)
        self.assert_no_synthetic_passwords_printed(result)

    def test_placeholder_or_invalid_secret_fails_without_revealing_values(self) -> None:
        text = self.valid_env().replace(f"AES_SECRET={'a' * 32}", "AES_SECRET=replace-with-a-32-byte-random-key")
        result = self.run_preflight(text)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("AES_SECRET", result.stderr)
        self.assert_no_synthetic_passwords_printed(result)

    def test_example_domain_url_fails(self) -> None:
        text = self.valid_env().replace(
            "ALIPAY_NOTIFY_URL=https://api.synthetic.invalid/callback/alipay",
            "ALIPAY_NOTIFY_URL=https://api.example.com/callback/alipay",
        )
        result = self.run_preflight(text)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("ALIPAY_NOTIFY_URL", result.stderr)
        self.assert_no_synthetic_passwords_printed(result)


if __name__ == "__main__":
    unittest.main(verbosity=2)
