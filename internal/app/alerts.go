package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"game-server-scripts/internal/system"
)

type AlertRule struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Metric    string  `json:"metric"`
	Operator  string  `json:"operator"`
	Threshold float64 `json:"threshold"`
	Enabled   bool    `json:"enabled"`
}

type AlertStatus struct {
	Rule      AlertRule `json:"rule"`
	Triggered bool      `json:"triggered"`
	Value     float64   `json:"value"`
	Message   string    `json:"message"`
	CheckedAt string    `json:"checkedAt"`
}

type AlertStore struct {
	path string
	mu   sync.Mutex
	list []AlertRule
}

func NewAlertStore(path string) *AlertStore {
	s := &AlertStore{path: path}
	_ = s.Load()
	return s
}

func defaultAlertRules() []AlertRule {
	return []AlertRule{
		{ID: "cpu-high", Name: "CPU 使用率过高", Metric: "cpu", Operator: ">=", Threshold: 85, Enabled: true},
		{ID: "memory-high", Name: "内存使用率过高", Metric: "memory", Operator: ">=", Threshold: 90, Enabled: true},
		{ID: "disk-high", Name: "磁盘使用率过高", Metric: "disk", Operator: ">=", Threshold: 90, Enabled: true},
		{ID: "network-failed", Name: "网络检测失败", Metric: "networkFailures", Operator: ">=", Threshold: 1, Enabled: true},
	}
}

func (s *AlertStore) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(s.path)
	if err != nil {
		s.list = defaultAlertRules()
		return nil
	}
	if err := json.Unmarshal(data, &s.list); err != nil {
		s.list = defaultAlertRules()
		return nil
	}
	if len(s.list) == 0 {
		s.list = defaultAlertRules()
	}
	return nil
}

func (s *AlertStore) List() []AlertRule {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]AlertRule, len(s.list))
	copy(out, s.list)
	return out
}

func (s *AlertStore) Replace(rules []AlertRule) error {
	if len(rules) == 0 {
		rules = defaultAlertRules()
	}
	for i := range rules {
		if rules[i].ID == "" {
			rules[i].ID = time.Now().Format("20060102150405")
		}
		if rules[i].Name == "" {
			rules[i].Name = rules[i].ID
		}
		if rules[i].Operator == "" {
			rules[i].Operator = ">="
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.list = rules
	return s.saveLocked()
}

func (s *AlertStore) Evaluate(info system.Info, checks []NetworkCheck) []AlertStatus {
	rules := s.List()
	networkFailures := 0.0
	for _, check := range checks {
		if !check.OK {
			networkFailures++
		}
	}
	now := time.Now().Format(time.RFC3339)
	out := make([]AlertStatus, 0, len(rules))
	for _, rule := range rules {
		value := alertValue(rule.Metric, info, networkFailures)
		triggered := rule.Enabled && compareAlert(value, rule.Operator, rule.Threshold)
		out = append(out, AlertStatus{Rule: rule, Triggered: triggered, Value: value, Message: alertMessage(rule, value, triggered), CheckedAt: now})
	}
	return out
}

func (s *AlertStore) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return err
	}
	data, _ := json.MarshalIndent(s.list, "", "  ")
	return os.WriteFile(s.path, data, 0644)
}

func alertValue(metric string, info system.Info, networkFailures float64) float64 {
	switch metric {
	case "cpu":
		return info.CPUPercent
	case "memory":
		return info.Memory.Percent
	case "disk":
		return info.Disk.Percent
	case "networkFailures":
		return networkFailures
	default:
		return 0
	}
}

func compareAlert(value float64, operator string, threshold float64) bool {
	switch operator {
	case ">":
		return value > threshold
	case ">=":
		return value >= threshold
	case "<":
		return value < threshold
	case "<=":
		return value <= threshold
	case "==":
		return value == threshold
	default:
		return value >= threshold
	}
}

func alertMessage(rule AlertRule, value float64, triggered bool) string {
	if !rule.Enabled {
		return "规则已停用"
	}
	if !triggered {
		return "正常"
	}
	return fmt.Sprintf("%s 当前值 %.1f，阈值 %s %.1f", rule.Name, value, rule.Operator, rule.Threshold)
}
