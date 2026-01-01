package modelrouter

import (
	"bytes"
	"encoding/json"
	"fmt"
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

// ModelRouter 实现路径重写的中间件
type ModelRouter struct {
	// 可配置的目标模型列表
	TargetModels []string `json:"target_models,omitempty"`
}

// CaddyModule 返回 Caddy 模块信息
func (ModelRouter) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.model_router",
		New: func() caddy.Module { return new(ModelRouter) },
	}
}

// Provision 设置模块
func (m *ModelRouter) Provision(ctx caddy.Context) error {
	// 如果没有配置目标模型，使用默认值
	if len(m.TargetModels) == 0 {
		m.TargetModels = []string{"gpt-5.1-codex-mini"}
	}
	return nil
}

// Validate 验证配置
func (m *ModelRouter) Validate() error {
	if len(m.TargetModels) == 0 {
		return fmt.Errorf("至少需要配置一个目标模型")
	}
	return nil
}

// ServeHTTP 实现 HTTP 处理逻辑
func (m ModelRouter) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	// 检查请求路径是否包含 "v1/chat/completions"
	if !strings.Contains(r.URL.Path, "v1/chat/completions") {
		// 不包含则直接传递给下一个处理程序
		return next.ServeHTTP(w, r)
	}

	// 读取请求体
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return caddyhttp.Error(http.StatusBadRequest, err)
	}
	r.Body.Close()

	// 解析 JSON
	var requestData map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &requestData); err != nil {
		// JSON 解析失败，恢复原始请求体并继续
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		return next.ServeHTTP(w, r)
	}

	// 检查是否包含目标模型
	shouldRewrite := false
	if model, ok := requestData["model"].(string); ok {
		for _, targetModel := range m.TargetModels {
			if model == targetModel {
				shouldRewrite = true
				break
			}
		}
	}

	// 如果匹配到目标模型，重写路径
	if shouldRewrite {
		r.URL.Path = strings.Replace(r.URL.Path, "v1/chat/completions", "v1/responses", 1)
		// 如果有 RawPath，也需要更新
		if r.URL.RawPath != "" {
			r.URL.RawPath = strings.Replace(r.URL.RawPath, "v1/chat/completions", "v1/responses", 1)
		}
	}

	// 恢复请求体（无论是否重写路径）
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	r.ContentLength = int64(len(bodyBytes))

	// 传递给下一个处理程序
	return next.ServeHTTP(w, r)
}

// UnmarshalCaddyfile 实现 Caddyfile 配置解析
func (m *ModelRouter) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	for d.Next() {
		// 解析配置块
		for d.NextBlock(0) {
			switch d.Val() {
			case "target_models":
				// 读取目标模型列表
				m.TargetModels = d.RemainingArgs()
				if len(m.TargetModels) == 0 {
					return d.ArgErr()
				}
			default:
				return d.Errf("未知的配置项: %s", d.Val())
			}
		}
	}
	return nil
}

// parseCaddyfile 解析 Caddyfile 指令
func parseCaddyfile(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	var m ModelRouter
	err := m.UnmarshalCaddyfile(h.Dispenser)
	return m, err
}

// 接口断言
var (
	_ caddy.Provisioner           = (*ModelRouter)(nil)
	_ caddy.Validator             = (*ModelRouter)(nil)
	_ caddyhttp.MiddlewareHandler = (*ModelRouter)(nil)
	_ caddyfile.Unmarshaler       = (*ModelRouter)(nil)
)
