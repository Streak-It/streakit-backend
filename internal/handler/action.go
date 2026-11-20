package handler

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/niteshs1/streakit-api/internal/domain"
	"github.com/niteshs1/streakit-api/internal/middleware"
	"github.com/niteshs1/streakit-api/internal/response"
)

// actionFunc is the shape of every bodiless "POST /:id/<action>" service call.
type actionFunc[T any] func(ctx context.Context, actor domain.Actor, id uuid.UUID) (*T, error)

func runAction[T, R any](c *gin.Context, fn actionFunc[T], msg string, toDTO func(*T) R) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	out, err := fn(c.Request.Context(), middleware.CurrentActor(c), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, msg, toDTO(out))
}
