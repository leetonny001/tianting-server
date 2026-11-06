package e2e

// protocol/vectors/account-session.json 的 serverCases（BE-402，AC-CHAT-90-15 的服务端部分）：参考模型产出期望，这里用真实服务端逐步断言。
// 客户端部分（cases）由三套 SDK 跑（FE-600～602）。

import (
	"fmt"
	"net/http"
	"os"
	"slices"
	"testing"
	"time"

	"tianting/server/e2e/accountmodel"
	"tianting/server/internal/protocol"
)

type vectorFile struct {
	ServerCases []accountmodel.ServerCase `json:"serverCases"`
}

func TestAccountSessionServerVectors(t *testing.T) {
	raw, err := os.ReadFile("../../protocol/vectors/account-session.json")
	if err != nil {
		t.Fatal(err)
	}
	var f vectorFile
	mustJSON(t, raw, &f)
	if len(f.ServerCases) == 0 {
		t.Fatal("no serverCases")
	}
	for _, ws := range []bool{false, true} {
		channel := map[bool]string{false: "http", true: "ws"}[ws]
		for i, c := range f.ServerCases {
			t.Run(fmt.Sprintf("%s/%d/%s", channel, i, c.Name), func(t *testing.T) {
				e := openEnv(t, nil)
				clk := e.freeze()
				login := e.register("alice_1", nil)
				tokens := map[int64]authResp{1: login}
				sid := sidOf(login.RefreshToken)
				for si, st := range c.Steps {
					label := fmt.Sprintf("step %d %s gen=%d", si, st.Op, st.Gen)
					switch st.Op {
					case "advance":
						clk.advance(time.Duration(st.Ms) * time.Millisecond)
					case "refresh":
						for range st.Repeat {
							r := e.mustRefresh(tokens[st.Gen].RefreshToken)
							tokens[genOf(t, r.RefreshToken)] = r
						}
						if st.Repeat > 0 {
							break
						}
						code, b := e.refresh(tokens[st.Gen].RefreshToken)
						switch st.Result {
						case accountmodel.ResultIssued, accountmodel.ResultPromoted:
							if code != http.StatusOK {
								t.Fatalf("%s: %d %s", label, code, b)
							}
							var r authResp
							mustJSON(t, b, &r)
							if g := genOf(t, r.RefreshToken); g != st.NewGen {
								t.Fatalf("%s: new gen %d, want %d", label, g, st.NewGen)
							}
							tokens[st.NewGen] = r
						case accountmodel.ResultLimited:
							wantErr(t, label, code, b, http.StatusTooManyRequests, protocol.CodeRateLimited, "")
						case accountmodel.ResultSuperseded:
							wantErr(t, label, code, b, http.StatusUnauthorized, protocol.CodeRefreshTokenInvalid, protocol.ReasonRefreshSuperseded)
						case accountmodel.ResultReplay, accountmodel.ResultRevoked:
							wantErr(t, label, code, b, http.StatusUnauthorized, protocol.CodeRefreshTokenInvalid, protocol.ReasonReplayDetected)
						default:
							t.Fatalf("%s: unknown result %s", label, st.Result)
						}
					case "access":
						access := tokens[st.Gen].AccessToken
						var got string
						if ws {
							c := e.connect("alice_1", access, testPlatform)
							got = accountmodel.AccessOK
							if len(c.pending) != 0 {
								got = kickReason(t, c)
							}
							c.ws.CloseNow()
						} else {
							code, b := e.get("/v1/account/events", access)
							switch {
							case code == http.StatusOK:
								got = accountmodel.AccessOK
							case parseErr(t, b).Code == protocol.CodeTokenExpired && parseErr(t, b).Reason == protocol.ReasonTokenSuperseded:
								got = accountmodel.AccessRejected
							case parseErr(t, b).Code == protocol.CodeTokenRevoked:
								got = accountmodel.AccessRevoked
							default:
								t.Fatalf("%s: %d %s", label, code, b)
							}
						}
						want := st.Result
						if want == accountmodel.AccessPromoted {
							want = accountmodel.AccessOK // 服务端不区分，下面看 current
						}
						if got != want {
							t.Fatalf("%s: %s, want %s", label, got, want)
						}
					}
					d := e.session(sid)
					if d.CurrentGen != st.Current || (d.RevokedAt != 0) != st.Revoked || d.OverlapMinGen != st.MinGen {
						t.Fatalf("%s: current %d revoked %v minGen %d; want %+v", label, d.CurrentGen, d.RevokedAt != 0, d.OverlapMinGen, st)
					}
					gens := []int64{}
					for _, p := range d.Pendings {
						gens = append(gens, p.Gen)
					}
					if !slices.Equal(gens, st.Pendings) {
						t.Fatalf("%s: pendings %v, want %v", label, gens, st.Pendings)
					}
				}
			})
		}
	}
}

func kickReason(t *testing.T, c *client) string {
	t.Helper()
	var ev protocol.KickedEvent
	mustJSON(t, c.pending[0].Data, &ev)
	return ev.Reason
}
