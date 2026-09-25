package handler

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/gbplantwiki/gbplantwiki/internal/constants"
	"github.com/gbplantwiki/gbplantwiki/internal/dto"
	"github.com/gbplantwiki/gbplantwiki/internal/middleware"
	"github.com/gbplantwiki/gbplantwiki/internal/model"
	"github.com/gbplantwiki/gbplantwiki/internal/service"
	"github.com/gbplantwiki/gbplantwiki/internal/util"
)

// AnswerHandler exposes answer endpoints.
type AnswerHandler struct {
	svc    *service.AnswerService
	logger *slog.Logger
}

// NewAnswerHandler creates an AnswerHandler.
func NewAnswerHandler(svc *service.AnswerService, logger *slog.Logger) *AnswerHandler {
	return &AnswerHandler{svc: svc, logger: logger}
}

// List handles GET /questions/:questionId/answers.
func (h *AnswerHandler) List(c *gin.Context) {
	questionID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.Error(util.NewAppError(http.StatusBadRequest, constants.CodeBadRequest, "invalid question id"))
		return
	}
	items, liked, err := h.svc.ListByQuestion(middleware.GetUserID(c), uint(questionID))
	if err != nil {
		c.Error(err)
		return
	}
	resp := make([]dto.AnswerResponse, 0, len(items))
	for _, a := range items {
		resp = append(resp, toAnswerResponse(a, liked[a.ID]))
	}
	c.JSON(http.StatusOK, dto.OK(resp))
}

// Create handles POST /questions/:questionId/answers.
func (h *AnswerHandler) Create(c *gin.Context) {
	questionID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.Error(util.NewAppError(http.StatusBadRequest, constants.CodeBadRequest, "invalid question id"))
		return
	}
	var req dto.AnswerCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(util.NewAppError(http.StatusBadRequest, constants.CodeBadRequest, constants.MsgInvalidParam))
		return
	}
	a, err := h.svc.Create(middleware.GetUserID(c), uint(questionID), req.Content)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusCreated, dto.OK(toAnswerResponse(*a, false)))
}

// Adopt handles PUT /questions/:questionId/adopt.
func (h *AnswerHandler) Adopt(c *gin.Context) {
	questionID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.Error(util.NewAppError(http.StatusBadRequest, constants.CodeBadRequest, "invalid question id"))
		return
	}
	var req dto.AdoptRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(util.NewAppError(http.StatusBadRequest, constants.CodeBadRequest, constants.MsgInvalidParam))
		return
	}
	a, err := h.svc.Adopt(middleware.GetUserID(c), uint(questionID), req.AnswerID)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, dto.OK(a))
}

// Like handles PUT /answers/:id/like. It toggles the current user's support:
// the first click records one like, clicking again withdraws it.
func (h *AnswerHandler) Like(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.Error(util.NewAppError(http.StatusBadRequest, constants.CodeBadRequest, "invalid answer id"))
		return
	}
	a, liked, err := h.svc.ToggleLike(middleware.GetUserID(c), uint(id))
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, dto.OK(dto.LikeResponse{LikeCount: a.LikeCount, Liked: liked}))
}

func toAnswerResponse(a model.Answer, liked bool) dto.AnswerResponse {
	return dto.AnswerResponse{
		ID:         a.ID,
		QuestionID: a.QuestionID,
		UserID:     a.UserID,
		Content:    a.Content,
		IsBest:     a.IsBest,
		LikeCount:  a.LikeCount,
		Liked:      liked,
		CreatedAt:  a.CreatedAt,
	}
}
