package admin

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// AC-ADM-30-7：元数据响应的类型里没有正文字段；新增字段不能把正文带出去。
func TestMessageMetaHasNoBody(t *testing.T) {
	typ := reflect.TypeOf(messageMetaJSON{})
	for i := range typ.NumField() {
		f := typ.Field(i)
		if strings.EqualFold(f.Name, "body") || strings.EqualFold(f.Tag.Get("json"), "body") {
			t.Errorf("field %s carries the message body", f.Name)
		}
	}
	b, _ := json.Marshal(messageMetaJSON{Seq: 1})
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	if _, ok := m["body"]; ok {
		t.Errorf("json has body: %s", b)
	}
}

func TestSummaryWithCount(t *testing.T) {
	if got := summaryWithCount(`{"fields":["userId"]}`, 2); got != `{"count":2,"fields":["userId"]}` {
		t.Errorf("got %s", got)
	}
	if got := summaryWithCount(`not json`, 2); got != `not json` {
		t.Errorf("got %s", got)
	}
}
