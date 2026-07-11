package handlers

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/zaidejaz/saaf-islamabad-backend/utils"
)

// UploadIssueImage godoc
// @Summary      Upload issue photo
// @Description  Citizen uploads a civic-issue photo. Returns a public URL for use with /classify and /reports.
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

	userID := c.MustGet("user_id").(uuid.UUID)
	key := fmt.Sprintf("issues/issue_%s_%s", userID.String(), uuid.NewString())
	publicURL, err := saveUploadedImage(c, fileHeader, key)
	if badImageUpload(c, err) {
		return
	}

	utils.Created(c, gin.H{
		"image_url":  publicURL,
		"image_urls": []string{publicURL},
	})
}
