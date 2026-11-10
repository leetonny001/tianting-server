package protocol

// 错误码、关闭码、被踢原因。唯一真相源是 protocol/errors.json，
// errors_test.go 保证这里与它完全一致；新增错误码先改 errors.json。

// 错误码。
const (
	CodeBadRequest               = 4000
	CodeUnknownCmd               = 4001
	CodeInvalidFrame             = 4002
	CodeMissingRequestID         = 4003
	CodeInvalidID                = 4004
	CodeTextEmpty                = 4005
	CodeTextTooLong              = 4006
	CodeInvalidConvID            = 4007
	CodeClientMsgIDConflict      = 4008
	CodeInvalidPlatform          = 4009
	CodeTokenInvalid             = 4010
	CodeTokenExpired             = 4011
	CodeTokenRevoked             = 4012
	CodeAdminKeyInvalid          = 4013
	CodeAdminSignatureInvalid    = 4017
	CodeAccountCredentials       = 4018 // 用户名、密码或恢复码错误，不区分（PRD-chat F-ACC-02 规则 2）
	CodeRefreshTokenInvalid      = 4019 // PRD-chat F-ACC-03
	CodeFileNotFound             = 4048 // PRD-chat F-FILE-04：不存在、无权访问、已过期或已撤回，不区分
	CodeFileNotReady             = 4096 // 文件未完成上传，或已绑定其他消息（PRD-chat 第 11.5 节）
	CodeRecallWindowExpired      = 4097 // 超过撤回时限，details.windowSeconds（C4，PRD-chat F-RCL-01、02）
	CodeUserBanned               = 4034
	CodeMessageNotFound          = 4044
	CodeMsgReadGrantRequired     = 4039
	CodeForbidden                = 4030
	CodeMuted                    = 4031
	CodeNotConvParticipant       = 4032
	CodeNotGroupMember           = 4033
	CodeNotFriend                = 4035
	CodeRejectedByPeer           = 4036
	CodeNotFound                 = 4040
	CodeNotInRoom                = 4041
	CodeUserNotFound             = 4042
	CodeGroupNotFound            = 4043
	CodeFriendRequestNotFound    = 4045
	CodeTargetNotGroupMember     = 4046
	CodeGroupExists              = 4090
	CodeFriendRequestHandled     = 4091
	CodePushIDConflict           = 4094
	CodeUsernameTaken            = 4095
	CodeGroupMemberLimitExceeded = 4130
	CodeBatchLimitExceeded       = 4131
	CodeFriendLimitExceeded      = 4132
	CodePeerFriendLimitExceeded  = 4133
	CodeBlockLimitExceeded       = 4134
	CodeJoinedGroupLimitExceeded = 4135
	CodeGroupAdminLimitExceeded  = 4136
	CodeStrangerMsgLimit         = 4137
	CodeFileTooLarge             = 4139 // details.maxBytes
	CodeFileTypeNotAllowed       = 4150
	CodePinLimitExceeded         = 4220 // 置顶数量已满，不带 details，文案固定「最多置顶 20 个会话」（C5，F-CVS-01 规则 2）
	CodeRateLimited              = 4290
	CodeQuotaExceeded            = 4292
	CodeMentionAllLimited        = 4293 // @ 所有人次数用完，details.retryAfterSeconds，不自动重试（C4，F-MEN-02 规则 2）
	CodeAccountLocked            = 4294 // details.retryAfterSeconds，不自动重试
	CodeInternal                 = 5000
	CodeServiceUnavailable       = 5030
	CodeObjectStoreUnavailable   = 5032
)

type codeInfo struct {
	Name string
	HTTP int // 0 表示只出现在 WebSocket
}

var codes = map[int]codeInfo{
	CodeBadRequest:               {"BAD_REQUEST", 400},
	CodeUnknownCmd:               {"UNKNOWN_CMD", 0},
	CodeInvalidFrame:             {"INVALID_FRAME", 0},
	CodeMissingRequestID:         {"MISSING_REQUEST_ID", 0},
	CodeInvalidID:                {"INVALID_ID", 400},
	CodeTextEmpty:                {"TEXT_EMPTY", 0},
	CodeTextTooLong:              {"TEXT_TOO_LONG", 0},
	CodeInvalidConvID:            {"INVALID_CONV_ID", 400},
	CodeClientMsgIDConflict:      {"CLIENT_MSG_ID_CONFLICT", 0},
	CodeInvalidPlatform:          {"INVALID_PLATFORM", 0},
	CodeTokenInvalid:             {"TOKEN_INVALID", 401},
	CodeTokenExpired:             {"TOKEN_EXPIRED", 401},
	CodeTokenRevoked:             {"TOKEN_REVOKED", 401},
	CodeAdminKeyInvalid:          {"ADMIN_KEY_INVALID", 401},
	CodeAdminSignatureInvalid:    {"ADMIN_SIGNATURE_INVALID", 401},
	CodeAccountCredentials:       {"ACCOUNT_CREDENTIALS_INVALID", 401},
	CodeRefreshTokenInvalid:      {"REFRESH_TOKEN_INVALID", 401},
	CodeFileNotFound:             {"FILE_NOT_FOUND", 404},
	CodeFileNotReady:             {"FILE_NOT_READY", 409},
	CodeRecallWindowExpired:      {"RECALL_WINDOW_EXPIRED", 0},
	CodeUserBanned:               {"USER_BANNED", 403},
	CodeMessageNotFound:          {"MESSAGE_NOT_FOUND", 404},
	CodeMsgReadGrantRequired:     {"MSG_READ_GRANT_REQUIRED", 403},
	CodeForbidden:                {"FORBIDDEN", 403},
	CodeMuted:                    {"MUTED", 0},
	CodeNotConvParticipant:       {"NOT_CONV_PARTICIPANT", 0},
	CodeNotGroupMember:           {"NOT_GROUP_MEMBER", 0},
	CodeNotFriend:                {"NOT_FRIEND", 0},
	CodeRejectedByPeer:           {"REJECTED_BY_PEER", 0},
	CodeNotFound:                 {"NOT_FOUND", 404},
	CodeNotInRoom:                {"NOT_IN_ROOM", 0},
	CodeUserNotFound:             {"USER_NOT_FOUND", 404},
	CodeGroupNotFound:            {"GROUP_NOT_FOUND", 404},
	CodeFriendRequestNotFound:    {"FRIEND_REQUEST_NOT_FOUND", 0},
	CodeTargetNotGroupMember:     {"TARGET_NOT_GROUP_MEMBER", 404},
	CodeGroupExists:              {"GROUP_EXISTS", 409},
	CodeFriendRequestHandled:     {"FRIEND_REQUEST_HANDLED", 0},
	CodePushIDConflict:           {"PUSH_ID_CONFLICT", 409},
	CodeUsernameTaken:            {"USERNAME_TAKEN", 409},
	CodeGroupMemberLimitExceeded: {"GROUP_MEMBER_LIMIT_EXCEEDED", 400},
	CodeBatchLimitExceeded:       {"BATCH_LIMIT_EXCEEDED", 400},
	CodeFriendLimitExceeded:      {"FRIEND_LIMIT_EXCEEDED", 400},
	CodePeerFriendLimitExceeded:  {"PEER_FRIEND_LIMIT_EXCEEDED", 0},
	CodeBlockLimitExceeded:       {"BLOCK_LIMIT_EXCEEDED", 0},
	CodeJoinedGroupLimitExceeded: {"JOINED_GROUP_LIMIT_EXCEEDED", 0},
	CodeGroupAdminLimitExceeded:  {"GROUP_ADMIN_LIMIT_EXCEEDED", 0},
	CodeStrangerMsgLimit:         {"STRANGER_MSG_LIMIT", 0},
	CodeFileTooLarge:             {"FILE_TOO_LARGE", 413},
	CodeFileTypeNotAllowed:       {"FILE_TYPE_NOT_ALLOWED", 415},
	CodePinLimitExceeded:         {"PIN_LIMIT_EXCEEDED", 0},
	CodeRateLimited:              {"RATE_LIMITED", 429},
	CodeQuotaExceeded:            {"QUOTA_EXCEEDED", 429},
	CodeMentionAllLimited:        {"MENTION_ALL_LIMITED", 0},
	CodeAccountLocked:            {"ACCOUNT_LOGIN_LOCKED", 429},
	CodeInternal:                 {"INTERNAL", 500},
	CodeServiceUnavailable:       {"SERVICE_UNAVAILABLE", 503},
	CodeObjectStoreUnavailable:   {"OBJECT_STORE_UNAVAILABLE", 503},
}

// HTTPStatus 返回错误码在管理接口上对应的 HTTP 状态码；未定义时按前三位推断。
func HTTPStatus(code int) int {
	if info, ok := codes[code]; ok && info.HTTP != 0 {
		return info.HTTP
	}
	if s := code / 10; s >= 400 && s < 600 {
		return s
	}
	return 500
}

// WebSocket 关闭码。
const (
	CloseNormal          = 1000
	CloseGoingAway       = 1001
	CloseMessageTooBig   = 1009
	CloseInvalidPlatform = 4009
	CloseTokenInvalid    = 4010
	CloseTokenExpired    = 4011
	CloseTokenRevoked    = 4012
	CloseTooManyConns    = 4429
	CloseInternalError   = 1011
	CloseAuthRequired    = 4401
	CloseSlowConsumer    = 4400
	CloseIdleTimeout     = 4408
)

var closeCodes = map[int]string{
	CloseNormal:          "NORMAL",
	CloseGoingAway:       "GOING_AWAY",
	CloseMessageTooBig:   "MESSAGE_TOO_BIG",
	CloseInvalidPlatform: "INVALID_PLATFORM",
	CloseTokenInvalid:    "TOKEN_INVALID",
	CloseTokenExpired:    "TOKEN_EXPIRED",
	CloseTokenRevoked:    "TOKEN_REVOKED",
	CloseTooManyConns:    "TOO_MANY_CONNECTIONS",
	CloseInternalError:   "INTERNAL_ERROR",
	CloseAuthRequired:    "AUTH_REQUIRED",
	CloseSlowConsumer:    "SLOW_CONSUMER",
	CloseIdleTimeout:     "IDLE_TIMEOUT",
}

// CloseCodeName 返回关闭码的名称（如 4408 → "IDLE_TIMEOUT"）；不认识的返回空字符串。
func CloseCodeName(code int) string { return closeCodes[code] }

// 被踢原因（event/kicked 的 reason）。
const (
	KickTokenInvalid = "tokenInvalid"
	KickTokenExpired = "tokenExpired"
	KickBanned       = "banned"
	KickSuspended    = "suspended" // 封禁（PRD-admin F-ADM-13，第二批），和 banned 同一关闭码
	KickReplaced     = "replaced"
	KickTooManyConns = "tooManyConnections"
	KickShutdown     = "serverShutdown"
	// 账号的设备会话被吊销（PRD-chat F-ACC-03 规则 2），都可以带 recoverBefore。
	KickPasswordChanged = "passwordChanged"
	KickSessionRevoked  = "sessionRevoked"
	KickAccountDeleted  = "accountDeleted"
)

// KickCloseCode 返回被踢原因对应的关闭码。
var KickCloseCode = map[string]int{
	KickTokenInvalid:    CloseTokenInvalid,
	KickTokenExpired:    CloseTokenExpired,
	KickBanned:          CloseTokenRevoked,
	KickSuspended:       CloseTokenRevoked,
	KickTooManyConns:    CloseTooManyConns,
	KickShutdown:        CloseGoingAway,
	KickPasswordChanged: CloseTokenRevoked,
	KickSessionRevoked:  CloseTokenRevoked,
	KickAccountDeleted:  CloseTokenRevoked,
}

// SessionKickReason 把设备会话的吊销原因映射成踢线原因（PRD-chat F-ACC-03 规则 2）。
func SessionKickReason(revokeReason string) string {
	switch revokeReason {
	case ReasonCredentialsChanged:
		return KickPasswordChanged
	case ReasonAccountDeleted:
		return KickAccountDeleted
	}
	return KickSessionRevoked
}

// 错误的 reason（PRD-chat 第 11.5 节，C1 部分）。errors.json 的 reasons 是真相源，errors_test.go 保证一致。
const (
	ReasonPasswordWeak           = "passwordWeak"
	ReasonRecoveryCodeStale      = "recoveryCodeStale"
	ReasonTokenSuperseded        = "tokenSuperseded"
	ReasonRefreshInvalid         = "invalid"
	ReasonRefreshExpired         = "expired"
	ReasonRefreshRevoked         = "revoked"
	ReasonRefreshSuperseded      = "superseded"
	ReasonReplayDetected         = "replayDetected"
	ReasonCredentialsChanged     = "credentialsChanged"
	ReasonLoggedOutOthers        = "loggedOutOthers"
	ReasonAccountDeleted         = "accountDeleted"
	ReasonRegisterClosed         = "registerClosed"
	ReasonInviteInvalid          = "inviteInvalid"
	ReasonAccountSessionRequired = "accountSessionRequired"
	ReasonAccountExists          = "accountExists"
	ReasonAccountDeletionPending = "accountDeletionPending"
	ReasonFeatureDisabled        = "featureDisabled"
	ReasonMentionAllForbidden    = "mentionAllForbidden" // 4030：普通成员 @ 所有人（C4，F-MEN-02 规则 1）
	ReasonQuoteUnavailable       = "quoteUnavailable"    // 4040：引用的消息不可用，几种原因不区分（C4，F-QUO-01 规则 2）
	ReasonRecoverLimited         = "recoverLimited"
	// C2 文件（PRD-chat F-FILE-01、02）。
	ReasonFileKindMismatch   = "fileKindMismatch"
	ReasonUploadMismatch     = "uploadMismatch"
	ReasonFileNameTooLong    = "fileNameTooLong"
	ReasonUploadPendingLimit = "uploadPendingLimit"
	ReasonCompleteInProgress = "completeInProgress"
)

// reasons 按错误码列出 reason，只给一致性测试用。
var reasons = map[int][]string{
	CodeBadRequest:             {ReasonPasswordWeak, ReasonRecoveryCodeStale, ReasonFileKindMismatch, ReasonUploadMismatch, ReasonFileNameTooLong},
	CodeTokenExpired:           {ReasonTokenSuperseded},
	CodeRefreshTokenInvalid:    {ReasonRefreshInvalid, ReasonRefreshExpired, ReasonRefreshRevoked, ReasonRefreshSuperseded, ReasonReplayDetected, ReasonCredentialsChanged, ReasonLoggedOutOthers, ReasonAccountDeleted},
	CodeForbidden:              {ReasonRegisterClosed, ReasonInviteInvalid, ReasonAccountSessionRequired, ReasonAccountExists, ReasonAccountDeletionPending, ReasonFeatureDisabled, ReasonMentionAllForbidden},
	CodeNotFound:               {ReasonQuoteUnavailable},
	CodeRateLimited:            {ReasonRecoverLimited, ReasonUploadPendingLimit},
	CodeObjectStoreUnavailable: {ReasonCompleteInProgress},
}

// WithReason 构造带 reason 的协议错误。
func WithReason(code int, reason, msg string) *Error {
	return &Error{Code: code, Msg: msg, Reason: reason}
}
