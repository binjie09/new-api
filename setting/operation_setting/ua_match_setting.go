package operation_setting

import (
	"regexp"
	"sync"

	"github.com/QuantumNous/new-api/setting/config"
)

type UAMatchRule struct {
	Name       string `json:"name"`
	Regex      string `json:"regex"`
	StatusCode int    `json:"status_code"`
	Body       string `json:"body"`
}

type UAMatchSetting struct {
	Enabled bool           `json:"enabled"`
	Rules   []UAMatchRule  `json:"rules"`
}

var uaMatchSetting = UAMatchSetting{
	Enabled: false,
	Rules:   []UAMatchRule{},
}

// compiled rules cache
var (
	uaCompiledMu   sync.RWMutex
	uaCompiledRules []*regexp.Regexp
)

func init() {
	config.GlobalConfig.Register("ua_match_setting", &uaMatchSetting)
}

func GetUAMatchSetting() *UAMatchSetting {
	return &uaMatchSetting
}

func GetCompiledUARules() []*regexp.Regexp {
	uaCompiledMu.RLock()
	defer uaCompiledMu.RUnlock()
	return uaCompiledRules
}

func CompileUARules() {
	uaCompiledMu.Lock()
	defer uaCompiledMu.Unlock()

	compiled := make([]*regexp.Regexp, 0, len(uaMatchSetting.Rules))
	for _, rule := range uaMatchSetting.Rules {
		re, err := regexp.Compile(rule.Regex)
		if err != nil {
			continue
		}
		compiled = append(compiled, re)
	}
	uaCompiledRules = compiled
}

func MatchUA(userAgent string) *UAMatchRule {
	if !uaMatchSetting.Enabled || userAgent == "" {
		return nil
	}
	uaCompiledMu.RLock()
	rules := uaCompiledRules
	uaCompiledMu.RUnlock()

	setting := GetUAMatchSetting()
	for i, re := range rules {
		if i >= len(setting.Rules) {
			break
		}
		if re.MatchString(userAgent) {
			return &setting.Rules[i]
		}
	}
	return nil
}
