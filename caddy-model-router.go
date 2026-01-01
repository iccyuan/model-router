package handler

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
	caddy.RegisterModule(ModelRouteHandler{})
	httpcaddyfile.RegisterHandlerDirective("model_route", parseCaddyfile)
}

// ModelRouteHandler 实现路由重写处理器
type ModelRouteHandler struct {
	// 可配置的模型列表，如果为空则默认只检查 gpt-5.1-codex-mini
	Models []string `json:"models,omitempty"`
}

// CaddyModule 返回 Caddy 模块信息
func (ModelRouteHandler) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.model_route",
		New: func() caddy.Module { return new(ModelRouteHandler) },
	}
}

// Provision 实现 caddy.Provisioner 接口
func (m *ModelRouteHandler) Provision(ctx caddy.Context) error {
	// 如果没有配置模型列表，使用默认值
	if len(m.Models) == 0 {
		m.Models = []string{"gpt-5.1-codex-mini"}
	}
	return nil
}

// ServeHTTP 实现 caddyhttp.MiddlewareHandler 接口
func (m ModelRouteHandler) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
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

	// 解析 JSON
	var requestData map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &requestData); err != nil {
		// 如果解析失败，恢复原始请求体并继续
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		return next.ServeHTTP(w, r)
	}

	// 检查是否包含目标模型
	shouldRewrite := false
	if model, ok := requestData["model"].(string); ok {
		for _, targetModel := range m.Models {
			if model == targetModel {
				shouldRewrite = true
				break
			}
		}
	}

	// 如果匹配到目标模型，重写路径
	if shouldRewrite {
		r.URL.Path = strings.Replace(r.URL.Path, "v1/chat/completions", "v1/responses", 1)
		// 如果需要也更新 RequestURI
		r.RequestURI = strings.Replace(r.RequestURI, "v1/chat/completions", "v1/responses", 1)
	}

	// 恢复请求体
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	return next.ServeHTTP(w, r)
}

// UnmarshalCaddyfile 实现 caddyfile.Unmarshaler 接口
func (m *ModelRouteHandler) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	for d.Next() {
		for d.NextBlock(0) {
			switch d.Val() {
			case "models":
				m.Models = d.RemainingArgs()
				if len(m.Models) == 0 {
					return d.ArgErr()
				}
			}
		}
	}
	return nil
}

// parseCaddyfile 解析 Caddyfile 配置
func parseCaddyfile(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	var m ModelRouteHandler
	err := m.UnmarshalCaddyfile(h.Dispenser)
	return m, err
}

// Interface guards
var (
	_ caddy.Provisioner           = (*ModelRouteHandler)(nil)
	_ caddyhttp.MiddlewareHandler = (*ModelRouteHandler)(nil)
	_ caddyfile.Unmarshaler       = (*ModelRouteHandler)(nil)
)
