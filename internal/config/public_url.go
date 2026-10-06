package config

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// NormalizePublicURL accepts an origin only: never credentials, query or a path.
func NormalizePublicURL(raw string, production bool) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	u, err := url.Parse(raw)
	if err != nil || u.User != nil || u.Hostname() == "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.RawFragment != "" || (u.Path != "" && u.Path != "/") || u.RawPath != "" || strings.ContainsAny(raw, "\r\n\\") {
		return "", fmt.Errorf("alamat harus berupa origin tanpa path, query, atau kredensial")
	}
	if u.Scheme != "https" && (u.Scheme != "http" || production) {
		return "", fmt.Errorf("alamat publik harus HTTPS; HTTP hanya untuk pengembangan lokal")
	}
	host := u.Hostname()
	if strings.HasSuffix(u.Host, ":") {
		return "", fmt.Errorf("port tidak valid")
	}
	if net.ParseIP(host) == nil {
		for _, label := range strings.Split(strings.TrimSuffix(host, "."), ".") {
			if label == "" || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
				return "", fmt.Errorf("nama domain tidak valid")
			}
		}
		for _, c := range host {
			if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '.' || c == '-') {
				return "", fmt.Errorf("nama domain tidak valid")
			}
		}
	}
	if port := u.Port(); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return "", fmt.Errorf("port tidak valid")
		}
	}
	u.Path = ""
	u.Host = strings.ToLower(u.Host)
	return u.String(), nil
}
