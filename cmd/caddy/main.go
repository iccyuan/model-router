package main

import (
	caddycmd "github.com/caddyserver/caddy/v2/cmd"

	// 导入标准 Caddy 模块
	_ "github.com/caddyserver/caddy/v2/modules/standard"
	
	// 导入自定义模块
	_ "github.com/yourusername/caddy-model-router"
)

func main() {
	caddycmd.Main()
}
