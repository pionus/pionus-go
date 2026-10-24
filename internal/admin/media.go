package admin

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pionus/arry"
	"github.com/pionus/pionus-go/internal/model"
	"github.com/pionus/pionus-go/internal/store"
)

func (a *Admin) mediaList(ctx arry.Context) {
	a.renderMedia(ctx, http.StatusOK, "")
}

func (a *Admin) renderMedia(ctx arry.Context, code int, errMsg string) {
	page, limit := pageOf(ctx, 24)
	items, total, err := a.store.ListMedia(page, limit)
	if err != nil {
		logErr("admin list media", "err", err)
		ctx.Reply(http.StatusInternalServerError)
		return
	}

	totalPages := (total + limit - 1) / limit
	if totalPages < 1 {
		totalPages = 1
	}

	data := a.base(ctx, "媒体库")
	data["Media"] = items
	data["Total"] = total
	data["Page"] = page
	data["TotalPages"] = totalPages
	data["HasPrev"] = page > 1
	data["HasNext"] = page < totalPages
	data["PrevPage"] = page - 1
	data["NextPage"] = page + 1
	data["Error"] = errMsg
	a.render(ctx, code, "media.html", data)
}

func (a *Admin) mediaUpload(ctx arry.Context) {
	file, header, err := ctx.Request().FormFile("file")
	if err != nil {
		a.renderMedia(ctx, http.StatusBadRequest, "请选择要上传的文件。")
		return
	}
	defer file.Close()

	if header.Size > a.maxUpload {
		a.renderMedia(ctx, http.StatusBadRequest, fmt.Sprintf("文件不能超过 %d MB。", a.maxUpload>>20))
		return
	}

	data, err := io.ReadAll(io.LimitReader(file, a.maxUpload+1))
	if err != nil {
		a.renderMedia(ctx, http.StatusBadRequest, "读取文件失败。")
		return
	}
	if int64(len(data)) > a.maxUpload {
		a.renderMedia(ctx, http.StatusBadRequest, fmt.Sprintf("文件不能超过 %d MB。", a.maxUpload>>20))
		return
	}

	mimeType := http.DetectContentType(data)
	ext, ok := uploadExtension(mimeType, header.Filename)
	if !ok {
		a.renderMedia(ctx, http.StatusBadRequest, "不支持的文件类型，仅允许图片、PDF、ZIP 和文本。")
		return
	}

	rel := time.Now().Format("2006/01") + "/" + randomName() + ext
	full := filepath.Join(a.uploadDir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		logErr("create upload dir", "err", err)
		a.renderMedia(ctx, http.StatusInternalServerError, "创建上传目录失败。")
		return
	}
	if err := os.WriteFile(full, data, 0644); err != nil {
		logErr("write upload", "err", err)
		a.renderMedia(ctx, http.StatusInternalServerError, "保存文件失败。")
		return
	}

	m := &model.Media{
		Path:     rel,
		Filename: filepath.Base(header.Filename),
		MIME:     mimeType,
		Size:     int64(len(data)),
	}
	if cfg, _, err := image.DecodeConfig(bytes.NewReader(data)); err == nil {
		m.Width = cfg.Width
		m.Height = cfg.Height
	}

	if err := a.store.CreateMedia(m); err != nil {
		os.Remove(full)
		logErr("create media", "err", err)
		a.renderMedia(ctx, http.StatusInternalServerError, "保存文件记录失败。")
		return
	}

	a.redirect(ctx, "/admin/media?uploaded=1")
}

func (a *Admin) mediaDelete(ctx arry.Context) {
	id := atoiDefault(ctx.Request().PostFormValue("id"), 0)
	if id <= 0 {
		ctx.Reply(http.StatusBadRequest)
		return
	}

	m, err := a.store.GetMedia(int64(id))
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			logErr("admin load media", "id", id, "err", err)
		}
		a.redirect(ctx, "/admin/media?deleted=1")
		return
	}

	full := filepath.Join(a.uploadDir, filepath.FromSlash(m.Path))
	if withinDir(a.uploadDir, full) {
		_ = os.Remove(full)
	}

	if err := a.store.DeleteMedia(int64(id)); err != nil && !errors.Is(err, store.ErrNotFound) {
		logErr("admin delete media", "id", id, "err", err)
	}
	a.redirect(ctx, "/admin/media?deleted=1")
}

func uploadExtension(mimeType, filename string) (string, bool) {
	switch mimeType {
	case "image/jpeg":
		return ".jpg", true
	case "image/png":
		return ".png", true
	case "image/gif":
		return ".gif", true
	case "image/webp":
		return ".webp", true
	case "application/pdf":
		return ".pdf", true
	case "application/zip":
		return ".zip", true
	case "text/plain; charset=utf-8", "text/plain":
		switch strings.ToLower(filepath.Ext(filename)) {
		case ".md", ".markdown":
			return strings.ToLower(filepath.Ext(filename)), true
		}
		return ".txt", true
	}
	return "", false
}

func randomName() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		panic("pionus: cannot generate upload name: " + err.Error())
	}
	return hex.EncodeToString(buf)
}

func withinDir(base, target string) bool {
	rel, err := filepath.Rel(base, target)
	if err != nil || filepath.IsAbs(rel) {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
