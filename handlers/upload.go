package handlers

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/zaidejaz/saaf-islamabad-backend/utils"
)

// UploadIssueImage godoc
// @Summary      Upload issue photo
// @Description  Citizen uploads a civic-issue photo. Returns a public /uploads/ URL for use with /classify and /reports.
// @Tags         Uploads
// @Accept       multipart/form-data
// @Produce      json
// @Security     BearerAuth
// @Param        image  formData  file  true  "Issue photo (jpg/png/webp/gif, max 10MB)"
// @Success      201    {object}  utils.APIResponse
// @Failure      400    {object}  utils.APIResponse
// @Router       /uploads [post]
func UploadIssueImage(c *gin.Context) {
	fileHeader, err := c.FormFile("image")
	if err != nil {
		utils.BadRequest(c, "image file is required (form field: image)")
		return
	}
	if fileHeader.Size > 10*1024*1024 {
		utils.BadRequest(c, "image must be 10MB or smaller")
		return
	}

	contentType := fileHeader.Header.Get("Content-Type")
	ext, ok := allowedImageMIME[strings.ToLower(contentType)]
	if !ok {
		fallbackExt := strings.ToLower(filepath.Ext(fileHeader.Filename))
		switch fallbackExt {
		case ".jpg", ".jpeg":
			ext = ".jpg"
		case ".png":
			ext = ".png"
		case ".webp":
			ext = ".webp"
		case ".gif":
			ext = ".gif"
		default:
			utils.BadRequest(c, "unsupported image type; allowed: jpg, png, webp, gif")
			return
		}
	}

	if err := os.MkdirAll(uploadsDir, 0o755); err != nil {
		utils.InternalError(c, "failed to prepare upload directory")
		return
	}

	userID := c.MustGet("user_id").(uuid.UUID)
	filename := fmt.Sprintf("issue_%s_%s%s", userID.String(), uuid.NewString(), ext)
	dst := filepath.Join(uploadsDir, filename)
	if err := c.SaveUploadedFile(fileHeader, dst); err != nil {
		utils.InternalError(c, "failed to save uploaded file")
		return
	}

	publicURL := "/uploads/" + filename
	utils.Created(c, gin.H{
		"image_url":  publicURL,
		"image_urls": []string{publicURL},
	})
}
