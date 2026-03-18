package store

import "github.com/pionus/pionus-go/internal/model"

type Store interface {
	GetArticle(slug string) (*model.Article, error)
	ListArticles(page, limit int) ([]*model.Article, int, error)
	GetAdjacentArticles(slug string) (prev *model.Article, next *model.Article, err error)
	CreateArticle(a *model.Article) error
	UpdateArticle(slug string, a *model.Article) error
	DeleteArticle(slug string) error
	Close() error
}
