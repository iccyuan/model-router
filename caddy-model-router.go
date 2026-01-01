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
	"go.uber.org/zap"
)

func init() {
	caddy.RegisterModule(ModelRouter{})
	httpcaddyfile.RegisterHandlerDirective("model_router", parseCaddyfile)
}

// ModelRouter 实现路径重写的中间件
type ModelRouter struct {
	// 可配置的目标模型列表
	TargetModels []string `json:"target_models,omitempty"`
	
	logger *zap.Logger
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
	m.logger = ctx.Logger(m)
	
	// 如果没有配置目标模型，使用默认值
	if len(m.TargetModels) == 0 {
		m.TargetModels = []string{"gpt-5.1-codex-mini"}
	}
	
	m.logger.Info("ModelRouter 已初始化",
		zap.Strings("target_models", m.TargetModels),
	)
	
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
	m.logger.Info("收到请求",
		zap.String("method", r.Method),
		zap.String("path", r.URL.Path),
		zap.String("host", r.Host),
	)

	// 检查请求路径是否包含 "chat/completions"（支持 /api/chat/completions 和 /v1/chat/completions）
	if !strings.Contains(r.URL.Path, "chat/completions") {
		m.logger.Info("路径不匹配，跳过处理",
			zap.String("path", r.URL.Path),
		)
		// 不包含则直接传递给下一个处理程序
		return next.ServeHTTP(w, r)
	}

	m.logger.Info("路径匹配，开始处理")

	// 读取请求体
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		m.logger.Error("读取请求体失败", zap.Error(err))
		return caddyhttp.Error(http.StatusBadRequest, err)
	}
	r.Body.Close()

	m.logger.Info("请求体内容",
		zap.String("body", string(bodyBytes)),
		zap.Int("length", len(bodyBytes)),
	)

	// 解析 JSON
	var requestData map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &requestData); err != nil {
		m.logger.Warn("JSON 解析失败，恢复原始请求体",
			zap.Error(err),
			zap.String("body", string(bodyBytes)),
		)
		// JSON 解析失败，恢复原始请求体并继续
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		return next.ServeHTTP(w, r)
	}

	m.logger.Info("JSON 解析成功",
		zap.Any("data", requestData),
	)

	// 检查是否包含目标模型
	shouldRewrite := false
	modelValue := ""
	if model, ok := requestData["model"].(string); ok {
		modelValue = model
		m.logger.Info("检测到模型字段",
			zap.String("model", model),
		)
		
		for _, targetModel := range m.TargetModels {
			m.logger.Info("比较模型",
				zap.String("request_model", model),
				zap.String("target_model", targetModel),
			)
			if model == targetModel {
				shouldRewrite = true
				m.logger.Info("模型匹配成功！",
					zap.String("matched_model", targetModel),
				)
				break
			}
		}
	} else {
		m.logger.Warn("请求中未找到 model 字段或类型不正确",
			zap.Any("model_value", requestData["model"]),
		)
	}

	// 如果匹配到目标模型，重写路径
	if shouldRewrite {
		oldPath := r.URL.Path
		
		// 支持多种路径格式的重写
		if strings.Contains(r.URL.Path, "/v1/chat/completions") {
			r.URL.Path = strings.Replace(r.URL.Path, "/v1/chat/completions", "/v1/responses", 1)
		} else if strings.Contains(r.URL.Path, "/api/chat/completions") {
			r.URL.Path = strings.Replace(r.URL.Path, "/api/chat/completions", "/api/responses", 1)
		} else {
			// 通用替换
			r.URL.Path = strings.Replace(r.URL.Path, "chat/completions", "responses", 1)
		}
		
		m.logger.Info("路径重写成功",
			zap.String("old_path", oldPath),
			zap.String("new_path", r.URL.Path),
			zap.String("model", modelValue),
		)
		
		// 如果有 RawPath，也需要更新
		if r.URL.RawPath != "" {
			oldRawPath := r.URL.RawPath
			if strings.Contains(r.URL.RawPath, "/v1/chat/completions") {
				r.URL.RawPath = strings.Replace(r.URL.RawPath, "/v1/chat/completions", "/v1/responses", 1)
			} else if strings.Contains(r.URL.RawPath, "/api/chat/completions") {
				r.URL.RawPath = strings.Replace(r.URL.RawPath, "/api/chat/completions", "/api/responses", 1)
			} else {
				r.URL.RawPath = strings.Replace(r.URL.RawPath, "chat/completions", "responses", 1)
			}
			m.logger.Info("RawPath 也已重写",
				zap.String("old_raw_path", oldRawPath),
				zap.String("new_raw_path", r.URL.RawPath),
			)
		}
	} else {
		m.logger.Info("模型不匹配，不进行路径重写",
			zap.String("request_model", modelValue),
			zap.Strings("target_models", m.TargetModels),
		)
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
