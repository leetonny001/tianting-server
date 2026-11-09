package files

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
)

// TC-FILE-01-07、TC-FILE-05-02 的清洗部分（F-FILE-01 规则 7）。
func TestSanitizeName(t *testing.T) {
	for in, want := range map[string]string{
		"../../etc/pass\u202Ewd.": "etcpasswd",
		"a\nb":                    "ab",
		"\x00\x01\x1f":            "file",
		"photo\u202Egpj.html":     "photogpj.html",
		"a.exe.":                  "a.exe",
		"a.exe::$DATA":            "a.exe",
		"a.exe . . ":              "a.exe",
		"...hidden":               "hidden",
		"e\u0301.txt":             "é.txt",  // NFC
		"a\u200bb\ufeff.txt":      "ab.txt", // 零宽字符、BOM（Default_Ignorable）
		"中文 名字.pdf":               "中文 名字.pdf",
		`a\b/c.txt`:               "abc.txt",
		"":                        "file",
	} {
		if got := SanitizeName(in); got != want {
			t.Errorf("SanitizeName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBlocklist(t *testing.T) {
	b, err := LoadBlocklist("")
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"a.exe", "A.EXE", "x.pdf.exe", "a.svgz", "b.mhtml", "c.desktop"} {
		if !b.Blocked(n) {
			t.Errorf("%s not blocked", n)
		}
	}
	for _, n := range []string{"a.pdf", "a.doc", "noext", "a.exe.txt"} {
		if b.Blocked(n) {
			t.Errorf("%s blocked", n)
		}
	}
}

// TC-FILE-01-25（AC-FILE-01-25）的头部生成：filename* 百分号编码，ASCII 回退名不含 `"` `;` `%`。
func TestDisposition(t *testing.T) {
	got := Disposition("attachment", `evil.exe"; x=".txt`)
	want := `attachment; filename="evil.exe__ x=_.txt"; filename*=UTF-8''evil.exe%22%3B%20x%3D%22.txt`
	if got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
	cn := Disposition("attachment", "报告%.pdf")
	fallback := cn[strings.Index(cn, `filename="`)+10 : strings.Index(cn, `";`)]
	if strings.ContainsAny(fallback, "\";%") || !strings.Contains(cn, "filename*=UTF-8''%E6%8A%A5%E5%91%8A%25.pdf") {
		t.Errorf("chinese name: %s", cn)
	}
	if Disposition("inline", "x") != "inline" {
		t.Error("inline")
	}
}

func TestSniff(t *testing.T) {
	png := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\x0dIHDR")
	png = binary.BigEndian.AppendUint32(png, 640)
	png = binary.BigEndian.AppendUint32(png, 480)
	if r := sniff(png); r.Mime != "image/png" || r.Width != 640 || r.Height != 480 {
		t.Errorf("png = %+v", r)
	}
	jpg := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0, 4, 0, 0, 0xFF, 0xC2, 0, 0x11, 8, 0x01, 0xE0, 0x02, 0x80}
	if r := sniff(jpg); r.Mime != "image/jpeg" || r.Width != 640 || r.Height != 480 {
		t.Errorf("progressive jpeg = %+v", r)
	}
	if r := sniff([]byte{0xFF, 0xD8, 0xFF, 0xE0, 0, 4, 0, 0, 0xFF, 0xDA, 0, 2}); r.Width != 0 {
		t.Errorf("jpeg without SOF = %+v", r)
	}
	gif := append([]byte("GIF89a"), 0x10, 0x00, 0x20, 0x00, 0x00, 0, 0)
	frame := []byte{0x2C, 0, 0, 0, 0, 1, 0, 1, 0, 0, 0x02, 0x01, 0x00, 0x00}
	gif = append(gif, bytes.Repeat(frame, 5)...)
	if r := sniff(gif); r.Mime != "image/gif" || r.Width != 16 || r.Height != 32 || r.Frames != 5 {
		t.Errorf("gif = %+v", r)
	}
	webp := []byte("RIFF\x00\x00\x00\x00WEBPVP8X\x0a\x00\x00\x00\x00\x00\x00\x00\x7f\x02\x00\xdf\x01\x00")
	if r := sniff(webp); r.Mime != "image/webp" || r.Width != 640 || r.Height != 480 {
		t.Errorf("webp = %+v", r)
	}
	for name, b := range map[string][]byte{
		"pe": []byte("MZ\x90\x00"), "elf": []byte("\x7fELF\x02"), "html": []byte("<!DOCTYPE html><html>"), "svg": []byte(`<?xml version="1.0"?><svg xmlns="x">`),
		"script": []byte("#!/bin/sh\nrm -rf /"),
	} {
		if r := sniff(b); !r.Forbidden {
			t.Errorf("%s not forbidden: %+v", name, r)
		}
	}
	if r := sniff([]byte{0x01, 0x02, 0x03, 0x04, 0xF0, 0xF1}); r.Mime != octetStream || r.Forbidden {
		t.Errorf("unknown = %+v", r)
	}
	if r := sniff([]byte("%PDF-1.7\n")); r.Mime != "application/pdf" {
		t.Errorf("pdf = %+v", r)
	}
}

// F-FILE-01 规则 3b、F-FILE-02 规则 1 的判定。
func TestJudge(t *testing.T) {
	s := &Service{}
	img := func(w, h int) sniffResult { return sniffResult{Mime: "image/png", Width: w, Height: h} }
	for _, tc := range []struct {
		kind, variant, declared string
		sn                      sniffResult
		ok                      bool
	}{
		{"image", "original", "image/png", img(4000, 3000), true},
		{"image", "original", "image/jpeg", img(4000, 3000), false}, // 和声明矛盾
		{"image", "original", "image/png", img(20000, 20000), false},
		{"image", "original", "image/png", img(10000, 4001), false}, // 像素超过 4000 万
		{"image", "thumb", "image/png", img(512, 300), true},
		{"image", "thumb", "image/png", img(6000, 6000), false},
		{"avatar", "original", "image/png", img(1024, 1024), true},
		{"avatar", "original", "image/png", img(6000, 6000), false},
		{"avatar", "original", "image/gif", sniffResult{Mime: "image/gif", Width: 10, Height: 10}, false},
		{"image", "original", "image/png", img(0, 0), false},
		{"image", "original", "image/gif", sniffResult{Mime: "image/gif", Width: 10, Height: 10, Frames: gifMaxFrames + 1}, false},
		{"file", "original", "application/pdf", sniffResult{Mime: octetStream}, true},
		{"file", "original", "application/pdf", sniffResult{Mime: octetStream, Forbidden: true}, false},
	} {
		if _, _, _, ok := s.judge(tc.kind, tc.variant, tc.declared, tc.sn); ok != tc.ok {
			t.Errorf("%s/%s %+v: ok=%v, want %v", tc.kind, tc.variant, tc.sn, ok, tc.ok)
		}
	}
}
