//go:build !race

package e2e

import "tianting/server/internal/account"

// timingArgon：AC-ACC-02-3 用生产参数。
func timingArgon() account.ArgonParams { return account.DefaultArgon }

const raceEnabled = false
