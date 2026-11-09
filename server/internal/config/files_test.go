package config

import (
	"strings"
	"testing"
	"time"
)

// BE-431：IM_FILES_ENABLED 和文件配置（PRD-chat 第 4.1 节）；对象存储配置（IM_OBJSTORE_*，BE-430 第 3 条）。
func TestFilesConfig(t *testing.T) {
	cfg, _, err := Load(envOf(map[string]string{}))
	if err != nil || cfg.App.Files.Enabled {
		t.Fatalf("files must be off by default: %+v %v", cfg.App.Files, err)
	}
	base := map[string]string{
		"IM_FILES_ENABLED": "1", "IM_OBJSTORE_ENDPOINT": "http://127.0.0.1:9000", "IM_OBJSTORE_BUCKET": "chat",
		"IM_OBJSTORE_PUBLIC_BUCKET": "avatars", "IM_OBJSTORE_ACCESS_KEY": "ak", "IM_OBJSTORE_SECRET_KEY": "test-only-secret-value",
		"IM_FILE_URL_TTL_IMAGE": "5", "IM_UPLOAD_PUT_TTL": "2m", "IM_FILE_RETENTION_DAYS": "0", "IM_UPLOAD_DAILY_BYTES": "1000",
	}
	cfg, _, err = Load(envOf(base))
	if err != nil {
		t.Fatal(err)
	}
	f := cfg.App.Files
	if !f.Enabled || f.URLTTLImage != 5*time.Second || f.PutTTL != 2*time.Minute || f.RetentionDays == nil || *f.RetentionDays != 0 || f.DailyBytes != 1000 {
		t.Fatalf("files = %+v", f)
	}
	if cfg.App.ObjStore.Bucket != "chat" || cfg.App.ObjStore.PublicBucket != "avatars" {
		t.Fatalf("objstore = %+v", cfg.App.ObjStore)
	}
	for name, change := range map[string]map[string]string{
		"missing endpoint": {"IM_OBJSTORE_ENDPOINT": ""},
		"same bucket":      {"IM_OBJSTORE_PUBLIC_BUCKET": "chat"},
		"bad ttl":          {"IM_FILE_URL_TTL_FILE": "0"},
		"bad retention":    {"IM_FILE_RETENTION_DAYS": "-1"},
		"bad bytes":        {"IM_FILE_MAX_BYTES": "x"},
		"bad blocklist":    {"IM_FILE_BLOCKLIST": "/nonexistent/blocklist.txt"},
	} {
		env := map[string]string{}
		for k, v := range base {
			env[k] = v
		}
		for k, v := range change {
			env[k] = v
		}
		_, _, err := Load(envOf(env))
		if err == nil {
			t.Errorf("%s: expected error", name)
		} else if strings.Contains(err.Error(), "test-only-secret-value") {
			t.Errorf("%s: secret in error", name)
		}
	}
	// 关闭时不检查对象存储配置。
	if _, _, err := Load(envOf(map[string]string{"IM_FILES_ENABLED": "0"})); err != nil {
		t.Fatalf("files off: %v", err)
	}
}

// 生产环境：对象存储地址和公开桶域名必须是 https，公开桶必须有独立域名。
func TestFilesConfigProduction(t *testing.T) {
	_, _, err := Load(envOf(map[string]string{
		"IM_ENV": "production", "IM_FILES_ENABLED": "1", "IM_OBJSTORE_ENDPOINT": "http://s3.internal", "IM_OBJSTORE_BUCKET": "chat",
		"IM_OBJSTORE_PUBLIC_BUCKET": "avatars", "IM_OBJSTORE_ACCESS_KEY": "ak", "IM_OBJSTORE_SECRET_KEY": "sk",
	}))
	if err == nil || !strings.Contains(err.Error(), "IM_OBJSTORE_ENDPOINT and IM_OBJSTORE_PUBLIC_BASE_URL must be https") || !strings.Contains(err.Error(), "IM_OBJSTORE_PUBLIC_BASE_URL is required") {
		t.Fatalf("err = %v", err)
	}
}

// BE-438 第 3 条（PRD-chat 第 14.2 节、F-FILE-01 规则 10）：配置检查有 objstore_https；生产环境只要有一个地址不是 https 就拒绝启动，
// 报错和检查项都不带地址和凭证。
func TestObjstoreHTTPSCheck(t *testing.T) {
	files := map[string]string{
		"IM_FILES_ENABLED": "1", "IM_OBJSTORE_ENDPOINT": "https://s3.internal.example", "IM_OBJSTORE_BUCKET": "chat",
		"IM_OBJSTORE_PUBLIC_BUCKET": "avatars", "IM_OBJSTORE_PUBLIC_BASE_URL": "https://avatars.example.com",
		"IM_OBJSTORE_ACCESS_KEY": "ak-placeholder", "IM_OBJSTORE_SECRET_KEY": "sk-test-only-value",
	}
	find := func(cfg Server) Check {
		t.Helper()
		for _, c := range productionChecks(cfg) {
			if c.ID == "objstore_https" {
				return c
			}
		}
		t.Fatal("no objstore_https check")
		return Check{}
	}
	cfg, err := prodOps(t, files)
	if err != nil {
		t.Fatal(err)
	}
	if c := find(cfg); !c.Passed || !strings.Contains(c.Detail, "https") || strings.Contains(c.Detail, "s3.internal.example") || strings.Contains(c.Detail, "sk-test-only-value") {
		t.Errorf("check = %+v", c)
	}
	off, err := prodOps(t, nil)
	if err != nil {
		t.Fatal(err)
	}
	if c := find(off); !c.Passed || !strings.Contains(c.Detail, "关闭") {
		t.Errorf("files off: %+v", c)
	}
	for name, change := range map[string]map[string]string{
		"http endpoint":       {"IM_OBJSTORE_ENDPOINT": "http://s3.internal.example"},
		"http public domain":  {"IM_OBJSTORE_PUBLIC_BASE_URL": "http://avatars.example.com"},
		"no public domain":    {"IM_OBJSTORE_PUBLIC_BASE_URL": ""},
		"scheme-less address": {"IM_OBJSTORE_ENDPOINT": "s3.internal.example"},
	} {
		env := map[string]string{}
		for k, v := range files {
			env[k] = v
		}
		for k, v := range change {
			env[k] = v
		}
		_, err := prodOps(t, env)
		if err == nil {
			t.Errorf("%s: production must refuse to start", name)
			continue
		}
		if strings.Contains(err.Error(), "sk-test-only-value") || strings.Contains(err.Error(), "s3.internal.example") {
			t.Errorf("%s: address or secret in error: %v", name, err)
		}
	}
	// 开发环境允许 http（MinIO 开发环境，F-FILE-01 规则 10 的客户端例外见部署说明），检查项照实显示。
	dev, _, err := Load(envOf(map[string]string{"IM_FILES_ENABLED": "1", "IM_OBJSTORE_ENDPOINT": "http://127.0.0.1:9000", "IM_OBJSTORE_BUCKET": "chat",
		"IM_OBJSTORE_PUBLIC_BUCKET": "avatars", "IM_OBJSTORE_ACCESS_KEY": "ak", "IM_OBJSTORE_SECRET_KEY": "sk"}))
	if err != nil {
		t.Fatalf("development http: %v", err)
	}
	if c := find(dev); c.Passed || !strings.Contains(c.Detail, "不是 https") {
		t.Errorf("development check = %+v", c)
	}
}
