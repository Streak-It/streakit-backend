package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/niteshs1/streakit-api/internal/dto"
	"github.com/niteshs1/streakit-api/internal/response"
	"github.com/niteshs1/streakit-api/internal/service"
)

type AuditHandler struct {
	svc service.AuditService
}

func NewAuditHandler(svc service.AuditService) *AuditHandler {
	return &AuditHandler{svc: svc}
}

// List
//
//	@Summary		Read the audit log
//	@Description	Newest first by default (`sort=-created_at`); use `sort=created_at` for oldest first. `from`/`to` are RFC 3339.
//	@Tags			audit
//	@Produce		json
//	@Security		CookieAuth
//	@Param			query	query		dto.AuditListQuery	false	"Filters"
//	@Success		200		{object}	response.Success{data=response.Page[dto.AuditEntryResponse]}
//	@Failure		422		{object}	response.Failure
//	@Router			/audit [get]
func (h *AuditHandler) List(c *gin.Context) {
	var q dto.AuditListQuery
	if !bindQuery(c, &q) {
		return
	}
	page, err := h.svc.List(c.Request.Context(), q.ToFilter())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, "OK", dto.ToAuditPage(page))
}
