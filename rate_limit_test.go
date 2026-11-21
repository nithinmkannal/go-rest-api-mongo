package main

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestRateLimitRejectsExcessRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RateLimit(2, time.Minute))
	router.GET("/", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	for i := 0; i < 3; i++ {
		response := serveFromIP(router, "192.0.2.1")
		if i < 2 && response.Code != http.StatusOK {
			t.Fatalf("request %d got status %d, want %d", i+1, response.Code, http.StatusOK)
		}
		if i == 2 {
			if response.Code != http.StatusTooManyRequests {
				t.Fatalf("got status %d, want %d", response.Code, http.StatusTooManyRequests)
			}
			if response.Header().Get("Retry-After") == "" {
				t.Fatal("expected Retry-After header")
			}
		}
	}
}

func TestRateLimitTracksClientsSeparately(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RateLimit(1, time.Minute))
	router.GET("/", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	for _, ip := range []string{"192.0.2.1", "192.0.2.2"} {
		if response := serveFromIP(router, ip); response.Code != http.StatusOK {
			t.Fatalf("client %s got status %d, want %d", ip, response.Code, http.StatusOK)
		}
	}
}

func TestRouterRateLimitIgnoresForwardedClientIP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := NewRouter(&fakeAlbumStore{})

	for i := 0; i < 61; i++ {
		request := httptest.NewRequest(http.MethodGet, "/albums", nil)
		request.RemoteAddr = "192.0.2.1:12345"
		request.Header.Set("X-Forwarded-For", "198.51.100."+strconv.Itoa(i+1))
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)

		wantStatus := http.StatusOK
		if i == 60 {
			wantStatus = http.StatusTooManyRequests
		}
		if response.Code != wantStatus {
			t.Fatalf("request %d got status %d, want %d", i+1, response.Code, wantStatus)
		}
	}
}

func TestRateLimitIsSafeForConcurrentRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const limit = 10
	router := gin.New()
	router.Use(RateLimit(limit, time.Minute))
	router.GET("/", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	const requests = 50
	var allowed atomic.Int32
	var rejected atomic.Int32
	var wait sync.WaitGroup
	for i := 0; i < requests; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			response := serveFromIP(router, "192.0.2.1")
			switch response.Code {
			case http.StatusOK:
				allowed.Add(1)
			case http.StatusTooManyRequests:
				rejected.Add(1)
			}
		}()
	}
	wait.Wait()

	if got := allowed.Load(); got != limit {
		t.Errorf("allowed %d requests, want %d", got, limit)
	}
	if got := rejected.Load(); got != requests-limit {
		t.Errorf("rejected %d requests, want %d", got, requests-limit)
	}
}

func serveFromIP(handler http.Handler, ip string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = ip + ":12345"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
