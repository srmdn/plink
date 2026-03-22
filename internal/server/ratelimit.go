package server

import (
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

func clientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		return strings.Split(fwd, ",")[0]
	}
	addr := r.RemoteAddr
	if i := strings.LastIndex(addr, ":"); i != -1 {
		return addr[:i]
	}
	return addr
}
