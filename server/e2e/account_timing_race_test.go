//go:build race

package e2e

import "tianting/server/internal/account"

// timingArgon：-race 下真实参数单次超过 1 秒，100 次太久，用 8 MiB。比较的是同一段代码路径，参数不影响结论；
// 生产参数的结果由不带 -race 的 go test 覆盖。
func timingArgon() account.ArgonParams {
	return account.ArgonParams{MemoryKiB: 8 * 1024, Time: 1, Threads: 1}
}

// raceEnabled：竞态检测的开销会盖过时序用例要测的差异。
const raceEnabled = true
