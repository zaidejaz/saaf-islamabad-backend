package handlers

import (
	"fmt"
	"mime/multipart"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/zaidejaz/saaf-islamabad-backend/services/storage"
	"github.com/zaidejaz/saaf-islamabad-backend/utils"
)

var imageUploader storage.Uploader

func InitStorage(uploader storage.Uploader) {
	imageUploader = uploader
}

func storageDriver() string {
	if imageUploader == nil {
		return storage.DriverLocal
	}
	return imageUploader.Driver()
}

func parseImageUpload(fileHeader *multipart.FileHeader) (ext, contentType string, err error) {
	if fileHeader.Size > 10*1024*1024 {
		return "", "", fmt.Errorf("image must be 10MB or smaller")
	}

	contentType = strings.ToLower(fileHeader.Header.Get("Content-Type"))
	ext, ok := allowedImageMIME[contentType]
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
			return "", "", fmt.Errorf("unsupported image type; allowed: jpg, png, webp, gif")
		}
		if contentType == "" {
			switch ext {
			case ".jpg":
				contentType = "image/jpeg"
			case ".png":
				contentType = "image/png"
			case ".webp":
				contentType = "image/webp"
			case ".gif":
				contentType = "image/gif"
			}
		}
	}
	return ext, contentType, nil
}

func saveUploadedImage(c *gin.Context, fileHeader *multipart.FileHeader, key string) (string, error) {
	if imageUploader == nil {
		return "", fmt.Errorf("image storage is not configured")
	}

	ext, contentType, err := parseImageUpload(fileHeader)
	if err != nil {
		return "", err
	}

	file, err := fileHeader.Open()
	if err != nil {
		return "", fmt.Errorf("open uploaded file")
	}
	defer file.Close()

	return imageUploader.Save(key+ext, file, contentType, fileHeader.Size)
}

func badImageUpload(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "10MB"):
		utils.BadRequest(c, msg)
	case strings.Contains(msg, "unsupported image"):
		utils.BadRequest(c, msg)
	default:
		utils.InternalError(c, "failed to save uploaded file")
	}
	return true
}
