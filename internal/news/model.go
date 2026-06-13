package news

import (
	"errors"
	"time"
)

var ErrNewsNotFound = errors.New("news not found")
var ErrNewsSlugExists = errors.New("news slug already exists")
var ErrInvalidNewsInput = errors.New("invalid news input")

type InvalidInputError struct {
	Message string
}

func (e *InvalidInputError) Error() string {
	return e.Message
}

func (e *InvalidInputError) Unwrap() error {
	return ErrInvalidNewsInput
}

type News struct {
	Id          int
	Slug        string
	Category    string
	Title       string
	Excerpt     string
	Body        string
	Author      string
	ReadingTime int
	CoverImage  string
	Caption     string
	Formats     []string
	PublishedAt time.Time
	IsPublished bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type NewsCard struct {
	Id          int
	Category    string
	Title       string
	Excerpt     string
	Author      string
	ReadingTime int
	CoverImage  string
	Caption     string
	Formats     []string
	PublishedAt time.Time
}

type CreateNewsInput struct {
	Slug        string
	Category    string
	Title       string
	Excerpt     string
	Body        string
	Author      string
	ReadingTime int
	CoverImage  string
	Caption     string
	Formats     []string
	PublishedAt time.Time
	IsPublished bool
}
