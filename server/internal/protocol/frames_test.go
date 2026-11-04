package protocol

import (
	"encoding/json"
	"strings"
	"testing"
)

func ptr[T any](v T) *T { return &v }

// Go 结构体编码出来的下行帧必须符合 Schema，防止代码和协议文档悄悄分叉。
func TestOutFramesMatchSchema(t *testing.T) {
	schema := compileSchema(t)
	msg := Message{
		MsgID: "m1", ClientMsgID: "cid-00000001", ConvID: C2CConvID("u2", "u1"), Seq: 1,
		From: "u1", Type: MsgText, Body: MsgBody{Text: "hi"}, TS: 1730000000000,
	}
	conv := Conversation{
		ConvID: "c2c:u1:u2", Type: ConvC2C, Name: "u2", MaxSeq: 3, ReadSeq: 1, MinSeq: 1, Unread: 2,
		LastMsg: &LastMsg{MsgID: "m3", From: "u2", Type: MsgText, Text: "hey", TS: 1}, UpdatedAt: 1,
	}
	emptyGroup := Conversation{ConvID: "group:g1", Type: ConvGroup, Name: "g", MinSeq: 1, UpdatedAt: 1}
	frames := map[string]OutFrame{
		"auth ack":       AckOK("a1", AuthAck{UserID: "alice", ServerTime: 1}),
		"send ack":       AckOK("r1", SendAck{MsgID: "m1", ConvID: "c2c:u1:u2", Seq: 1, TS: 1}),
		"sync ack":       AckOK("r2", SyncAck{Messages: []Message{msg}, HasMore: true}),
		"sync ack empty": AckOK("r2", SyncAck{Messages: []Message{}}),
		"convs ack":      AckOK("r3", ConvsAck{Convs: []Conversation{conv, emptyGroup}}),
		"convs empty":    AckOK("r3", ConvsAck{Convs: []Conversation{}}),
		"empty ack":      AckOK("r4", Empty{}),
		"error ack":      AckErr("r5", Errorf(CodeNotGroupMember, "not a member")),
		"error no id":    AckErr("", Errorf(CodeInvalidFrame, "invalid frame")),
		"msg":            {Cmd: CmdMsg, Data: msg},
		"custom msg": {Cmd: CmdMsg, Data: Message{
			MsgID: "m2", ClientMsgID: "cid-00000002", ConvID: GroupConvID("g1"), Seq: 2, From: "u1", Type: MsgCustom,
			Body: MsgBody{Data: json.RawMessage(`{"k":1}`)}, TS: 1,
		}},
		"kicked":         Event(KickedEvent{Type: EventKicked, Reason: KickTokenInvalid}),
		"conv added":     Event(ConvAddedEvent{Type: EventConvAdded, Conv: emptyGroup}),
		"conv removed":   Event(ConvRemovedEvent{Type: EventConvRemoved, ConvID: "group:g1"}),
		"conv dissolved": Event(ConvRemovedEvent{Type: EventConvRemoved, ConvID: "group:g1", Reason: RemovedDissolved}),
		"read updated":   Event(ReadUpdatedEvent{Type: EventReadUpdated, ConvID: "group:g1", ReadSeq: 3}),
		"pong":           {Cmd: CmdPong},
		// PRD-social M3
		"friendApply ack": AckOK("s1", SocialChangeAck{Status: "pending", Ver: 3, Changed: true, Items: SocialItems{
			Friends: []FriendItem{}, Blocks: []BlockItem{},
			Requests: []RequestItem{{UserID: "bob", Direction: "out", Status: "pending", Source: SourceSearch, ApplyCount: 1, CreatedAt: 1, UpdatedAt: 1, ExpireAt: 2, Ver: 3}},
		}}),
		"respond ack": AckOK("s2", SocialChangeAck{Ver: 4, Changed: true, Items: SocialItems{
			Friends:  []FriendItem{{UserID: "bob", Remark: "B", Source: SourceAdmin, CreatedAt: 1, Ver: 4, Profile: &Profile{Nickname: "bob"}}},
			Requests: []RequestItem{}, Blocks: []BlockItem{{UserID: "x", CreatedAt: 1, Ver: 4, Deleted: true}},
		}}),
		"seen ack":    AckOK("s3", RequestsSeenAck{RequestSeenVer: 5, Ver: 8, Changed: true}),
		"privacy ack": AckOK("s4", VerAck{Ver: 2, Changed: false}),
		"socialSync ack": AckOK("s5", SocialSyncAck{Full: true, UntilVer: 9, LatestVer: 10, HasMore: true, Cursor: "abc.def",
			Friends: []FriendItem{}, Requests: []RequestItem{}, Blocks: []BlockItem{}, Settings: PrivacySettings{Searchable: true}, Mode: "open"}),
		"restart ack": AckOK("s6", SocialSyncAck{Restart: true, Friends: []FriendItem{}, Requests: []RequestItem{}, Blocks: []BlockItem{}, Mode: "friends"}),
		"search hit":  AckOK("s7", UserSearchAck{User: &SearchUser{UserID: "bob", Nickname: "bob", Relation: RelPendingOut}}),
		"search miss": AckOK("s8", UserSearchAck{}),
		"users ack":   AckOK("s9", UsersAck{Users: []UserProfile{{UserID: "bob", Official: true}}}),
		"myGroups":    AckOK("s10", MyGroupsAck{Groups: []MyGroupItem{{GroupID: "g1", Name: "g", MemberCount: 3, JoinedAt: 1, MyRole: "member"}}, NextCursor: "g1"}),
		"tip msg": {Cmd: CmdMsg, Data: Message{MsgID: "m9", ClientMsgID: "tip-1", ConvID: "group:g1", Seq: 1, From: "", Type: MsgTip,
			Body: MsgBody{Tip: &TipBody{Kind: "ownerChanged", Targets: []string{}, Text: "群主已退出群聊"}}, TS: 1}},
		"groupCreate ack":  AckOK("g1", GroupCreateAck{GroupID: "ug-1", Conv: emptyGroup}),
		"groupInvite ack":  AckOK("g2", GroupInviteAck{Added: []string{"bob"}, Failed: []InviteFailed{{UserID: "eve", Code: CodeNotFriend}}}),
		"groupKick ack":    AckOK("g3", GroupKickAck{Removed: []string{"bob"}}),
		"groupInfo ack":    AckOK("g4", GroupInfoAck{GroupID: "g1", Name: "g", Managed: true, MyRole: "member", MemberCount: 2, MaxMembers: 2000, CreatedAt: 1}),
		"groupMembers ack": AckOK("g5", GroupMembersAck{Members: []GroupMemberItem{{UserID: "bob", Role: "owner", JoinedAt: 1}}, NextCursor: "x"}),
		"group changed":    Event(GroupChangedEvent{Type: EventGroupChanged, GroupID: "g1"}),
		"error with user":  AckErr("g6", &Error{Code: CodeNotFriend, Msg: "x", UserID: "dave"}),
		"social changed":   Event(SocialChangedEvent{Type: EventSocialChanged, Ver: 3}),
		"settings changed": Event(SocialSettingsChangedEvent{Type: EventSocialSettingsChanged, Mode: "open", StrangerMsgLimit: 3}),
	}
	for name, f := range frames {
		t.Run(name, func(t *testing.T) {
			b, err := json.Marshal(f)
			if err != nil {
				t.Fatal(err)
			}
			var v any
			if err := json.Unmarshal(b, &v); err != nil {
				t.Fatal(err)
			}
			if err := schema.Validate(v); err != nil {
				t.Errorf("%s does not match schema: %v", b, err)
			}
		})
	}
}

func TestParseConvID(t *testing.T) {
	ok := map[string]Conv{
		"c2c:a:b":  {Type: ConvC2C, A: "a", B: "b"},
		"c2c:u:u":  {Type: ConvC2C, A: "u", B: "u"},
		"c2c:B:a":  {Type: ConvC2C, A: "B", B: "a"}, // ASCII：大写字母小于小写字母
		"group:g1": {Type: ConvGroup, A: "g1"},
		"room:r-1": {Type: ConvRoom, A: "r-1"},
	}
	for s, want := range ok {
		got, err := ParseConvID(s)
		if err != nil || got != want {
			t.Errorf("ParseConvID(%q) = %+v, %v; want %+v", s, got, err, want)
		}
	}
	for _, s := range []string{"", "c2c:b:a", "c2c:a:B", "c2c:a", "group:", "group:a:b", "chan:x", "c2c:a:b c"} {
		if _, err := ParseConvID(s); err == nil || err.Code != CodeInvalidConvID {
			t.Errorf("ParseConvID(%q) = %v, want 4007", s, err)
		}
	}
	if C2CConvID("b", "a") != "c2c:a:b" || C2CConvID("alice", "Bob") != "c2c:Bob:alice" {
		t.Error("C2CConvID must sort user ids by ASCII byte order")
	}
}

func TestSendReqValidate(t *testing.T) {
	base := func() SendReq {
		return SendReq{ConvType: ConvC2C, To: "u2", ClientMsgID: "12345678", Type: MsgText, Body: MsgBody{Text: "hi"}}
	}
	r := base()
	if err := r.Validate(); err != nil {
		t.Fatalf("valid request failed: %v", err)
	}
	cases := map[string]struct {
		mutate func(*SendReq)
		code   int
	}{
		"bad convType":     {func(r *SendReq) { r.ConvType = "x" }, CodeBadRequest},
		"bad to":           {func(r *SendReq) { r.To = "a b" }, CodeInvalidID},
		"short clientId":   {func(r *SendReq) { r.ClientMsgID = "1234567" }, CodeBadRequest},
		"long clientId":    {func(r *SendReq) { r.ClientMsgID = strings.Repeat("x", 65) }, CodeBadRequest},
		"empty text":       {func(r *SendReq) { r.Body.Text = "" }, CodeTextEmpty},
		"whitespace text":  {func(r *SendReq) { r.Body.Text = " \n\t\u3000\u00a0\r" }, CodeTextEmpty},
		"text too long":    {func(r *SendReq) { r.Body.Text = strings.Repeat("字", MaxTextLen+1) }, CodeTextTooLong},
		"custom no data":   {func(r *SendReq) { r.Type = MsgCustom }, CodeBadRequest},
		"custom null":      {func(r *SendReq) { r.Type = MsgCustom; r.Body.Data = json.RawMessage("null") }, CodeBadRequest},
		"unknown msg type": {func(r *SendReq) { r.Type = "image" }, CodeBadRequest},
	}
	for name, tc := range cases {
		r := base()
		tc.mutate(&r)
		if err := r.Validate(); err == nil || err.Code != tc.code {
			t.Errorf("%s: got %v, want %d", name, err, tc.code)
		}
	}
}

// TC-MSG-01-*：去首尾空白后按码点计数（AC-MSG-01-4、AC-MSG-01-5，PRD C-26、C-27）。
func TestTextTrimAndCount(t *testing.T) {
	for _, tc := range []struct {
		in        string
		wantText  string
		wantError int
	}{
		{"  你好  ", "你好", 0},
		{"\u3000\n你好\t\r", "你好", 0},
		{strings.Repeat("a", MaxTextLen), strings.Repeat("a", MaxTextLen), 0},
		{"  " + strings.Repeat("😀", MaxTextLen) + "  ", strings.Repeat("😀", MaxTextLen), 0}, // emoji 每个算 1 个码点
		{strings.Repeat("a", MaxTextLen+1), "", CodeTextTooLong},
		{"\u200b", "\u200b", 0}, // 零宽空格不算空白
		{"\ufeff", "\ufeff", 0}, // BOM 不算空白
		{strings.Repeat("👨\u200d👩\u200d👧", 1000), strings.Repeat("👨\u200d👩\u200d👧", 1000), 0}, // ZWJ 一家人 = 5 码点，1000 个 = 5000
		{strings.Repeat("👨\u200d👩\u200d👧", 1000) + "a", "", CodeTextTooLong},
	} {
		r := SendReq{ConvType: ConvC2C, To: "u2", ClientMsgID: "12345678", Type: MsgText, Body: MsgBody{Text: tc.in}}
		err := r.Validate()
		switch {
		case tc.wantError != 0 && (err == nil || err.Code != tc.wantError):
			t.Errorf("%q: err = %v, want %d", tc.in[:min(len(tc.in), 20)], err, tc.wantError)
		case tc.wantError == 0 && err != nil:
			t.Errorf("%q: unexpected err %v", tc.in[:min(len(tc.in), 20)], err)
		case tc.wantError == 0 && r.Body.Text != tc.wantText:
			t.Errorf("trimmed to %q, want %q", r.Body.Text, tc.wantText)
		}
	}
}

func TestSyncReqValidate(t *testing.T) {
	if limit, err := (&SyncReq{ConvID: "group:g1", AfterSeq: ptr[int64](0)}).Validate(); err != nil || limit != defaultSyncLimit {
		t.Fatalf("default limit: %d %v", limit, err)
	}
	if limit, err := (&SyncReq{ConvID: "group:g1", BeforeSeq: ptr[int64](101), Limit: ptr(20)}).Validate(); err != nil || limit != 20 {
		t.Fatalf("beforeSeq: %d %v", limit, err)
	}
	for name, r := range map[string]SyncReq{
		"neither":        {ConvID: "group:g1"},
		"both":           {ConvID: "group:g1", AfterSeq: ptr[int64](0), BeforeSeq: ptr[int64](5)},
		"limit 0":        {ConvID: "group:g1", AfterSeq: ptr[int64](0), Limit: ptr(0)},
		"limit negative": {ConvID: "group:g1", AfterSeq: ptr[int64](0), Limit: ptr(-1)},
		"limit 201":      {ConvID: "group:g1", AfterSeq: ptr[int64](0), Limit: ptr(201)},
		"negative after": {ConvID: "group:g1", AfterSeq: ptr[int64](-1)},
		"before zero":    {ConvID: "group:g1", BeforeSeq: ptr[int64](0)},
	} {
		if _, err := r.Validate(); err == nil || err.Code != CodeBadRequest {
			t.Errorf("%s: got %v, want 4000", name, err)
		}
	}
	if _, err := (&SyncReq{ConvID: "x", AfterSeq: ptr[int64](0)}).Validate(); err == nil || err.Code != CodeInvalidConvID {
		t.Errorf("bad convId: %v", err)
	}
}

func TestSummary(t *testing.T) {
	long := strings.Repeat("字", 150)
	if got := Summary(long); TextLen(got) != 100 {
		t.Errorf("summary length %d, want 100", TextLen(got))
	}
	if Summary("short") != "short" {
		t.Error("short text must be unchanged")
	}
}
