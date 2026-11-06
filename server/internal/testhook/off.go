//go:build !imhooks

// Package testhook 是账号并发钩子（PRD-chat 第 4.5 节，RS4-06，BE-405）：让请求停在 Argon2 计算之后、提交事务之前，
// 确定性地构造 F-ACC-04 规则 8 的交错。**只有 `-tags imhooks` 的构建才有**：不带标签时这里全是空操作，
// 生产二进制不含钩子的符号（internal/archtest 用 go tool nm 断言），也没有任何接口能碰到它。
package testhook

// Compiled 表示这个二进制带钩子。生产环境带钩子的二进制拒绝启动（config.productionChecks）。
const Compiled = false

// AfterArgon 在 Argon2 计算之后、提交事务之前调用。key 是用户名的哈希。
func AfterArgon(string) {}

// BeforeRegister 在连接校验之后、入表之前调用。key 是 userId。
func BeforeRegister(string) {}
