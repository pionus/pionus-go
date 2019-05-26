package resolver

import (
    graphql "github.com/graph-gophers/graphql-go"

    "../../models"
)


type ArticleResolver struct {
    article *models.Article
}

func (r *ArticleResolver) ID() graphql.ID {
    return graphql.ID(r.article.ID)
}

func (r *ArticleResolver) Content() string {
    return r.article.Content
}
