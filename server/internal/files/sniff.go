package files

import (
	"bytes"
	"encoding/binary"
	"net/http"
	"strings"
)

// sniffResult 是前 64 KB 的嗅探结果（F-FILE-01 规则 3 ②）。只读头部，不解码像素（R-CS-12）。
type sniffResult struct {
	Mime          string // 真实类型；认不出是 application/octet-stream
	Forbidden     bool   // 禁止的真实类型：可执行文件、脚本、HTML、SVG（F-FILE-05 规则 1）
	Width, Height int    // 图片头部的真实宽高；解析不到是 0
	Frames        int    // GIF 在这段数据里能看到的帧数
}

const octetStream = "application/octet-stream"

// 图片白名单（F-FILE-02 规则 2）：SVG、BMP、TIFF 不允许。
var imageMimes = map[string]bool{"image/jpeg": true, "image/png": true, "image/webp": true, "image/gif": true}

// 头像只允许 jpeg、png、webp（F-ACC-06 规则 3）。
var avatarMimes = map[string]bool{"image/jpeg": true, "image/png": true, "image/webp": true}

func sniff(b []byte) sniffResult {
	switch {
	case bytes.HasPrefix(b, []byte("\x89PNG\r\n\x1a\n")):
		r := sniffResult{Mime: "image/png"}
		if len(b) >= 24 && string(b[12:16]) == "IHDR" {
			r.Width, r.Height = int(binary.BigEndian.Uint32(b[16:20])), int(binary.BigEndian.Uint32(b[20:24]))
		}
		return r
	case bytes.HasPrefix(b, []byte{0xFF, 0xD8, 0xFF}):
		r := sniffResult{Mime: "image/jpeg"}
		r.Width, r.Height = jpegSize(b)
		return r
	case bytes.HasPrefix(b, []byte("GIF87a")) || bytes.HasPrefix(b, []byte("GIF89a")):
		r := sniffResult{Mime: "image/gif"}
		if len(b) >= 10 {
			r.Width, r.Height = int(binary.LittleEndian.Uint16(b[6:8])), int(binary.LittleEndian.Uint16(b[8:10]))
		}
		r.Frames = gifFrames(b)
		return r
	case len(b) >= 12 && string(b[0:4]) == "RIFF" && string(b[8:12]) == "WEBP":
		r := sniffResult{Mime: "image/webp"}
		r.Width, r.Height = webpSize(b)
		return r
	}
	if forbiddenMagic(b) {
		return sniffResult{Mime: octetStream, Forbidden: true}
	}
	ct := http.DetectContentType(b)
	ct, _, _ = strings.Cut(ct, ";")
	switch {
	case ct == "text/html":
		return sniffResult{Mime: ct, Forbidden: true}
	case ct == "text/xml" || ct == "text/plain":
		if looksLikeSVGOrHTML(b) {
			return sniffResult{Mime: ct, Forbidden: true}
		}
		if ct == "text/xml" {
			return sniffResult{Mime: "application/xml"}
		}
		return sniffResult{Mime: ct}
	case ct == "image/bmp" || ct == "image/x-icon" || ct == "image/vnd.microsoft.icon":
		return sniffResult{Mime: ct}
	case ct == "application/pdf" || ct == "application/zip" || ct == "application/x-gzip" || ct == "application/vnd.rar" ||
		ct == "application/x-rar-compressed" || ct == "application/x-7z-compressed" || ct == "application/ogg" ||
		strings.HasPrefix(ct, "audio/") || strings.HasPrefix(ct, "video/") || ct == "font/woff" || ct == "font/woff2" ||
		ct == "font/ttf" || ct == "font/otf":
		return sniffResult{Mime: ct}
	}
	return sniffResult{Mime: octetStream}
}

// forbiddenMagic 识别可执行文件和脚本的魔数（F-FILE-05 规则 1：真实类型禁止）。
func forbiddenMagic(b []byte) bool {
	for _, p := range [][]byte{
		[]byte("MZ"),               // PE（exe、dll、scr）
		[]byte("\x7fELF"),          // ELF
		{0xFE, 0xED, 0xFA, 0xCE},   // Mach-O
		{0xFE, 0xED, 0xFA, 0xCF},   //
		{0xCE, 0xFA, 0xED, 0xFE},   //
		{0xCF, 0xFA, 0xED, 0xFE},   //
		{0xCA, 0xFE, 0xBA, 0xBE},   // Mach-O fat / Java class
		[]byte("#!"),               // 脚本
		[]byte("\x4C\x00\x00\x00"), // Windows 快捷方式 lnk
	} {
		if bytes.HasPrefix(b, p) {
			return true
		}
	}
	return false
}

func looksLikeSVGOrHTML(b []byte) bool {
	head := strings.ToLower(string(b[:min(len(b), 4096)]))
	return strings.Contains(head, "<svg") || strings.Contains(head, "<html") || strings.Contains(head, "<script") || strings.Contains(head, "<!doctype html")
}

// jpegSize 在 SOFn 段里读宽高；解析不到返回 0（RS4-21：解析不到尺寸就拒绝）。
func jpegSize(b []byte) (int, int) {
	i := 2
	for i+4 <= len(b) {
		if b[i] != 0xFF {
			return 0, 0
		}
		marker := b[i+1]
		if marker == 0xFF { // 填充
			i++
			continue
		}
		if marker == 0xD8 || marker == 0x01 || (marker >= 0xD0 && marker <= 0xD7) {
			i += 2
			continue
		}
		if marker == 0xD9 || marker == 0xDA { // 图像结束、扫描开始之前都没有 SOF
			return 0, 0
		}
		segLen := int(binary.BigEndian.Uint16(b[i+2 : i+4]))
		if segLen < 2 {
			return 0, 0
		}
		isSOF := marker >= 0xC0 && marker <= 0xCF && marker != 0xC4 && marker != 0xC8 && marker != 0xCC
		if isSOF {
			if i+9 > len(b) {
				return 0, 0
			}
			h := int(binary.BigEndian.Uint16(b[i+5 : i+7]))
			w := int(binary.BigEndian.Uint16(b[i+7 : i+9]))
			return w, h
		}
		i += 2 + segLen
	}
	return 0, 0
}

func webpSize(b []byte) (int, int) {
	if len(b) < 30 {
		return 0, 0
	}
	switch string(b[12:16]) {
	case "VP8 ":
		// 帧头：3 字节帧标签 + 3 字节起始码 9D 01 2A + 宽高各 14 位。
		if b[23] != 0x9D || b[24] != 0x01 || b[25] != 0x2A {
			return 0, 0
		}
		return int(binary.LittleEndian.Uint16(b[26:28]) & 0x3FFF), int(binary.LittleEndian.Uint16(b[28:30]) & 0x3FFF)
	case "VP8L":
		if b[20] != 0x2F {
			return 0, 0
		}
		v := binary.LittleEndian.Uint32(b[21:25])
		return int(v&0x3FFF) + 1, int((v>>14)&0x3FFF) + 1
	case "VP8X":
		w := int(b[24]) | int(b[25])<<8 | int(b[26])<<16
		h := int(b[27]) | int(b[28])<<8 | int(b[29])<<16
		return w + 1, h + 1
	}
	return 0, 0
}

// gifFrames 数这段数据里的图像描述符（F-FILE-01 规则 3b：前 64 KB 内能看到的帧数）。数据截断时数到截断处。
func gifFrames(b []byte) int {
	if len(b) < 13 {
		return 0
	}
	i := 13
	if b[10]&0x80 != 0 { // 全局颜色表
		i += 3 * (1 << (int(b[10]&0x07) + 1))
	}
	frames := 0
	skipSub := func(j int) int { // 跳过数据子块，返回结束后的位置；截断返回 -1
		for j < len(b) {
			n := int(b[j])
			j++
			if n == 0 {
				return j
			}
			j += n
		}
		return -1
	}
	for i < len(b) {
		switch b[i] {
		case 0x21: // 扩展
			if i+2 > len(b) {
				return frames
			}
			if i = skipSub(i + 2); i < 0 {
				return frames
			}
		case 0x2C: // 图像描述符
			frames++
			if i+10 > len(b) {
				return frames
			}
			flags := b[i+9]
			i += 10
			if flags&0x80 != 0 {
				i += 3 * (1 << (int(flags&0x07) + 1))
			}
			if i >= len(b) {
				return frames
			}
			if i = skipSub(i + 1); i < 0 { // 跳过 LZW 最小码长
				return frames
			}
		case 0x3B:
			return frames
		default:
			return frames
		}
	}
	return frames
}
