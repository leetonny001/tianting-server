package accountmodel

// 服务端判定的参考模型（PRD-chat F-ACC-03 规则 4，BE-402）：只有代号和时间，没有存储和签名。
// protocol/vectors/account-session.json 的 serverCases 由它生成；account_vectors_test.go 用同一份文件跑真实服务端，两边独立实现，互相校验。

// 判定结果。
const (
	ResultIssued     = "issued"         // refresh：签发新的一对，代号在 Gen
	ResultPromoted   = "issuedPromoted" // refresh：用 pending 刷新：先转正，再签发
	ResultSuperseded = "superseded"     // refresh：窗口内的旧值，不吊销，不发 token
	ResultReplay     = "replayDetected" // refresh：吊销会话
	ResultRevoked    = "alreadyRevoked" // refresh：吊销之后再来，reason 按吊销原因
	ResultLimited    = "rateLimited"    // refresh：429 / 4290，不吊销
	AccessOK         = "ok"             // access：等于 current
	AccessPromoted   = "promoted"       // access：属于 pending，转正
	AccessRejected   = "tokenExpired"   // access：其他：WS kicked/tokenExpired，HTTP 4011 tokenSuperseded
	AccessRevoked    = "sessionRevoked" // access：会话已吊销
)

// ServerStep 是服务端判定的一步（规则 4）：op 是 refresh、access 或 advance；gen 是请求用的 token 的代号。
type ServerStep struct {
	Op       string  `json:"op"`
	Gen      int64   `json:"gen,omitempty"`
	Ms       int64   `json:"ms,omitempty"`
	Repeat   int     `json:"repeat,omitempty"` // refresh：先重复这么多次，结果不断言（用来用满次数或挤掉 pending）
	Result   string  `json:"result,omitempty"`
	NewGen   int64   `json:"newGen,omitempty"`
	Current  int64   `json:"current"`
	Revoked  bool    `json:"revoked"`
	MinGen   int64   `json:"overlapMinGen"`
	InWindow bool    `json:"inWindow"`
	Pendings []int64 `json:"pendings"`
}

// ServerCase 是一条判定用例，从登录后的状态（current=1）开始。
type ServerCase struct {
	Name  string       `json:"name"`
	Rule  string       `json:"rule"`
	Steps []ServerStep `json:"steps"`
}

// ServerSession 是一条设备会话的代号状态。
type ServerSession struct {
	Current, Next, OverlapUntil, OverlapMinGen int64
	Pendings                                   []int64
	Revoked                                    bool
	Issued                                     int
	Now                                        int64
	MaxPending, IssueRate                      int
	OverlapMs                                  int64
}

// NewServerSession 是登录之后的状态：current=1，nextGen=2。
func NewServerSession() *ServerSession {
	return &ServerSession{Current: 1, Next: 2, MaxPending: 4, IssueRate: 20, OverlapMs: 2 * 60 * 1000}
}

func (s *ServerSession) isPending(gen int64) bool {
	for _, p := range s.Pendings {
		if p == gen {
			return true
		}
	}
	return false
}

func (s *ServerSession) promote(gen int64) {
	s.OverlapUntil = s.Now + s.OverlapMs
	s.OverlapMinGen = s.Current
	s.Current = gen
	s.Pendings = nil
}

// Refresh 按规则 4 的顺序判定一个 mac 正确的 refresh token。返回结果和新代号。
func (s *ServerSession) Refresh(gen int64) (string, int64) {
	if s.Revoked {
		return ResultRevoked, 0
	}
	pending := s.isPending(gen)
	if gen != s.Current && !pending {
		if gen >= s.OverlapMinGen && s.Now < s.OverlapUntil {
			return ResultSuperseded, 0
		}
		s.Revoked = true
		return ResultReplay, 0
	}
	// 第 ⑥ 步：签发频率只在将要签发的分支检查（RS6-05）。这里的窗口不随时间清零：用例在一个小时内。
	if s.Issued >= s.IssueRate {
		return ResultLimited, 0
	}
	s.Issued++
	result := ResultIssued
	if pending {
		s.promote(gen)
		result = ResultPromoted
	}
	next := s.Next
	s.Next++
	s.Pendings = append(s.Pendings, next)
	if len(s.Pendings) > s.MaxPending {
		s.Pendings = s.Pendings[len(s.Pendings)-s.MaxPending:]
	}
	return result, next
}

// Access 是 access token 的代号判定表（auth 和 /v1/*）。
func (s *ServerSession) Access(gen int64) string {
	switch {
	case s.Revoked:
		return AccessRevoked
	case gen == s.Current:
		return AccessOK
	case s.isPending(gen):
		s.promote(gen)
		return AccessPromoted
	}
	return AccessRejected
}
