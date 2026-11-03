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
	CodeGroupMemberLimitExceeded = 4130
	CodeBatchLimitExceeded       = 4131
	CodeFriendLimitExceeded      = 4132
	CodePeerFriendLimitExceeded  = 4133
	CodeBlockLimitExceeded       = 4134
	CodeJoinedGroupLimitExceeded = 4135
	CodeGroupAdminLimitExceeded  = 4136
	CodeStrangerMsgLimit         = 4137
	CodeRateLimited              = 4290
	CodeQuotaExceeded            = 4292
	CodeInternal                 = 5000
	CodeServiceUnavailable       = 5030
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
	CodeInvalidConvID:            {"INVALID_CONV_ID", 0},
	CodeClientMsgIDConflict:      {"CLIENT_MSG_ID_CONFLICT", 0},
	CodeInvalidPlatform:          {"INVALID_PLATFORM", 0},
	CodeTokenInvalid:             {"TOKEN_INVALID", 0},
	CodeTokenExpired:             {"TOKEN_EXPIRED", 0},
	CodeTokenRevoked:             {"TOKEN_REVOKED", 0},
	CodeAdminKeyInvalid:          {"ADMIN_KEY_INVALID", 401},
	CodeAdminSignatureInvalid:    {"ADMIN_SIGNATURE_INVALID", 401},
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
	CodeGroupMemberLimitExceeded: {"GROUP_MEMBER_LIMIT_EXCEEDED", 400},
	CodeBatchLimitExceeded:       {"BATCH_LIMIT_EXCEEDED", 400},
	CodeFriendLimitExceeded:      {"FRIEND_LIMIT_EXCEEDED", 400},
	CodePeerFriendLimitExceeded:  {"PEER_FRIEND_LIMIT_EXCEEDED", 0},
	CodeBlockLimitExceeded:       {"BLOCK_LIMIT_EXCEEDED", 0},
	CodeJoinedGroupLimitExceeded: {"JOINED_GROUP_LIMIT_EXCEEDED", 0},
	CodeGroupAdminLimitExceeded:  {"GROUP_ADMIN_LIMIT_EXCEEDED", 0},
	CodeStrangerMsgLimit:         {"STRANGER_MSG_LIMIT", 0},
	CodeRateLimited:              {"RATE_LIMITED", 429},
	CodeQuotaExceeded:            {"QUOTA_EXCEEDED", 429},
	CodeInternal:                 {"INTERNAL", 500},
	CodeServiceUnavailable:       {"SERVICE_UNAVAILABLE", 503},
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
	KickReplaced     = "replaced"
	KickTooManyConns = "tooManyConnections"
	KickShutdown     = "serverShutdown"
)

// KickCloseCode 返回被踢原因对应的关闭码。
var KickCloseCode = map[string]int{
	KickTokenInvalid: CloseTokenInvalid,
	KickTokenExpired: CloseTokenExpired,
	KickBanned:       CloseTokenRevoked,
	KickTooManyConns: CloseTooManyConns,
	KickShutdown:     CloseGoingAway,
}
