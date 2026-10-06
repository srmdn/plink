package server

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	_ "golang.org/x/image/webp"
)

const maxLogoBytes = 2 << 20

// Never let a configured public logo reach private networks or proxy credentials.
func publicLogoIP(ip net.IP) bool {
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return false
	}
	addr = addr.Unmap()
	if !addr.IsGlobalUnicast() || addr.IsPrivate() || addr.IsLoopback() || addr.IsLinkLocalUnicast() {
		return false
	}
	for _, block := range []string{"100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "2001:db8::/32", "64:ff9b::/96", "64:ff9b:1::/48", "2002::/16", "2001::/32"} {
		if netip.MustParsePrefix(block).Contains(addr) {
			return false
		}
	}
	return true
}

func (s *Server) ogLogo(raw string) image.Image {
	if raw == "" || !validBrandImage(raw) {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil
	}
	var data []byte
	if !u.IsAbs() {
		// Only embedded public assets or uploaded media; never interpret a URL
		// as an arbitrary filesystem path.
		clean := path.Clean(u.Path)
		switch {
		case strings.HasPrefix(clean, "/js/"):
			data, err = s.webFS.ReadFile("web" + clean)
		case strings.HasPrefix(clean, "/media/") && mediaNamePattern.MatchString(path.Base(clean)):
			data, err = os.ReadFile(filepath.Join(s.uploadsDir, path.Base(clean)))
		default:
			return nil
		}
	} else {
		transport := &http.Transport{DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
			if err != nil {
				return nil, err
			}
			if len(ips) == 0 {
				return nil, fmt.Errorf("no address")
			}
			for _, ip := range ips {
				if !publicLogoIP(ip) {
					return nil, fmt.Errorf("non-public address")
				}
			}
			dialer := net.Dialer{Timeout: 2 * time.Second}
			return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].String(), port))
		}}
		defer transport.CloseIdleConnections()
		client := http.Client{Transport: transport, Timeout: 3 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 || req.URL.Scheme != "https" || req.URL.User != nil {
				return fmt.Errorf("redirect rejected")
			}
			return nil
		}}
		response, e := client.Get(raw)
		if e != nil {
			return nil
		}
		defer response.Body.Close()
		if response.StatusCode != 200 {
			return nil
		}
		data, err = io.ReadAll(io.LimitReader(response.Body, maxLogoBytes+1))
	}
	if err != nil || len(data) > maxLogoBytes {
		return nil
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width)*int64(cfg.Height) > 4_000_000 {
		return nil
	}
	logo, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	return logo
}
