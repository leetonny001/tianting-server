package protocol

import (
	"strings"
	"testing"
)

// TC-CHAT-90-46 的补充（BE-491 第 1 条）：Schema 不写、只由服务端判的两个上限。msgIds 超过 20 条 4131（AC-MOD-03-2），
// note 超过 200 个码点 4000（AC-MOD-03-13）；码点不是 UTF-16 单位，200 个 emoji（400 个 UTF-16 单位）仍然合法。
func TestReportReqServerOnlyLimits(t *testing.T) {
	ids := func(n int) []string {
		out := make([]string, n)
		for i := range out {
			out[i] = "m" + strings.Repeat("x", i+1)
		}
		return out
	}
	note := func(s string, n int) *string { v := strings.Repeat(s, n); return &v }
	msg := func(n int) ReportReq {
		return ReportReq{TargetType: ReportTargetMessage, ConvID: "group:g1", MsgIDs: ids(n), Reason: "abuse"}
	}
	for _, tc := range []struct {
		name string
		req  ReportReq
		code int
	}{
		{"20 条", msg(20), 0},
		{"21 条", msg(21), CodeBatchLimitExceeded},
		{"note 200 个汉字", ReportReq{TargetType: ReportTargetUser, UserID: "u9", Reason: "spam", Note: note("举", 200)}, 0},
		{"note 200 个 emoji", ReportReq{TargetType: ReportTargetUser, UserID: "u9", Reason: "spam", Note: note("😀", 200)}, 0},
		{"note 201 个码点", ReportReq{TargetType: ReportTargetGroup, GroupID: "g1", Reason: "spam", Note: note("举", 201)}, CodeBadRequest},
		{"21 条且 reason 不合法：格式先报 4000", ReportReq{TargetType: ReportTargetMessage, ConvID: "group:g1", MsgIDs: ids(21), Reason: "xxx"}, CodeBadRequest},
	} {
		err := tc.req.Validate()
		switch {
		case tc.code == 0 && err != nil:
			t.Errorf("%s: %v", tc.name, err)
		case tc.code != 0 && (err == nil || err.Code != tc.code):
			t.Errorf("%s: got %v, want %d", tc.name, err, tc.code)
		}
	}
}
