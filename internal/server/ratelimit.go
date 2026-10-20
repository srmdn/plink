package server

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	maxLoginAttempts = 5
	loginWindow      = 15 * time.Minute
)

type loginAttempt struct {
	count    int
	lastSeen time.Time
}

type loginLimiter struct {
	mu       sync.Mutex
	attempts map[string]*loginAttempt
}

func newLoginLimiter() *loginLimiter {
	l := &loginLimiter{attempts: make(map[string]*loginAttempt)}
	go l.cleanup()
	return l
}

func (l *loginLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	a, ok := l.attempts[ip]
	if !ok {
		l.attempts[ip] = &loginAttempt{count: 1, lastSeen: time.Now()}
		return true
	}
	if time.Since(a.lastSeen) > loginWindow {
		a.count = 1
		a.lastSeen = time.Now()
		return true
	}
	if a.count >= maxLoginAttempts {
		return false
	}
	a.count++
	a.lastSeen = time.Now()
	return true
}

func (l *loginLimiter) reset(ip string) {
	l.mu.Lock()
	delete(l.attempts, ip)
	l.mu.Unlock()
}

func (l *loginLimiter) cleanup() {
	for range time.Tick(time.Hour) {
		l.mu.Lock()
		for ip, a := range l.attempts {
			if time.Since(a.lastSeen) > loginWindow {
				delete(l.attempts, ip)
			}
		}
		l.mu.Unlock()
	}
}

func remoteIP(r *http.Request) string {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return strings.Trim(r.RemoteAddr, "[]")
}

func isLoopbackRemote(r *http.Request) bool {
	ip := net.ParseIP(remoteIP(r))
	return ip != nil && ip.IsLoopback()
}

func clientIP(r *http.Request) string {
	if isLoopbackRemote(r) {
		if ip := strings.TrimSpace(r.Header.Get("X-Real-IP")); ip != "" {
			return ip
		}
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			if first := strings.TrimSpace(strings.Split(fwd, ",")[0]); first != "" {
				return first
			}
		}
	}
	return remoteIP(r)
}
