package main

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

var ErrNotFound = errors.New("album not found")

type Album struct {
	ID        string    `json:"id" bson:"_id,omitempty"`
	Title     string    `json:"title" bson:"title"`
	Artist    string    `json:"artist" bson:"artist"`
	Price     float64   `json:"price" bson:"price"`
	CreatedAt time.Time `json:"created_at" bson:"created_at"`
	UpdatedAt time.Time `json:"updated_at" bson:"updated_at"`
}

type AlbumStore interface {
	List(ctx context.Context, query string, skip, limit int) ([]Album, int64, error)
	Get(ctx context.Context, id string) (Album, error)
	Create(ctx context.Context, album Album) (Album, error)
	Update(ctx context.Context, id string, album Album) (Album, error)
	Delete(ctx context.Context, id string) error
}

type AlbumHandler struct {
	store AlbumStore
}

func NewAlbumHandler(store AlbumStore) AlbumHandler {
	return AlbumHandler{store: store}
}

type albumInput struct {
	Title  string  `json:"title"`
	Artist string  `json:"artist"`
	Price  float64 `json:"price"`
}

type albumPage struct {
	Items      []Album `json:"items"`
	Page       int     `json:"page"`
	PerPage    int     `json:"per_page"`
	PageCount  int64   `json:"page_count"`
	TotalCount int64   `json:"total_count"`
}

func (h AlbumHandler) List(c *gin.Context) {
	page, err := queryInt(c, "page", 1)
	if err != nil || page < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "page must be a positive number"})
		return
	}

	perPage, err := queryInt(c, "per_page", 10)
	if err != nil || perPage < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "per_page must be a positive number"})
		return
	}
	if perPage > 100 {
		perPage = 100
	}

	maxInt := int(^uint(0) >> 1)
	if page > maxInt/perPage {
		c.JSON(http.StatusBadRequest, gin.H{"error": "page is too large"})
		return
	}
	skip := (page - 1) * perPage
	items, total, err := h.store.List(c.Request.Context(), strings.TrimSpace(c.Query("q")), skip, perPage)
	if err != nil {
		writeError(c, err)
		return
	}

	pageCount := (total + int64(perPage) - 1) / int64(perPage)
	c.JSON(http.StatusOK, albumPage{
		Items:      items,
		Page:       page,
		PerPage:    perPage,
		PageCount:  pageCount,
		TotalCount: total,
	})
}

func (h AlbumHandler) Get(c *gin.Context) {
	album, err := h.store.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, album)
}

func (h AlbumHandler) Create(c *gin.Context) {
	input, ok := readAlbumInput(c)
	if !ok {
		return
	}

	now := time.Now().UTC()
	album, err := h.store.Create(c.Request.Context(), Album{
		Title:     input.Title,
		Artist:    input.Artist,
		Price:     input.Price,
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, album)
}

func (h AlbumHandler) Update(c *gin.Context) {
	input, ok := readAlbumInput(c)
	if !ok {
		return
	}

	album, err := h.store.Update(c.Request.Context(), c.Param("id"), Album{
		Title:     input.Title,
		Artist:    input.Artist,
		Price:     input.Price,
		UpdatedAt: time.Now().UTC(),
	})
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, album)
}

func (h AlbumHandler) Delete(c *gin.Context) {
	if err := h.store.Delete(c.Request.Context(), c.Param("id")); err != nil {
		writeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func queryInt(c *gin.Context, key string, fallback int) (int, error) {
	value := c.Query(key)
	if value == "" {
		return fallback, nil
	}
	return strconv.Atoi(value)
}

func readAlbumInput(c *gin.Context) (albumInput, bool) {
	var input albumInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "request body must be valid JSON"})
		return albumInput{}, false
	}

	input.Title = strings.TrimSpace(input.Title)
	input.Artist = strings.TrimSpace(input.Artist)
	if input.Title == "" || input.Artist == "" || input.Price < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title and artist are required; price cannot be negative"})
		return albumInput{}, false
	}
	return input, true
}

func writeError(c *gin.Context, err error) {
	if errors.Is(err, ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "album not found"})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": "something went wrong"})
}
