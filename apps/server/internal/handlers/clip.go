package handlers

import (
	"fmt"
	"io"
	"log"
	"net/url"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"clipshare/internal/middleware"
	"clipshare/internal/models"
	"clipshare/internal/services"

	"clipshare/pkg/auth"
)

type ClipHandler struct {
	clipService     *services.ClipService
	instanceService *services.InstanceService
	jwtManager      *auth.JWTManager
}

func NewClipHandler(clipService *services.ClipService, instanceService *services.InstanceService, jwtManager *auth.JWTManager) *ClipHandler {
	return &ClipHandler{
		clipService:     clipService,
		instanceService: instanceService,
		jwtManager:      jwtManager,
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
	clips.Post("/:id/thumbnail", middleware.RequireAuth, h.UploadThumbnail)
	clips.Get("/:id/thumbnail", middleware.RequireAuth, h.GetThumbnail)
	clips.Get("/:id/download", middleware.RequireAuth, h.DownloadClip)
	clips.Get("/:id/comments", middleware.RequireAuth, h.ListClipComments)

	shares := clips.Group("/:clipId/shares")
	shares.Post("/", middleware.RequireAuth, h.CreateShare)
	shares.Get("/", middleware.RequireAuth, h.ListShares)
	shares.Delete("/:shareId", middleware.RequireAuth, h.DeleteShare)

	r.Get("/s/:code", h.GetSharedClip)
	r.Get("/s/:code/video", h.StreamSharedClip)
	r.Get("/s/:code/comments", h.ListSharedComments)
	r.Post("/s/:code/comments", h.CreateSharedComment)
}

func (h *ClipHandler) getUserID(c *fiber.Ctx) (uuid.UUID, error) {
	idStr := c.Locals("user_id")
	if idStr == nil {
		return uuid.Nil, fiber.NewError(fiber.StatusUnauthorized, "not authenticated")
	}

	switch v := idStr.(type) {
	case string:
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

	// Reject before touching object storage if the server is out of room.
	if err := h.instanceService.CheckRoomFor(c.Context(), file.Size); err != nil {
		log.Printf("[UploadFile] Rejected: %v", err)
		return c.Status(fiber.StatusInsufficientStorage).JSON(fiber.Map{
			"error": "Server storage limit reached — ask the admin to free up space or raise the limit.",
		})
	}

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
		ViewURL      string `json:"view_url"`
		ThumbnailURL string `json:"thumbnail_url,omitempty"`
	}

	clipsWithURL := make([]clipWithURL, len(resp.Clips))
	for i, clip := range resp.Clips {
		viewURL, err := h.clipService.GetClipViewURL(c.Context(), clip)
		if err != nil {
			viewURL = h.buildAbsoluteURL(c, fmt.Sprintf("/api/v1/clips/%s/download", clip.ID))
		}
		thumbnailURL := ""
		if clip.ThumbnailKey != nil && *clip.ThumbnailKey != "" {
			thumbnailURL = h.buildAbsoluteURL(c, fmt.Sprintf("/api/v1/clips/%s/thumbnail", clip.ID))
		}
		clipsWithURL[i] = clipWithURL{
			Clip:         clip,
			ViewURL:      viewURL,
			ThumbnailURL: thumbnailURL,
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

	viewURL, err := h.clipService.GetClipViewURL(c.Context(), clip)
	if err != nil {
		viewURL = h.buildAbsoluteURL(c, fmt.Sprintf("/api/v1/clips/%s/download", clipID))
	}

	thumbnailURL := ""
	if clip.ThumbnailKey != nil && *clip.ThumbnailKey != "" {
		thumbnailURL = h.buildAbsoluteURL(c, fmt.Sprintf("/api/v1/clips/%s/thumbnail", clipID))
	}

	return c.JSON(fiber.Map{
		"clip":          clip,
		"view_url":      viewURL,
		"thumbnail_url": thumbnailURL,
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

	result := make([]fiber.Map, len(shares))
	for i, s := range shares {
		result[i] = fiber.Map{
			"id":           s.ID,
			"clip_id":      s.ClipID,
			"user_id":      s.UserID,
			"share_code":   s.ShareCode,
			"custom_slug":  s.CustomSlug,
			"has_password": s.HasPassword,
			"expires_at":   s.ExpiresAt,
			"max_views":    s.MaxViews,
			"view_count":   s.ViewCount,
			"is_active":    s.IsActive,
			"created_at":   s.CreatedAt,
			"share_url":    h.clipService.ShareURL(s.ShareCode, s.CustomSlug),
		}
	}

	return c.JSON(fiber.Map{
		"shares": result,
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

	clip, err := h.clipService.GetSharedClip(c.Context(), code, password)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	title := clip.Title
	if title == "" {
		title = "Shared Clip"
	}

	videoURL := h.buildAbsoluteURL(c, fmt.Sprintf("/api/v1/s/%s/video", code))
	if password != nil {
		videoURL += "?password=" + url.QueryEscape(*password)
	}

	contentType := services.ClipContentType(clip.OriginalFilename)

	passwordJS := "null"
	if password != nil {
		passwordJS = fmt.Sprintf("%q", *password)
	}
	commentsEnabled := "false"
	if clip.AllowComments {
		commentsEnabled = "true"
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
      padding: 24px;
    }
    .container { width: 100%%; max-width: 960px; margin: 0 auto; }
    h1 { font-size: 1.25rem; font-weight: 600; margin-bottom: 16px; }
    h2 { font-size: 1rem; font-weight: 600; margin: 24px 0 12px; color: #c0c0c0; }
    video {
      width: 100%%;
      border-radius: 8px;
      background: #000;
      display: block;
    }
    .meta { margin-top: 12px; font-size: 0.8rem; color: #777; }
    .comments { margin-top: 24px; }
    .comment {
      background: #1a1a1a;
      border-radius: 6px;
      padding: 10px 12px;
      margin-bottom: 8px;
    }
    .comment-head {
      display: flex;
      justify-content: space-between;
      font-size: 0.8rem;
      margin-bottom: 4px;
    }
    .comment-name { font-weight: 600; color: #e0e0e0; }
    .comment-time { color: #777; }
    .comment-body { color: #d0d0d0; white-space: pre-wrap; word-break: break-word; }
    .empty { color: #777; font-size: 0.9rem; font-style: italic; }
    form.new-comment {
      background: #1a1a1a;
      border-radius: 6px;
      padding: 12px;
      margin-top: 12px;
    }
    form.new-comment input, form.new-comment textarea {
      width: 100%%;
      background: #0f0f0f;
      color: #e0e0e0;
      border: 1px solid #333;
      border-radius: 4px;
      padding: 8px 10px;
      font: inherit;
      margin-bottom: 8px;
    }
    form.new-comment textarea { min-height: 72px; resize: vertical; }
    form.new-comment button {
      background: #3a7a3a;
      color: #fff;
      border: 0;
      border-radius: 4px;
      padding: 8px 14px;
      font: inherit;
      font-weight: 600;
      cursor: pointer;
    }
    form.new-comment button:disabled { opacity: 0.6; cursor: not-allowed; }
    .error { color: #e07676; font-size: 0.85rem; margin-top: 4px; }
  </style>
</head>
<body>
  <div class="container">
    <h1>%s</h1>
    <video controls autoplay preload="metadata">
      <source src="%s" type="%s">
      Your browser does not support the video tag.
    </video>
    <p class="meta">Shared via clipshare</p>
    <div id="comments-section" class="comments" style="display:none">
      <h2>Comments</h2>
      <div id="comments-list"><p class="empty">Loading…</p></div>
      <form id="comment-form" class="new-comment">
        <input id="comment-name" type="text" maxlength="64" placeholder="Your name" required>
        <textarea id="comment-content" maxlength="2000" placeholder="Leave a comment…" required></textarea>
        <button type="submit">Post comment</button>
        <div id="comment-error" class="error" style="display:none"></div>
      </form>
    </div>
  </div>
  <script>
    (function () {
      var commentsEnabled = %s;
      if (!commentsEnabled) return;
      var code = %q;
      var password = %s;
      var section = document.getElementById('comments-section');
      var list = document.getElementById('comments-list');
      var form = document.getElementById('comment-form');
      var nameInput = document.getElementById('comment-name');
      var contentInput = document.getElementById('comment-content');
      var errorBox = document.getElementById('comment-error');
      var submitBtn = form.querySelector('button');
      section.style.display = '';

      var storedName = null;
      try { storedName = localStorage.getItem('clipshare:comment-name'); } catch (e) {}
      if (storedName) nameInput.value = storedName;

      function escapeHTML(s) {
        return String(s).replace(/[&<>"']/g, function (c) {
          return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c];
        });
      }

      function formatDate(iso) {
        try { return new Date(iso).toLocaleString(); } catch (e) { return ''; }
      }

      function renderComments(items) {
        if (!items.length) {
          list.innerHTML = '<p class="empty">No comments yet — be the first.</p>';
          return;
        }
        list.innerHTML = items.map(function (c) {
          var name = c.display_name || 'Guest';
          return '<div class="comment">' +
            '<div class="comment-head">' +
              '<span class="comment-name">' + escapeHTML(name) + '</span>' +
              '<span class="comment-time">' + escapeHTML(formatDate(c.created_at)) + '</span>' +
            '</div>' +
            '<div class="comment-body">' + escapeHTML(c.content) + '</div>' +
          '</div>';
        }).join('');
      }

      function qs() {
        return password ? ('?password=' + encodeURIComponent(password)) : '';
      }

      function loadComments() {
        fetch('/api/v1/s/' + encodeURIComponent(code) + '/comments' + qs())
          .then(function (r) { return r.json().then(function (j) { return { ok: r.ok, body: j }; }); })
          .then(function (res) {
            if (!res.ok) {
              list.innerHTML = '<p class="empty">Could not load comments.</p>';
              return;
            }
            renderComments(res.body.comments || []);
          })
          .catch(function () {
            list.innerHTML = '<p class="empty">Could not load comments.</p>';
          });
      }

      form.addEventListener('submit', function (e) {
        e.preventDefault();
        errorBox.style.display = 'none';
        var name = nameInput.value.trim();
        var content = contentInput.value.trim();
        if (!name || !content) return;

        submitBtn.disabled = true;
        try { localStorage.setItem('clipshare:comment-name', name); } catch (e2) {}

        fetch('/api/v1/s/' + encodeURIComponent(code) + '/comments', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ name: name, content: content, password: password })
        })
          .then(function (r) { return r.json().then(function (j) { return { ok: r.ok, body: j }; }); })
          .then(function (res) {
            if (!res.ok) {
              errorBox.textContent = (res.body && res.body.error) || 'Could not post comment.';
              errorBox.style.display = '';
              return;
            }
            contentInput.value = '';
            loadComments();
          })
          .catch(function () {
            errorBox.textContent = 'Network error. Please try again.';
            errorBox.style.display = '';
          })
          .finally(function () { submitBtn.disabled = false; });
      });

      loadComments();
    })();
  </script>
</body>
</html>`, title, title, videoURL, contentType, commentsEnabled, code, passwordJS)

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

	_, clip, err := h.clipService.ValidateShareAccess(c.Context(), code, password)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	size := clip.FileSizeBytes
	contentType := services.ClipContentType(clip.OriginalFilename)

	c.Set("Content-Type", contentType)
	c.Set("Accept-Ranges", "bytes")

	start, end, isRange := resolveRange(c.Get("Range"), size)
	if start < 0 {
		return c.Status(fiber.StatusRequestedRangeNotSatisfiable).JSON(fiber.Map{
			"error": "invalid range",
		})
	}

	reader, err := h.clipService.StreamClipFileRange(c.Context(), clip, start, end)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	contentLength := end - start + 1
	c.Set("Content-Length", strconv.FormatInt(contentLength, 10))
	if isRange {
		c.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, size))
		c.Status(fiber.StatusPartialContent)
	} else {
		c.Status(fiber.StatusOK)
	}
	c.Response().SetBodyStream(&limitReadCloser{
		Reader: io.LimitReader(reader, contentLength),
		Closer: reader,
	}, int(contentLength))
	return nil
}

func (h *ClipHandler) ListClipComments(c *fiber.Ctx) error {
	userID, err := h.getUserID(c)
	if err != nil {
		return err
	}
	clipIDStr := c.Params("id")
	clipID, err := uuid.Parse(clipIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid clip ID"})
	}
	comments, err := h.clipService.ListClipCommentsForOwner(c.Context(), clipID, userID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"comments": comments})
}

func (h *ClipHandler) ListSharedComments(c *fiber.Ctx) error {
	code := c.Params("code")
	if code == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Share code is required"})
	}
	var password *string
	if pw := c.Query("password"); pw != "" {
		password = &pw
	}
	comments, err := h.clipService.ListShareComments(c.Context(), code, password)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"comments": comments})
}

func (h *ClipHandler) CreateSharedComment(c *fiber.Ctx) error {
	code := c.Params("code")
	if code == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Share code is required"})
	}

	var req struct {
		Name     string  `json:"name"`
		Content  string  `json:"content"`
		Password *string `json:"password,omitempty"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	comment, err := h.clipService.CreateGuestComment(c.Context(), code, req.Password, req.Name, req.Content)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"comment": comment})
}

func (h *ClipHandler) UploadThumbnail(c *fiber.Ctx) error {
	userID, err := h.getUserID(c)
	if err != nil {
		return err
	}

	clipIDStr := c.Params("id")
	clipID, err := uuid.Parse(clipIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid clip ID"})
	}

	file, err := c.FormFile("thumbnail")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "thumbnail field is required"})
	}

	f, err := file.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to open file"})
	}
	defer f.Close()

	contentType := file.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg"
	}

	if err := h.clipService.UploadThumbnail(c.Context(), clipID, userID, f, file.Size, contentType); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"success": true})
}

func (h *ClipHandler) GetThumbnail(c *fiber.Ctx) error {
	clipIDStr := c.Params("id")
	clipID, err := uuid.Parse(clipIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid clip ID"})
	}

	obj, size, contentType, err := h.clipService.StreamThumbnail(c.Context(), clipID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
	}
	// No defer close — fasthttp calls Close() on the stream after SetBodyStream finishes.

	c.Set("Content-Type", contentType)
	c.Set("Content-Length", strconv.FormatInt(size, 10))
	c.Set("Cache-Control", "public, max-age=3600")
	c.Response().SetBodyStream(obj, int(size))
	return nil
}

// limitReadCloser limits reads to n bytes and closes the underlying ReadCloser
// when fasthttp is done with the stream (fasthttp calls Close() if present).
type limitReadCloser struct {
	io.Reader
	io.Closer
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

// maxRangeChunkBytes caps a single Range response. Chromium-based players
// (Chrome, Edge, WebView2) stop reading once their media buffer is full, which
// stalls an open-ended response and wedges the MinIO stream + fasthttp goroutine
// waiting on TCP backpressure. When they later seek, reusing the connection can
// block on that stalled response. Capping each range response to a small chunk
// keeps every response short-lived: the player consumes it, asks for the next
// chunk, and the server never holds a long-running stream open. Firefox keeps
// working — it just makes a few extra 206 requests to assemble the full clip.
const maxRangeChunkBytes = 4 * 1024 * 1024

// resolveRange parses a Range header and caps the response to maxRangeChunkBytes.
// Returns (start, end, isRange). On invalid range, returns start == -1.
// When there is no Range header, the full file is served (start=0, end=size-1)
// so non-browser clients (curl, download tools) still get a complete file.
func resolveRange(rangeHeader string, size int64) (start, end int64, isRange bool) {
	if rangeHeader == "" {
		return 0, size - 1, false
	}
	s, e, err := parseRange(rangeHeader, size)
	if err != nil {
		return -1, -1, true
	}
	if e-s+1 > maxRangeChunkBytes {
		e = s + maxRangeChunkBytes - 1
	}
	return s, e, true
}

func parseRange(rangeHeader string, size int64) (start, end int64, err error) {
	if !strings.HasPrefix(rangeHeader, "bytes=") {
		return 0, 0, fmt.Errorf("invalid range format")
	}
	spec := strings.TrimPrefix(rangeHeader, "bytes=")
	parts := strings.Split(spec, "-")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid range format")
	}
	if parts[0] == "" {
		end = size - 1
		start, err = strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return 0, 0, err
		}
		start = size - start
		if start < 0 {
			start = 0
		}
	} else {
		start, err = strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return 0, 0, err
		}
		if parts[1] == "" {
			end = size - 1
		} else {
			end, err = strconv.ParseInt(parts[1], 10, 64)
			if err != nil {
				return 0, 0, err
			}
		}
	}
	if start > end || start >= size {
		return 0, 0, fmt.Errorf("range out of bounds")
	}
	if end >= size {
		end = size - 1
	}
	return start, end, nil
}

func (h *ClipHandler) buildAbsoluteURL(c *fiber.Ctx, path string) string {
	scheme := "http"
	if c.Protocol() == "https" {
		scheme = "https"
	}
	return scheme + "://" + c.Hostname() + path
}

func (h *ClipHandler) DownloadClip(c *fiber.Ctx) error {
	clipIDStr := c.Params("id")
	clipID, err := uuid.Parse(clipIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid clip ID",
		})
	}

	clip, err := h.clipService.GetClip(c.Context(), clipID)
	if err != nil || clip == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Clip not found",
		})
	}

	userID, _ := h.getUserID(c)
	if !clip.IsPublic && clip.UserID != userID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Not authorized to download this clip",
		})
	}

	size := clip.FileSizeBytes
	contentType := services.ClipContentType(clip.OriginalFilename)

	c.Set("Content-Type", contentType)
	c.Set("Accept-Ranges", "bytes")

	start, end, isRange := resolveRange(c.Get("Range"), size)
	if start < 0 {
		return c.Status(fiber.StatusRequestedRangeNotSatisfiable).JSON(fiber.Map{
			"error": "invalid range",
		})
	}

	// Use native MinIO range request — no full-file download, no seek.
	reader, err := h.clipService.StreamClipFileRange(c.Context(), clip, start, end)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	// No defer close — fasthttp calls Close() on the stream after SetBodyStream finishes.

	contentLength := end - start + 1
	c.Set("Content-Length", strconv.FormatInt(contentLength, 10))
	if isRange {
		c.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, size))
		c.Status(fiber.StatusPartialContent)
	} else {
		c.Status(fiber.StatusOK)
	}
	c.Response().SetBodyStream(&limitReadCloser{
		Reader: io.LimitReader(reader, contentLength),
		Closer: reader,
	}, int(contentLength))
	return nil
}
