package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"sigs.k8s.io/yaml"
)

type Config struct {
	Kubernetes Kubernetes
	Check      Check
	Health     Health
	Feishu     Feishu
}

type Kubernetes struct {
	Kubeconfig string `json:"kubeconfig"`
	Context    string `json:"context"`
	Namespace  string `json:"namespace"`
	Deployment string `json:"deployment"`
}

type Check struct {
	Timeout     time.Duration `json:"timeout"`
	Interval    time.Duration `json:"interval"`
	MaxRestarts int           `json:"maxRestarts"`
}

type Health struct {
	URL           string        `json:"url"`
	Timeout       time.Duration `json:"timeout"`
	Retries       int           `json:"retries"`
	RetryInterval time.Duration `json:"retryInterval"`
}

type Feishu struct {
	Webhook string        `json:"webhook"`
	Secret  string        `json:"secret"`
	Timeout time.Duration `json:"timeout"`
}

type Overrides struct {
	Kubeconfig    string
	Context       string
	Namespace     string
	Deployment    string
	Timeout       time.Duration
	Interval      time.Duration
	MaxRestarts   *int
	HealthURL     string
	HealthTimeout time.Duration
	HealthRetries int
	RetryInterval time.Duration
	FeishuWebhook string
	FeishuSecret  string
	FeishuTimeout time.Duration
}

type fileConfig struct {
	Kubernetes Kubernetes `yaml:"kubernetes"`
	Check      struct {
		Timeout     string `yaml:"timeout"`
		Interval    string `yaml:"interval"`
		MaxRestarts *int   `yaml:"maxRestarts"`
	} `yaml:"check"`
	Health struct {
		URL           string `yaml:"url"`
		Timeout       string `yaml:"timeout"`
		Retries       *int   `yaml:"retries"`
		RetryInterval string `yaml:"retryInterval"`
	} `yaml:"health"`
	Feishu struct {
		Webhook string `yaml:"webhook"`
		Secret  string `yaml:"secret"`
		Timeout string `yaml:"timeout"`
	} `yaml:"feishu"`
}

func Default() Config {
	return Config{
		Check:  Check{Timeout: 3 * time.Minute, Interval: 5 * time.Second, MaxRestarts: 3},
		Health: Health{Timeout: 10 * time.Second, Retries: 3, RetryInterval: 5 * time.Second},
		Feishu: Feishu{Timeout: 10 * time.Second},
	}
}

func Load(path string, overrides Overrides) (Config, error) {
	config := Default()
	if path != "" {
		contents, err := os.ReadFile(path)
		if err != nil {
			return Config{}, fmt.Errorf("读取配置文件失败: %w", err)
		}
		var file fileConfig
		if err := yaml.Unmarshal(contents, &file); err != nil {
			return Config{}, fmt.Errorf("解析配置文件失败: %w", err)
		}
		config.Kubernetes = file.Kubernetes
		config.Health.URL = file.Health.URL
		config.Feishu.Webhook = file.Feishu.Webhook
		config.Feishu.Secret = file.Feishu.Secret
		if err := applyDuration(&config.Check.Timeout, file.Check.Timeout, "check.timeout"); err != nil {
			return Config{}, err
		}
		if err := applyDuration(&config.Check.Interval, file.Check.Interval, "check.interval"); err != nil {
			return Config{}, err
		}
		if file.Check.MaxRestarts != nil {
			config.Check.MaxRestarts = *file.Check.MaxRestarts
		}
		if err := applyDuration(&config.Health.Timeout, file.Health.Timeout, "health.timeout"); err != nil {
			return Config{}, err
		}
		if file.Health.Retries != nil {
			config.Health.Retries = *file.Health.Retries
		}
		if err := applyDuration(&config.Health.RetryInterval, file.Health.RetryInterval, "health.retryInterval"); err != nil {
			return Config{}, err
		}
		if err := applyDuration(&config.Feishu.Timeout, file.Feishu.Timeout, "feishu.timeout"); err != nil {
			return Config{}, err
		}
	}
	applyOverrides(&config, overrides)
	return config, config.Validate()
}

func (config Config) Validate() error {
	if strings.TrimSpace(config.Kubernetes.Namespace) == "" {
		return fmt.Errorf("缺少必填配置 kubernetes.namespace")
	}
	if strings.TrimSpace(config.Kubernetes.Deployment) == "" {
		return fmt.Errorf("缺少必填配置 kubernetes.deployment")
	}
	if config.Check.Timeout <= 0 || config.Check.Interval <= 0 {
		return fmt.Errorf("check.timeout 和 check.interval 必须为正数")
	}
	if config.Check.MaxRestarts < 0 {
		return fmt.Errorf("check.maxRestarts 不能为负数")
	}
	if config.Health.URL != "" {
		if !validHTTPURL(config.Health.URL) {
			return fmt.Errorf("health.url 必须是绝对的 HTTP(S) 地址")
		}
		if config.Health.Timeout <= 0 || config.Health.RetryInterval <= 0 || config.Health.Retries < 1 {
			return fmt.Errorf("health.timeout、health.retryInterval 和 health.retries 必须为正数")
		}
	}
	if config.Feishu.Webhook != "" {
		if !validHTTPURL(config.Feishu.Webhook) {
			return fmt.Errorf("feishu.webhook 必须是绝对的 HTTP(S) 地址")
		}
		if config.Feishu.Timeout <= 0 {
			return fmt.Errorf("feishu.timeout 必须为正数")
		}
	}
	return nil
}

func applyOverrides(config *Config, overrides Overrides) {
	if overrides.Kubeconfig != "" {
		config.Kubernetes.Kubeconfig = overrides.Kubeconfig
	}
	if overrides.Context != "" {
		config.Kubernetes.Context = overrides.Context
	}
	if overrides.Namespace != "" {
		config.Kubernetes.Namespace = overrides.Namespace
	}
	if overrides.Deployment != "" {
		config.Kubernetes.Deployment = overrides.Deployment
	}
	if overrides.Timeout > 0 {
		config.Check.Timeout = overrides.Timeout
	}
	if overrides.Interval > 0 {
		config.Check.Interval = overrides.Interval
	}
	if overrides.MaxRestarts != nil {
		config.Check.MaxRestarts = *overrides.MaxRestarts
	}
	if overrides.HealthURL != "" {
		config.Health.URL = overrides.HealthURL
	}
	if overrides.HealthTimeout > 0 {
		config.Health.Timeout = overrides.HealthTimeout
	}
	if overrides.HealthRetries > 0 {
		config.Health.Retries = overrides.HealthRetries
	}
	if overrides.RetryInterval > 0 {
		config.Health.RetryInterval = overrides.RetryInterval
	}
	if overrides.FeishuWebhook != "" {
		config.Feishu.Webhook = overrides.FeishuWebhook
	}
	if overrides.FeishuSecret != "" {
		config.Feishu.Secret = overrides.FeishuSecret
	}
	if overrides.FeishuTimeout > 0 {
		config.Feishu.Timeout = overrides.FeishuTimeout
	}
}

func validHTTPURL(raw string) bool {
	parsed, err := url.ParseRequestURI(raw)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}

func applyDuration(target *time.Duration, raw string, name string) error {
	if raw == "" {
		return nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil {
		return fmt.Errorf("解析 %s 失败: %w", name, err)
	}
	*target = value
	return nil
}
