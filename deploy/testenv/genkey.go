//go:build ignore

// 生成一对 Ed25519 密钥：输出 "base64种子 base64公钥"。macOS 自带的 openssl 不支持 ed25519，这里不依赖它。
package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
)

func main() {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		panic(err)
	}
	e := base64.StdEncoding.EncodeToString
	fmt.Println(e(priv.Seed()), e(pub))
}
