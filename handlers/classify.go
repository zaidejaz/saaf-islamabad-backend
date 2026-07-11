package handlers

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zaidejaz/saaf-islamabad-backend/database"
	"github.com/zaidejaz/saaf-islamabad-backend/models"
	"github.com/zaidejaz/saaf-islamabad-backend/services/ai"
	"github.com/zaidejaz/saaf-islamabad-backend/utils"
)

var (
	classifier ai.Classifier
	aiTimeout  = 8 * time.Second
)

// InitAI wires the swappable image classifier used by /classify and
// auto-classify on report creation. Safe to call with a nil classifier
// when AI is disabled — endpoints return a clear error.
func InitAI(c ai.Classifier, timeoutSecs int) {
	classifier = c
	if timeoutSecs > 0 {
		aiTimeout = time.Duration(timeoutSecs) * time.Second
	}
	if c != nil {
		log.Printf("ai classifier ready: provider=%s model=%s timeout=%s", c.Name(), c.Model(), aiTimeout)
	} else {
		log.Println("ai classifier unavailable — set GROQ_API_KEY to enable /classify")
	}
}

// ClassifyIssue godoc
// @Summary      Classify civic issue image
// @Description  Validates the photo and returns AI-generated category, title, description, severity, priority, and confidence. Does not create a report.
// @Tags         AI
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      ClassifyRequest  true  "Image URLs and optional location"
// @Success      200   {object}  utils.APIResponse{data=ai.ClassifyResult}
// @Failure      400   {object}  utils.APIResponse
// @Failure      503   {object}  utils.APIResponse
// @Router       /classify [post]
func ClassifyIssue(c *gin.Context) {
	var req ClassifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if len(req.ImageURLs) == 0 {
		utils.BadRequest(c, "image_urls is required")
		return
	}

	result, err := runClassification(c.Request.Context(), req.ImageURLs, &req.Latitude, &req.Longitude, req.Address)
	if err != nil {
		writeClassifyError(c, err)
		return
	}
	enrichClassificationResult(result)
	utils.OK(c, result)
}

func runClassification(ctx context.Context, imageURLs []string, lat, lng *float64, address string) (*ai.ClassifyResult, error) {
	if classifier == nil {
		return nil, ai.ErrNotConfigured
	}

	var categories []models.IssueCategory
	if err := database.DB.Order("name ASC").Find(&categories).Error; err != nil {
		return nil, err
	}
	if len(categories) == 0 {
		return nil, errors.New("no issue categories configured")
	}

	opts := make([]ai.CategoryOption, 0, len(categories))
	for _, cat := range categories {
		opts = append(opts, ai.CategoryOption{ID: cat.ID, Name: cat.Name})
	}

	classifyReq := ai.ClassifyRequest{
		ImageURLs:  imageURLs,
		Address:    address,
		Categories: opts,
	}
	if lat != nil {
		classifyReq.Latitude = lat
	}
	if lng != nil {
		classifyReq.Longitude = lng
	}

	ctx, cancel := context.WithTimeout(ctx, aiTimeout)
	defer cancel()

	return classifier.Classify(ctx, classifyReq)
}

func writeClassifyError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ai.ErrNotConfigured), errors.Is(err, ai.ErrDisabled):
		utils.Error(c, http.StatusServiceUnavailable, "AI classification is not configured. Set GROQ_API_KEY and AI_ENABLED=true.")
	case errors.Is(err, ai.ErrNoImages):
		utils.BadRequest(c, err.Error())
	case errors.Is(err, context.DeadlineExceeded):
		utils.Error(c, http.StatusGatewayTimeout, "AI classification timed out")
	default:
		log.Printf("classify failed: %v", err)
		utils.InternalError(c, "AI classification failed")
	}
}
