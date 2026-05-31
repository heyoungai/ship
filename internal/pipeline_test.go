package internal

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRenderContextRenderString 验证模板变量可从 version、vars、module 等来源渲染。
func TestRenderContextRenderString(t *testing.T) {
	withTempConfigDir(t, map[string]string{
		"go.mod": "module example.com/demo\n\ngo 1.25.8\n",
	}, func() {
		cfg := &Config{}
		cfg.applyDefaults()
		cfg.ImageName = "demo"
		cfg.Project.Name = "demo"
		cfg.Vars = map[string]string{"remote_tag_key": "APP_IMAGE_TAG"}

		ctx := NewRenderContext(cfg, Profile{Name: "brand-a", Vars: map[string]string{"brand": "brand-a"}}, "v1.2.3")
		got, err := ctx.RenderString("{{ project.name }} {{ version }} {{ vars.brand }} {{ vars.remote_tag_key }} {{ module }}")
		if err != nil {
			t.Fatalf("RenderString error: %v", err)
		}
		want := "demo v1.2.3 brand-a APP_IMAGE_TAG example.com/demo"
		if got != want {
			t.Fatalf("RenderString = %q, want %q", got, want)
		}
	})
}

// TestExecuteTemplates_RenderContentAndFile 验证模板内容和输出路径都会被渲染并写盘。
func TestExecuteTemplates_RenderContentAndFile(t *testing.T) {
	withTempConfigDir(t, map[string]string{
		"templates/app.env.tpl": "APP_IMAGE_TAG={{ version }}\nBRAND={{ vars.brand }}\n",
	}, func() {
		cfg := &Config{}
		cfg.applyDefaults()
		cfg.Templates = []TemplateSpec{{
			Path:     "./dist/{{ vars.brand }}.env",
			From:     "./templates/app.env.tpl",
			Profiles: []string{"brand-a"},
		}}

		profile := Profile{Name: "brand-a", Vars: map[string]string{"brand": "brand-a"}}
		if err := ExecuteTemplates(cfg, profile, "v2.0.0"); err != nil {
			t.Fatalf("ExecuteTemplates error: %v", err)
		}

		data, err := os.ReadFile(filepath.Join("dist", "brand-a.env"))
		if err != nil {
			t.Fatalf("ReadFile rendered template error: %v", err)
		}
		content := string(data)
		if !strings.Contains(content, "APP_IMAGE_TAG=v2.0.0") || !strings.Contains(content, "BRAND=brand-a") {
			t.Fatalf("rendered template content = %q", content)
		}
	})
}

// TestStepAppliesToProfile 验证 steps 的 profiles 选择器与 enabled 开关生效。
func TestStepAppliesToProfile(t *testing.T) {
	falseValue := false
	profile := Profile{Name: "brand-a", Default: true}
	if !StepAppliesToProfile(Step{Profiles: []string{"brand-a"}}, profile) {
		t.Fatal("StepAppliesToProfile should match named profile")
	}
	if StepAppliesToProfile(Step{Profiles: []string{"brand-b"}}, profile) {
		t.Fatal("StepAppliesToProfile should reject unmatched profile")
	}
	if StepAppliesToProfile(Step{Profiles: []string{"*"}, Enabled: &falseValue}, profile) {
		t.Fatal("StepAppliesToProfile should respect enabled=false")
	}
}

// TestExecuteVerifyHTTP 验证 verify.http 走统一校验入口。
func TestExecuteVerifyHTTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &Config{}
	cfg.applyDefaults()
	cfg.Features.Verify = true
	cfg.Verify.Driver = "http"
	cfg.Verify.HTTP.URL = server.URL
	cfg.Verify.HTTP.Attempts = 1
	cfg.Verify.HTTP.IntervalSeconds = 1
	cfg.Verify.HTTP.TimeoutSeconds = 1

	if err := ExecuteVerify(cfg, Profile{}, "v1.0.0"); err != nil {
		t.Fatalf("ExecuteVerify(http) error: %v", err)
	}
}

// TestExecuteVerifyHTTPFailureWrapsContext 验证 verify.http 失败时会保留 driver 和目标 URL 上下文。
func TestExecuteVerifyHTTPFailureWrapsContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	cfg := &Config{}
	cfg.applyDefaults()
	cfg.Features.Verify = true
	cfg.Verify.Driver = "http"
	cfg.Verify.HTTP.URL = server.URL
	cfg.Verify.HTTP.Attempts = 1
	cfg.Verify.HTTP.IntervalSeconds = 1
	cfg.Verify.HTTP.TimeoutSeconds = 1

	err := ExecuteVerify(cfg, Profile{}, "v1.0.0")
	if err == nil {
		t.Fatal("ExecuteVerify(http) should fail when the endpoint is unhealthy")
	}
	if !strings.Contains(err.Error(), "verify.http 失败") || !strings.Contains(err.Error(), server.URL) {
		t.Fatalf("ExecuteVerify(http) error should contain verify driver and url, got: %v", err)
	}
}

// TestExecuteVerifyHTTPRejectsRenderedBlankURL 验证模板渲染为空时不会被静默跳过。
func TestExecuteVerifyHTTPRejectsRenderedBlankURL(t *testing.T) {
	cfg := &Config{}
	cfg.applyDefaults()
	cfg.Features.Verify = true
	cfg.Verify.Driver = "http"
	cfg.Verify.HTTP.URL = "{{ vars.empty_url }}"
	cfg.Vars = map[string]string{"empty_url": ""}

	err := ExecuteVerify(cfg, Profile{}, "v1.0.0")
	if err == nil {
		t.Fatal("ExecuteVerify(http) should fail when rendered url is blank")
	}
	if !strings.Contains(err.Error(), "verify.http.url 渲染结果不能为空") {
		t.Fatalf("ExecuteVerify(http) error should mention blank rendered url, got: %v", err)
	}
}

// TestUsesVerifyStage_LegacyHealthcheck 验证 legacy deploy.healthcheck 仍会触发 verify 阶段。
func TestUsesVerifyStage_LegacyHealthcheck(t *testing.T) {
	cfg := &Config{}
	cfg.applyDefaults()
	cfg.Features.Verify = false
	cfg.Verify.Driver = "none"
	cfg.Deploy.Healthcheck.URL = "https://example.com/health"

	if !cfg.UsesVerifyStage() {
		t.Fatal("UsesVerifyStage should be true when legacy deploy.healthcheck is configured")
	}
}
