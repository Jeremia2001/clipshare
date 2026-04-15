package handlers

import (
	"fmt"
	"io"
	"log"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"clipshare/internal/middleware"
	"clipshare/internal/models"
	"clipshare/internal/services"

	"clipshare/pkg/auth"
)

type ClipHandler struct {
	clipService *services.ClipService
	jwtManager  *auth.JWTManager
}

func NewClipHandler(clipService *services.ClipService, jwtManager *auth.JWTManager) *ClipHandler {
	return &ClipHandler{
		clipService: clipService,
		jwtManager:  jwtManager,
	}
}

func (h *ClipHandler) RegisterRoutes(r fiber.Router) {
	clips := r.Group("/clips")

	clips.Get("/", middleware.RequireAuth, h.ListClips)
	clips.Get("/:id", middleware.RequireAuth, h.GetClip)
	clips.Put("/:id", middleware.RequireAuth, h.UpdateClip)
	clips.Delete("/:id", middleware.RequireAuth, h.DeleteClip)

	clips.Post("/upload", middleware.RequireAuth, h.UploadFile)
	clips.Post("/:id/finalize", middleware.RequireAuth, h.FinalizeUpload)
	clips.Get("/:id/download", middleware.RequireAuth, h.DownloadClip)

	shares := clips.Group("/:clipId/shares")
	shares.Post("/", middleware.RequireAuth, h.CreateShare)
	shares.Get("/", middleware.RequireAuth, h.ListShares)
	shares.Delete("/:shareId", middleware.RequireAuth, h.DeleteShare)

	r.Get("/s/:code", h.GetSharedClip)
	r.Get("/s/:code/video", h.StreamSharedClip)
}

func (h *ClipHandler) getUserID(c *fiber.Ctx) (uuid.UUID, error) {
	idStr := c.Locals("user_id")
	if idStr == nil {
		return uuid.Nil, fiber.NewError(fiber.StatusUnauthorized, "not authenticated")
	}

	switch v := idStr.(type) {
	case string:
		if v == "dev-user-id" {
			devUUID, _ := uuid.Parse("00000000-0000-0000-0000-000000000001")
			return devUUID, nil
		}
		return uuid.Parse(v)
	case uuid.UUID:
		return v, nil
	default:
		return uuid.Nil, fiber.NewError(fiber.StatusUnauthorized, "invalid user id")
	}
}

func (h *ClipHandler) UploadFile(c *fiber.Ctx) error {
	userID, err := h.getUserID(c)
	if err != nil {
		return err
	}

	file, err := c.FormFile("file")
	if err != nil {
		log.Printf("[UploadFile] No file in form: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "File is required",
		})
	}

	log.Printf("[UploadFile] Received file: %s (%d bytes, content-type: %s)", file.Filename, file.Size, file.Header.Get("Content-Type"))

	f, err := file.Open()
	if err != nil {
		log.Printf("[UploadFile] Failed to open file: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to open file",
		})
	}
	defer f.Close()

	objectKey := fmt.Sprintf("uploads/%s/%s", userID.String(), uuid.New().String())

	contentType := file.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "video/mp4"
	}

	log.Printf("[UploadFile] Uploading to RustFS: bucket=clips-raw, key=%s, size=%d, contentType=%s", objectKey, file.Size, contentType)

	if err := h.clipService.UploadToStorage(c.Context(), objectKey, f, file.Size, contentType); err != nil {
		log.Printf("[UploadFile] RustFS upload failed: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Upload failed: " + err.Error(),
		})
	}

	log.Printf("[UploadFile] RustFS upload successful, creating clip record")

	clip := &models.Clip{
		UserID:           userID,
		Title:            file.Filename,
		RustfsBucket:     "clips-raw",
		RustfsObjectKey:  objectKey,
		OriginalFilename: file.Filename,
		FileSizeBytes:    file.Size,
		DurationSeconds:  0,
		Width:            0,
		Height:           0,
		IsPublic:         true,
		AllowComments:    true,
	}

	created, err := h.clipService.CreateClipRecord(c.Context(), clip)
	if err != nil {
		log.Printf("[UploadFile] Failed to create clip record: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create clip: " + err.Error(),
		})
	}

	log.Printf("[UploadFile] Clip created successfully: id=%s", created.ID)

	return c.JSON(fiber.Map{
		"clip":       created,
		"object_key": objectKey,
	})
}

func (h *ClipHandler) FinalizeUpload(c *fiber.Ctx) error {
	userID, err := h.getUserID(c)
	if err != nil {
		return err
	}

	clipIDStr := c.Params("id")
	clipID, err := uuid.Parse(clipIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid clip ID",
		})
	}

	var req services.CreateClipRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	clip, err := h.clipService.FinalizeUpload(c.Context(), clipID, userID, req)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(clip)
}

func (h *ClipHandler) ListClips(c *fiber.Ctx) error {
	userID, err := h.getUserID(c)
	if err != nil {
		return err
	}

	page, _ := strconv.Atoi(c.Query("page", "1"))
	perPage, _ := strconv.Atoi(c.Query("per_page", "20"))

	resp, err := h.clipService.ListUserClips(c.Context(), userID, page, perPage)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	type clipWithURL struct {
		*models.Clip
		ViewURL string `json:"view_url"`
	}

	clipsWithURL := make([]clipWithURL, len(resp.Clips))
	for i, clip := range resp.Clips {
		clipsWithURL[i] = clipWithURL{
			Clip:    clip,
			ViewURL: fmt.Sprintf("/api/v1/clips/%s/download", clip.ID),
		}
	}

	return c.JSON(fiber.Map{
		"clips":    clipsWithURL,
		"total":    resp.Total,
		"page":     resp.Page,
		"per_page": resp.PerPage,
	})
}

func (h *ClipHandler) GetClip(c *fiber.Ctx) error {
	clipIDStr := c.Params("id")
	clipID, err := uuid.Parse(clipIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid clip ID",
		})
	}

	clip, err := h.clipService.GetClip(c.Context(), clipID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	if clip == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Clip not found",
		})
	}

	userID, _ := h.getUserID(c)
	if !clip.IsPublic && clip.UserID != userID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Not authorized to view this clip",
		})
	}

	return c.JSON(fiber.Map{
		"clip":     clip,
		"view_url": fmt.Sprintf("/api/v1/clips/%s/download", clipID),
	})
}

func (h *ClipHandler) UpdateClip(c *fiber.Ctx) error {
	userID, err := h.getUserID(c)
	if err != nil {
		return err
	}

	clipIDStr := c.Params("id")
	clipID, err := uuid.Parse(clipIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid clip ID",
		})
	}

	var req services.UpdateClipRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	clip, err := h.clipService.UpdateClip(c.Context(), clipID, userID, req)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(clip)
}

func (h *ClipHandler) DeleteClip(c *fiber.Ctx) error {
	userID, err := h.getUserID(c)
	if err != nil {
		return err
	}

	clipIDStr := c.Params("id")
	clipID, err := uuid.Parse(clipIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid clip ID",
		})
	}

	if err := h.clipService.DeleteClip(c.Context(), clipID, userID); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
	})
}

func (h *ClipHandler) CreateShare(c *fiber.Ctx) error {
	userID, err := h.getUserID(c)
	if err != nil {
		return err
	}

	clipIDStr := c.Params("clipId")
	clipID, err := uuid.Parse(clipIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid clip ID",
		})
	}

	var req services.CreateShareRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	req.ClipID = clipID

	resp, err := h.clipService.CreateShare(c.Context(), userID, req)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	enrichShare(resp.Share)

	return c.JSON(resp)
}

func (h *ClipHandler) ListShares(c *fiber.Ctx) error {
	userID, err := h.getUserID(c)
	if err != nil {
		return err
	}

	clipIDStr := c.Params("clipId")
	clipID, err := uuid.Parse(clipIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid clip ID",
		})
	}

	shares, err := h.clipService.ListClipShares(c.Context(), clipID, userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	enrichShares(shares)

	return c.JSON(fiber.Map{
		"shares": shares,
	})
}

func (h *ClipHandler) DeleteShare(c *fiber.Ctx) error {
	userID, err := h.getUserID(c)
	if err != nil {
		return err
	}

	shareIDStr := c.Params("shareId")
	shareID, err := uuid.Parse(shareIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid share ID",
		})
	}

	if err := h.clipService.DeleteShare(c.Context(), shareID, userID); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
	})
}

func (h *ClipHandler) GetSharedClip(c *fiber.Ctx) error {
	code := c.Params("code")
	if code == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Share code is required",
		})
	}

	var password *string
	if pw := c.Query("password"); pw != "" {
		password = &pw
	}

	clip, _, err := h.clipService.GetSharedClip(c.Context(), code, password)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	title := clip.Title
	if title == "" {
		title = "Shared Clip"
	}

	videoURL := fmt.Sprintf("/api/v1/s/%s/video", code)
	if pw := c.Query("password"); pw != "" {
		videoURL += "?password=" + pw
	}

	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>%s</title>
  <style>
    *, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
    body {
      background: #0f0f0f;
      color: #e0e0e0;
      font-family: system-ui, sans-serif;
      min-height: 100vh;
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      padding: 24px;
    }
    .container { width: 100%%; max-width: 960px; }
    h1 { font-size: 1.25rem; font-weight: 600; margin-bottom: 16px; }
    video {
      width: 100%%;
      border-radius: 8px;
      background: #000;
      display: block;
    }
    .meta { margin-top: 12px; font-size: 0.8rem; color: #777; }
  </style>
</head>
<body>
  <div class="container">
    <h1>%s</h1>
    <video controls autoplay preload="metadata">
      <source src="%s">
      Your browser does not support the video tag.
    </video>
    <p class="meta">Shared via clipshare</p>
  </div>
</body>
</html>`, title, title, videoURL)

	c.Set("Content-Type", "text/html; charset=utf-8")
	return c.SendString(html)
}

func (h *ClipHandler) StreamSharedClip(c *fiber.Ctx) error {
	code := c.Params("code")
	if code == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Share code is required",
		})
	}

	var password *string
	if pw := c.Query("password"); pw != "" {
		password = &pw
	}

	clip, _, err := h.clipService.GetSharedClip(c.Context(), code, password)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	reader, size, contentType, err := h.clipService.StreamClipFile(c.Context(), clip.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	defer reader.Close()

	c.Set("Content-Type", contentType)
	c.Set("Content-Length", strconv.FormatInt(size, 10))
	c.Set("Accept-Ranges", "bytes")

	_, err = io.Copy(c.Response().BodyWriter(), reader)
	return err
}

func enrichShare(s *models.Share) {
	if s != nil {
		s.HasPassword = s.PasswordHash != nil && *s.PasswordHash != ""
	}
}

func enrichShares(shares []*models.Share) {
	for _, s := range shares {
		enrichShare(s)
	}
}

func (h *ClipHandler) DownloadClip(c *fiber.Ctx) error {
	clipIDStr := c.Params("id")
	clipID, err := uuid.Parse(clipIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid clip ID",
		})
	}

	clip, size, contentType, err := h.clipService.StreamClipFile(c.Context(), clipID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	defer clip.Close()

	c.Set("Content-Type", contentType)
	c.Set("Content-Length", strconv.FormatInt(size, 10))

	_, err = io.Copy(c.Response().BodyWriter(), clip)
	return err
}
