package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
)

type fakeAlbumStore struct {
	mu      sync.Mutex
	albums  []Album
	query   string
	skip    int
	limit   int
	listErr error
}

func (s *fakeAlbumStore) List(_ context.Context, query string, skip, limit int) ([]Album, int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.query = query
	s.skip = skip
	s.limit = limit
	if s.listErr != nil {
		return nil, 0, s.listErr
	}

	matches := make([]Album, 0)
	for _, album := range s.albums {
		text := strings.ToLower(album.Title + " " + album.Artist)
		if query == "" || strings.Contains(text, strings.ToLower(query)) {
			matches = append(matches, album)
		}
	}
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Title < matches[j].Title
	})

	total := int64(len(matches))
	if skip >= len(matches) {
		return []Album{}, total, nil
	}
	end := skip + limit
	if end > len(matches) {
		end = len(matches)
	}
	return matches[skip:end], total, nil
}

func (s *fakeAlbumStore) Get(context.Context, string) (Album, error) {
	return Album{}, ErrNotFound
}

func (s *fakeAlbumStore) Create(_ context.Context, album Album) (Album, error) {
	return album, nil
}

func (s *fakeAlbumStore) Update(context.Context, string, Album) (Album, error) {
	return Album{}, ErrNotFound
}

func (s *fakeAlbumStore) Delete(context.Context, string) error {
	return ErrNotFound
}

func TestAlbumListSearchAndPagination(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &fakeAlbumStore{albums: []Album{
		{ID: "1", Title: "Blue Train", Artist: "John Coltrane"},
		{ID: "2", Title: "Kind of Blue", Artist: "Miles Davis"},
		{ID: "3", Title: "Sketches of Spain", Artist: "Miles Davis"},
		{ID: "4", Title: "Abbey Road", Artist: "The Beatles"},
	}}
	router := gin.New()
	router.GET("/albums", NewAlbumHandler(store).List)

	request := httptest.NewRequest(http.MethodGet, "/albums?q=miles&page=2&per_page=1", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", response.Code, http.StatusOK)
	}

	var result albumPage
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if result.TotalCount != 2 || result.Page != 2 || result.PerPage != 1 || result.PageCount != 2 {
		t.Fatalf("unexpected page details: %+v", result)
	}
	if len(result.Items) != 1 || result.Items[0].ID != "3" {
		t.Fatalf("unexpected page items: %+v", result.Items)
	}
	if store.query != "miles" || store.skip != 1 || store.limit != 1 {
		t.Fatalf("store received query=%q skip=%d limit=%d", store.query, store.skip, store.limit)
	}
}

func TestAlbumListRejectsInvalidPage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/albums", NewAlbumHandler(&fakeAlbumStore{}).List)

	request := httptest.NewRequest(http.MethodGet, "/albums?page=abc", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestAlbumGetReturnsNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/albums/:id", NewAlbumHandler(&fakeAlbumStore{}).Get)

	request := httptest.NewRequest(http.MethodGet, "/albums/missing", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("got status %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestAlbumCreateRejectsMissingFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/albums", NewAlbumHandler(&fakeAlbumStore{}).Create)

	request := httptest.NewRequest(http.MethodPost, "/albums", strings.NewReader(`{"title":"No artist"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestAlbumListReportsStoreErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &fakeAlbumStore{listErr: errors.New("database unavailable")}
	router := gin.New()
	router.GET("/albums", NewAlbumHandler(store).List)

	request := httptest.NewRequest(http.MethodGet, "/albums", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("got status %d, want %d", response.Code, http.StatusInternalServerError)
	}
}
