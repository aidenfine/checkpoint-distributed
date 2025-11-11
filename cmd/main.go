package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/aidenfine/checkpoint"
)

func main() {

	checkpoint.LoadConfig()
	cfg := checkpoint.GetConfig()

	if cfg.ServiceUrl == "" {
		panic("missing SERVICE_URL env")
	}
	if cfg.Port == "" {
		panic("missing PORT env")
	}

	backendURL, _ := url.Parse(cfg.ServiceUrl)
	proxy := httputil.NewSingleHostReverseProxy(backendURL)

	limiter := checkpoint.NewTokenBucket(3, 75, 5)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		ip, err := getClientIP(r)
		if err != nil {
			fmt.Printf("Err when getting client ip:%v\n", err)
		}
		allowed, remaining := limiter.Allow(ip)
		if !allowed {
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprintf(w, "Rate limit exceeded! Try again later.\n")
			return
		}

		r.Header.Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
		fmt.Printf("[%s] allowed: %d tokens left\n", ip, remaining)

		proxy.ServeHTTP(w, r)
	})

	fmt.Printf("Reverse proxy :%s → forwarding to :%s \n", cfg.Port, cfg.ServiceUrl)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, nil))
}

// TODO: add test later
func getClientIP(r *http.Request) (string, error) {
	var ip string

	if tcip := r.Header.Get("True-Client-IP"); tcip != "" {
		fmt.Println("using true client ip")
		ip = tcip
	} else if xrip := r.Header.Get("X-Real-IP"); xrip != "" {
		fmt.Println("using x-real-ip")
		ip = xrip
	} else if cfip := r.Header.Get("CF-Connecting-IP"); cfip != "" {
		fmt.Printf("Found Cloudflare ip %s", cfip)
		ip = cfip
	} else if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		fmt.Println("using x-forwarded-for")
		i := strings.Index(xff, ", ")
		if i == -1 {
			i = len(xff)
		}
		ip = xff[:i]
	} else {
		var err error
		ip, _, err = net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			fmt.Println("using remote address")
			ip = r.RemoteAddr
		}
	}
	return ip, nil
}
