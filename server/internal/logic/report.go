package logic

import (
	"context"

	"tianting/server/internal/delivery"
	"tianting/server/internal/protocol"
)

// submitReport 是开关打开之后的举报（对象校验、限额、幂等、证据）：随 BE-495。
func (s *Service) submitReport(_ context.Context, _ delivery.Session, _ protocol.ReportReq) (any, *protocol.Error) {
	return nil, protocol.Errorf(protocol.CodeUnknownCmd, "report is not available yet")
}
