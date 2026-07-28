package moderation

import "testing"

func TestDefaultConfig(t *testing.T) {
	c := DefaultConfig()
	if c.Enabled {
		t.Errorf("Enabled = true, want false")
	}
	if c.Provider != "webhook" {
		t.Errorf("Provider = %q, want webhook", c.Provider)
	}
	if c.Threshold != 0.8 {
		t.Errorf("Threshold = %v, want 0.8", c.Threshold)
	}
	if c.Action != "pending" {
		t.Errorf("Action = %q, want pending", c.Action)
	}
	if c.Endpoint != "" || c.APIKey != "" {
		t.Errorf("Endpoint/APIKey 默认应为空: %+v", c)
	}
	if c.AccessKeyID != "" || c.AccessKeySecret != "" || c.Region != "" {
		t.Errorf("AccessKeyID/AccessKeySecret/Region 默认应为空: %+v", c)
	}
	// 默认配置本身必须校验通过（disabled 时 endpoint 允许为空）。
	if err := ValidateConfig(c); err != nil {
		t.Errorf("默认配置未通过校验: %v", err)
	}
}

func TestValidateConfigThreshold(t *testing.T) {
	base := DefaultConfig()
	base.Endpoint = "https://mod.example.com/score"

	cases := []struct {
		name      string
		threshold float64
		wantErr   bool
	}{
		{"下界", 0, false},
		{"上界", 1, false},
		{"中间", 0.5, false},
		{"负数", -0.01, true},
		{"超过1", 1.01, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg := base
			cfg.Threshold = c.threshold
			err := ValidateConfig(cfg)
			if c.wantErr && err == nil {
				t.Errorf("threshold=%v 应报错", c.threshold)
			}
			if !c.wantErr && err != nil {
				t.Errorf("threshold=%v 不应报错: %v", c.threshold, err)
			}
		})
	}
}

func TestValidateConfigAction(t *testing.T) {
	base := DefaultConfig()
	base.Endpoint = "https://mod.example.com/score"

	for _, action := range []string{"pending", "rejected"} {
		cfg := base
		cfg.Action = action
		if err := ValidateConfig(cfg); err != nil {
			t.Errorf("action=%q 不应报错: %v", action, err)
		}
	}
	for _, action := range []string{"", "approved", "deleted", "ban"} {
		cfg := base
		cfg.Action = action
		if err := ValidateConfig(cfg); err == nil {
			t.Errorf("action=%q 应报错", action)
		}
	}
}

func TestValidateConfigProvider(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Provider = "other"
	if err := ValidateConfig(cfg); err == nil {
		t.Errorf("provider=other 应报错")
	}
	cfg.Provider = ""
	if err := ValidateConfig(cfg); err == nil {
		t.Errorf("provider 为空应报错")
	}
}

func TestValidateConfigEndpointRequiredWhenEnabled(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.Endpoint = ""
	if err := ValidateConfig(cfg); err == nil {
		t.Errorf("enabled 但 endpoint 为空应报错")
	}
	cfg.Endpoint = "not-a-url"
	if err := ValidateConfig(cfg); err == nil {
		t.Errorf("enabled 但 endpoint 非法应报错")
	}
	cfg.Endpoint = "ftp://mod.example.com"
	if err := ValidateConfig(cfg); err == nil {
		t.Errorf("enabled 但 endpoint 非 http(s) 应报错")
	}
	cfg.Endpoint = "https://mod.example.com/score"
	if err := ValidateConfig(cfg); err != nil {
		t.Errorf("enabled 且 endpoint 合法不应报错: %v", err)
	}
	cfg.Endpoint = "http://127.0.0.1:9000/score"
	if err := ValidateConfig(cfg); err != nil {
		t.Errorf("裁决9：内网 endpoint 不做 SSRF 限制，应放行: %v", err)
	}
}

func TestValidateConfigDisabledAllowsEmptyEndpoint(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = false
	cfg.Endpoint = ""
	if err := ValidateConfig(cfg); err != nil {
		t.Errorf("disabled 时 endpoint 允许为空: %v", err)
	}
}

func TestValidateProviderMatrix(t *testing.T) {
	// disabled 各 provider 合法（不校验凭据/endpoint）
	for _, p := range []string{"webhook", "aliyun", "tencent", "openai", "nsfwjs"} {
		cfg := DefaultConfig()
		cfg.Provider = p
		cfg.Enabled = false
		if err := ValidateConfig(cfg); err != nil {
			t.Errorf("disabled provider=%q 应合法: %v", p, err)
		}
	}

	// enabled webhook 缺 endpoint → ErrEndpointInvalid；有则合法
	{
		cfg := DefaultConfig()
		cfg.Enabled = true
		cfg.Provider = "webhook"
		cfg.Endpoint = ""
		if err := ValidateConfig(cfg); err != ErrEndpointInvalid {
			t.Errorf("enabled webhook 缺 endpoint: err=%v want ErrEndpointInvalid", err)
		}
		cfg.Endpoint = "https://mod.example.com/score"
		if err := ValidateConfig(cfg); err != nil {
			t.Errorf("enabled webhook 有 endpoint 应合法: %v", err)
		}
	}

	// enabled nsfwjs 同 webhook：需 endpoint
	{
		cfg := DefaultConfig()
		cfg.Enabled = true
		cfg.Provider = "nsfwjs"
		cfg.Endpoint = ""
		if err := ValidateConfig(cfg); err != ErrEndpointInvalid {
			t.Errorf("enabled nsfwjs 缺 endpoint: err=%v want ErrEndpointInvalid", err)
		}
		cfg.Endpoint = "http://127.0.0.1:8080/score"
		if err := ValidateConfig(cfg); err != nil {
			t.Errorf("enabled nsfwjs 有 endpoint 应合法: %v", err)
		}
	}

	// enabled openai 缺 api_key → ErrCredentialMissing；有则合法
	{
		cfg := DefaultConfig()
		cfg.Enabled = true
		cfg.Provider = "openai"
		cfg.APIKey = ""
		if err := ValidateConfig(cfg); err != ErrCredentialMissing {
			t.Errorf("enabled openai 缺 api_key: err=%v want ErrCredentialMissing", err)
		}
		cfg.APIKey = "sk-test"
		if err := ValidateConfig(cfg); err != nil {
			t.Errorf("enabled openai 有 api_key 应合法: %v", err)
		}
	}

	// enabled aliyun/tencent 缺 region/AKID/AKSecret 任一 → ErrCredentialMissing；三者齐合法
	for _, p := range []string{"aliyun", "tencent"} {
		base := DefaultConfig()
		base.Enabled = true
		base.Provider = p
		base.AccessKeyID = "akid"
		base.AccessKeySecret = "secret"
		base.Region = "cn-hangzhou"

		if err := ValidateConfig(base); err != nil {
			t.Errorf("enabled %s 三者齐应合法: %v", p, err)
		}
		for _, miss := range []struct {
			name string
			mut  func(*Config)
		}{
			{"缺 AccessKeyID", func(c *Config) { c.AccessKeyID = "" }},
			{"缺 AccessKeySecret", func(c *Config) { c.AccessKeySecret = "" }},
			{"缺 Region", func(c *Config) { c.Region = "" }},
		} {
			cfg := base
			miss.mut(&cfg)
			if err := ValidateConfig(cfg); err != ErrCredentialMissing {
				t.Errorf("enabled %s %s: err=%v want ErrCredentialMissing", p, miss.name, err)
			}
		}
	}

	// provider "foo" → ErrProviderInvalid
	{
		cfg := DefaultConfig()
		cfg.Provider = "foo"
		if err := ValidateConfig(cfg); err != ErrProviderInvalid {
			t.Errorf("provider=foo: err=%v want ErrProviderInvalid", err)
		}
	}
}
