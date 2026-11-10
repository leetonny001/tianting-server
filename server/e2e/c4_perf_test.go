package e2e

// AC-CHAT-90-1 的 C4 部分（TC-CHAT-90-27）的开发机记录：带引用的单聊、带 @ 的群聊各 1000 条的 ack 延迟。
// 判定在 EP（P-12）：开发机结果**只记录，不算通过**；IM_PERF_DEV=1 才运行，3 次用 -count=3。顺序发送，不是 500 条/秒。

import (
	"fmt"
	"testing"

	"tianting/server/internal/app"
	"tianting/server/internal/protocol"
)

// TC-CHAT-90-27（开发机记录）：u1 给在线的 u2 发 1000 条引用 m0 的单聊文本；群（o、m、n 在线）里 o 发 1000 条 @ m 的文本。
// 断言只有功能部分（都成功、对方都收到），延迟只打印。
func TestTC_CHAT_90_27_QuoteAndMentionAckDevRecord(t *testing.T) {
	perfDevOnly(t)
	e := newC4Env(t, func(c *app.Config) { c.SendRate = 1 << 20 })
	e.usersNamed("u1", "u2")
	e.mutual("u1", "u2")
	u1, u2 := e.dial("u1", "web"), e.dial("u2", "web")
	m0 := mustAck[protocol.SendAck](t, u2.sendText("c2c", "u1", "cmid-perf-q0", "the quoted message"))
	d := ackLatencies(t, 1000, func(i int) frame {
		return sendQuote(u1, "c2c", "u2", fmt.Sprintf("cmid-perf-q%06d", i+1), "reply with quote", m0.MsgID)
	})
	logPerf(t, "c2c text with quote", d)
	waitMsgs(t, []*client{u2}, 1000)

	gid, cs := e.selfGroup("o", nil, "m", "n")
	d = ackLatencies(t, 1000, func(i int) frame {
		return sendMentions(cs["o"], gid, fmt.Sprintf("cmid-perf-m%06d", i), "@m perf", []string{"m"}, false)
	})
	logPerf(t, "group text with @member", d)
	waitMsgs(t, []*client{cs["m"], cs["n"]}, 1000)
}
