package redact

import (
	"net"
	"regexp"
	"strings"
)

var (
	ipv4     = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
	mac      = regexp.MustCompile(`(?i)\b(?:[0-9a-f]{2}:){5}[0-9a-f]{2}\b`)
	token    = regexp.MustCompile(`(?i)(token|password|secret|api[_-]?key)=([^\s,;]+)`)
	homePath = regexp.MustCompile(`(?i)(/home/|\\users\\)[^/\\\s]+`)
)

func Text(value string) string {
	value = mac.ReplaceAllString(value, "00:00:00:00:00:00")
	value = ipv4.ReplaceAllStringFunc(value, func(candidate string) string {
		if net.ParseIP(candidate) == nil {
			return candidate
		}
		return "192.0.2.1"
	})
	return token.ReplaceAllString(value, "$1=[REDACTED]")
}

func Path(value string) string { return homePath.ReplaceAllString(value, "$1[USER]") }
func ModelPath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.ContainsAny(value, "/\\") {
		return "/models/[REDACTED]"
	}
	return "[REDACTED_MODEL]"
}
func Env(values map[string]string) map[string]string {
	result := map[string]string{}
	for key, value := range values {
		upper := strings.ToUpper(key)
		if strings.Contains(upper, "TOKEN") || strings.Contains(upper, "PASSWORD") || strings.Contains(upper, "SECRET") || strings.Contains(upper, "KEY") {
			result[key] = "[REDACTED]"
		} else {
			result[key] = Text(value)
		}
	}
	return result
}
