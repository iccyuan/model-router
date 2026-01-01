package modelrouter

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
)

func init() {
	caddy.RegisterModule(ModelRouter{})
	httpcaddyfile.RegisterHandlerDirective("model_router", parseCaddyfile)
}

// ModelRouter 是一个 Caddy 处理程序，用于根据请求体中的 model 字段路由请求
type ModelRouter struct {
	// 可以添加配置字段，例如要匹配的模型名称
	TargetModel string `json:"target_model,omitempty"`
}

// CaddyModule 返回 Caddy 模块信息
func (ModelRouter) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.model_router",
		New: func() caddy.Module { return new(ModelRouter) },
	}
}

// Provision 实现 caddy.Provisioner 接口
func (m *ModelRouter) Provision(ctx caddy.Context) error {
	// 设置默认值
	if m.TargetModel == "" {
		m.TargetModel = "gpt-5.1-codex-mini"
	}
	return nil
}

// ServeHTTP 实现 caddyhttp.MiddlewareHandler 接口
func (m ModelRouter) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	// 检查请求路径是否包含 "v1/chat/completions"
	if !strings.Contains(r.URL.Path, "v1/chat/completions") {
		return next.ServeHTTP(w, r)
	}

	// 读取请求体
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	r.Body.Close()

	// 解析 JSON 请求体
	var requestBody map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &requestBody); err != nil {
		// 如果解析失败，恢复原始请求体并继续
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		return next.ServeHTTP(w, r)
	}

	// 检查是否包含目标 model
	if model, ok := requestBody["model"].(string); ok && model == m.TargetModel {
		// 替换路径中的 "v1/chat/completions" 为 "v1/responses"
		r.URL.Path = strings.Replace(r.URL.Path, "v1/chat/completions", "v1/responses", 1)
		r.RequestURI = strings.Replace(r.RequestURI, "v1/chat/completions", "v1/responses", 1)
	}

	// 恢复请求体供后续处理使用
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	return next.ServeHTTP(w, r)
}

// UnmarshalCaddyfile 实现 caddyfile.Unmarshaler 接口
func (m *ModelRouter) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	for d.Next() {
		for d.NextBlock(0) {
			switch d.Val() {
			case "target_model":
				if !d.Args(&m.TargetModel) {
					return d.ArgErr()
				}
			}
		}
	}
	return nil
}

// parseCaddyfile 解析 Caddyfile 配置
func parseCaddyfile(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	var m ModelRouter
	err := m.UnmarshalCaddyfile(h.Dispenser)
	return m, err
}

// Interface guards
var (
	_ caddy.Provisioner           = (*ModelRouter)(nil)
	_ caddyhttp.MiddlewareHandler = (*ModelRouter)(nil)
	_ caddyfile.Unmarshaler       = (*ModelRouter)(nil)
)
