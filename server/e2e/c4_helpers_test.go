package e2e

// C4 撤回、引用、@ 的 e2e 公共设施（BE-472～BE-477）。环境 E1：测试模式、SQLite、IM_MSG_ACTIONS_ENABLED=1、
// IM_QUARANTINE_KEY 用测试密钥；后台 AC 要 IM_V13_ADMIN_FEATURES=1（第 4.1 节），所以默认一起打开。时钟是 [IM 钟]（冻结在 t0）。

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"tianting/server/internal/app"
	"tianting/server/internal/protocol"
)

type c4Env struct {
	*modEnv
}

// newC4Env：发送限速调高（冻结时钟下令牌不恢复），撤回频率也调高；只有测频率的用例用默认值。
func newC4Env(t *testing.T, mutate func(*app.Config)) *c4Env {
	t.Helper()
	m := newModEnv(t, func(c *app.Config) {
		c.SendRate = 10000
		c.MsgActions.Enabled = true
		c.MsgActions.RecallRatePerMin = 10000
		c.MsgActions.MemberSearchRatePerMin = 10000
		if mutate != nil {
			mutate(c)
		}
	})
	return &c4Env{modEnv: m}
}

// selfGroup 建一个自建群：owner 是群主，admins 是管理员，其余是成员。返回 groupId 和每个人的连接（已经收掉建群的帧）。
func (e *c4Env) selfGroup(owner string, admins []string, members ...string) (string, map[string]*client) {
	e.t.Helper()
	if len(admins)+len(members) < 2 { // 建群至少要 2 个其他成员（F-SOC-21）
		members = append(members, owner+"-filler")
	}
	all := append(append([]string{owner}, admins...), members...)
	e.usersNamed(all...)
	e.mutual(owner, append(append([]string{}, admins...), members...)...)
	cs := map[string]*client{}
	for _, u := range all {
		cs[u] = e.dial(u, "web")
	}
	gid := createG(e.t, cs[owner], append(append([]string{}, admins...), members...)...).GroupID
	for _, a := range admins {
		mustAck[protocol.Empty](e.t, cs[owner].call(protocol.CmdGroupSetAdmin, map[string]any{"groupId": gid, "userId": a, "admin": true}))
	}
	for _, c := range cs {
		c.drain()
	}
	return gid, cs
}

// drain 丢掉已经收到的帧（建群提示、convAdded 等），之后的断言只看新帧。
func (c *client) drain() {
	c.t.Helper()
	time.Sleep(50 * time.Millisecond)
	c.expectNoFrameDrain()
}

func recallCall(c *client, convID, msgID string) frame {
	c.t.Helper()
	return c.call(protocol.CmdRecall, map[string]any{"convId": convID, "msgId": msgID})
}

func mustRecall(t *testing.T, c *client, convID, msgID string) {
	t.Helper()
	mustAck[protocol.Empty](t, recallCall(c, convID, msgID))
}

func sendBody(c *client, convType, to, cid string, body map[string]any) frame {
	c.t.Helper()
	return c.call(protocol.CmdSend, map[string]any{"convType": convType, "to": to, "clientMsgId": cid, "type": protocol.MsgText, "body": body})
}

func sendQuote(c *client, convType, to, cid, text, quoteID string) frame {
	c.t.Helper()
	return sendBody(c, convType, to, cid, map[string]any{"text": text, "quote": map[string]any{"msgId": quoteID}})
}

func sendMentions(c *client, gid, cid, text string, userIDs []string, all bool) frame {
	c.t.Helper()
	m := map[string]any{}
	if userIDs != nil {
		m["userIds"] = userIDs
	}
	if all {
		m["all"] = true
	}
	return sendBody(c, protocol.ConvGroup, gid, cid, map[string]any{"text": text, "mentions": m})
}

// waitRecalled 等这个会话的 msgDeleted 事件（跳过其他事件）。
func waitMsgDeleted(t *testing.T, c *client) (protocol.MsgDeletedEvent, json.RawMessage) {
	t.Helper()
	raw := c.waitEvent(protocol.EventMsgDeleted)
	var ev protocol.MsgDeletedEvent
	if err := json.Unmarshal(raw, &ev); err != nil {
		t.Fatal(err)
	}
	return ev, raw
}

// expectNoMsgDeleted 用哨兵法断言没有 msgDeleted：from 给 c 所在的会话再发一条，c 收到它之前没有 msgDeleted（第 4.4 节）。
func expectNoMsgDeleted(t *testing.T, c *client, sentinel func()) {
	t.Helper()
	for _, f := range c.pending { // 等 ack 时顺带收下的帧也要看
		var head struct{ Type string }
		json.Unmarshal(f.Data, &head)
		if f.Cmd == protocol.CmdEvent && head.Type == protocol.EventMsgDeleted {
			t.Fatalf("%s: unexpected msgDeleted %s", c.uid, f.Data)
		}
	}
	c.pending = nil
	sentinel()
	for {
		f := c.next()
		if f.Cmd == protocol.CmdMsg {
			return
		}
		if f.Cmd == protocol.CmdEvent {
			var head struct{ Type string }
			json.Unmarshal(f.Data, &head)
			if head.Type == protocol.EventMsgDeleted {
				t.Fatalf("%s: unexpected msgDeleted %s", c.uid, f.Data)
			}
		}
	}
}

func (e *c4Env) db() *sql.DB {
	e.t.Helper()
	db, err := sql.Open("sqlite", e.dbPath)
	if err != nil {
		e.t.Fatal(err)
	}
	e.t.Cleanup(func() { db.Close() })
	return db
}

// rawDB 做检查点后读主库和 WAL 的原始字节（"搜不到"指对原始字节做子串搜索，TC-RCL 通用约定）。
func rawDB(t *testing.T, path string) []byte {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		t.Fatal(err)
	}
	var all []byte
	for _, suffix := range []string{"", "-wal"} {
		b, err := os.ReadFile(path + suffix)
		if err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		all = append(all, b...)
	}
	return all
}

func syncConv(t *testing.T, c *client, convID string) protocol.SyncAck {
	t.Helper()
	return mustAck[protocol.SyncAck](t, c.call(protocol.CmdSync, map[string]any{"convId": convID, "afterSeq": 0, "limit": 200}))
}

func syncRaw(t *testing.T, c *client, convID string) json.RawMessage {
	t.Helper()
	f := c.call(protocol.CmdSync, map[string]any{"convId": convID, "afterSeq": 0, "limit": 200})
	if f.Error != nil {
		t.Fatalf("sync %s: %+v", convID, f.Error)
	}
	return f.Data
}

func msgBySeq(t *testing.T, msgs []protocol.Message, seq int64) protocol.Message {
	t.Helper()
	for _, m := range msgs {
		if m.Seq == seq {
			return m
		}
	}
	t.Fatalf("no message with seq %d", seq)
	return protocol.Message{}
}

func convByID(t *testing.T, c *client, convID string) protocol.Conversation {
	t.Helper()
	for _, x := range mustAck[protocol.ConvsAck](t, c.call(protocol.CmdConvs, map[string]any{})).Convs {
		if x.ConvID == convID {
			return x
		}
	}
	t.Fatalf("%s: no conversation %s", c.uid, convID)
	return protocol.Conversation{}
}

func mentionUnread(t *testing.T, c *client, convID string) (int64, int64) {
	t.Helper()
	cv := convByID(t, c, convID)
	if cv.MentionUnread == nil {
		t.Fatalf("%s: convs has no mentionUnread for %s", c.uid, convID)
	}
	return *cv.MentionUnread, cv.FirstMentionSeq
}

// frameSchema 编译 protocol/schema/frame.schema.json，校验 e2e 抓到的真实下行帧（AC-RCL-03-7）。
func frameSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	path, _ := filepath.Abs("../../protocol/schema/frame.schema.json")
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	doc, err := jsonschema.UnmarshalJSON(f)
	if err != nil {
		t.Fatal(err)
	}
	c := jsonschema.NewCompiler()
	const id = "https://im-mvp.local/protocol/frame.schema.json"
	if err := c.AddResource(id, doc); err != nil {
		t.Fatal(err)
	}
	s, err := c.Compile(id)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func validFrame(t *testing.T, s *jsonschema.Schema, label string, fr any) {
	t.Helper()
	if f, ok := fr.(frame); ok { // e2e 的宽松结构会多出 "error": null，按线上的样子重组
		m := map[string]any{"cmd": f.Cmd}
		if f.ID != "" {
			m["id"] = f.ID
		}
		if f.Data != nil {
			m["data"] = f.Data
		}
		if f.Error != nil {
			m["error"] = f.Error
		}
		fr = m
	}
	b, _ := json.Marshal(fr)
	v, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Validate(v); err != nil {
		t.Errorf("%s does not match frame.schema.json: %v\n%s", label, err, b)
	}
}

// metricValue 读 /metrics 里一行计数器的值（没有时为 0）。
func (e *env) metricValue(line string) float64 {
	e.t.Helper()
	rec := httptest.NewRecorder()
	e.app.MetricsHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	for _, l := range bytes.Split(rec.Body.Bytes(), []byte("\n")) {
		if bytes.HasPrefix(l, []byte(line+" ")) {
			var v float64
			fmt.Sscanf(string(l[len(line)+1:]), "%g", &v)
			return v
		}
	}
	return 0
}
