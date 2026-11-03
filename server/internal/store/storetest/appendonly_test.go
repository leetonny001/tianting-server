package storetest

import (
	"reflect"
	"strings"
	"testing"

	"tianting/server/internal/store"
)

// AC-KEY-04-4：存储接口对审计只有追加和查询方法，没有任何更新 / 删除。
func TestAdminAuditIsAppendOnly(t *testing.T) {
	typ := reflect.TypeOf((*store.Store)(nil)).Elem()
	var methods []string
	for i := 0; i < typ.NumMethod(); i++ {
		if name := typ.Method(i).Name; strings.Contains(name, "AdminAudit") {
			methods = append(methods, name)
		}
	}
	if strings.Join(methods, ",") != "AppendAdminAudit,QueryAdminAudit" {
		t.Errorf("Store admin audit methods = %v, want only AppendAdminAudit and QueryAdminAudit", methods)
	}
}
