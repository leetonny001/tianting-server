package gateway

import (
	"reflect"
	"slices"
	"strings"
	"testing"
)

// 完整 IP 不进连接元数据（PRD-admin F-ADM-14 规则 2，BE-206）：conn 上和 IP 有关的字段只有脱敏后的那一个。
func TestConnKeepsOnlyMaskedIP(t *testing.T) {
	var fields []string
	for _, f := range reflect.VisibleFields(reflect.TypeOf(conn{})) {
		if strings.Contains(strings.ToLower(f.Name), "ip") || strings.Contains(strings.ToLower(f.Name), "addr") || strings.Contains(strings.ToLower(f.Name), "remote") {
			fields = append(fields, f.Name)
		}
	}
	if !slices.Equal(fields, []string{"maskedIP"}) {
		t.Errorf("IP related fields on conn = %v", fields)
	}
}
