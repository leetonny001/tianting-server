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
