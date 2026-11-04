//go:build ignore

// 生成一对 Ed25519 密钥：输出 "base64种子 base64公钥"（console-server 和 IM_OPS_PUBKEYS 要的是原始密钥，不是 PEM）。
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
