package account

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"runtime/debug"
	"strings"

	"golang.org/x/crypto/argon2"
)

// release 立刻把 Argon2id 的工作内存还给操作系统（和 console-server 同样的做法）：每次计算分配 64 MiB，
// 交给 GC 的节奏回收时 RSS 增量会超过 AC-CHAT-90-11 的上限。登录是低频操作，多一次 GC 的成本可以接受。
func release() { debug.FreeOSMemory() }

// hashPassword 返回 PHC 格式的哈希：$argon2id$v=19$m=...,t=...,p=...$salt$hash。调用方要先拿到闸门的位（gate.go）。
func hashPassword(p ArgonParams, password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	sum := argon2.IDKey([]byte(password), salt, p.Time, p.MemoryKiB, p.Threads, 32)
	release()
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, p.MemoryKiB, p.Time, p.Threads,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(sum)), nil
}

// verifyPassword 校验密码，参数取自哈希本身：改默认参数之后旧密码还能验证。
// current 是现在的默认参数，哈希的参数和它不同时 stale=true，登录成功后要重算（F-ACC-02 异常）。
func verifyPassword(current ArgonParams, encoded, password string) (ok, stale bool, err error) {
	var v int
	var p ArgonParams
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, false, errors.New("not an argon2id hash")
	}
	if _, err := fmt.Sscanf(parts[2], "v=%d", &v); err != nil || v != argon2.Version {
		return false, false, errors.New("unsupported argon2 version")
	}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.MemoryKiB, &p.Time, &p.Threads); err != nil {
		return false, false, errors.New("bad argon2 parameters")
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, false, errors.New("bad argon2 salt")
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(want) == 0 {
		return false, false, errors.New("bad argon2 hash")
	}
	got := argon2.IDKey([]byte(password), salt, p.Time, p.MemoryKiB, p.Threads, uint32(len(want))) // #nosec G115 -- 长度来自自己写入的 32 字节哈希
	release()
	return subtle.ConstantTimeCompare(got, want) == 1, p != current, nil
}

// dummyHash 是"不存在的账号"也要校验的哈希（F-ACC-02 规则 2）：用现在的默认参数，不用真实密码，所以不可能校验通过。
// 启动时算一次，之后每次请求都做一次相同参数的计算，响应时间不可区分。
func dummyHash(p ArgonParams) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hashPassword(p, base64.RawStdEncoding.EncodeToString(b))
}
