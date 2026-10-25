package admin

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/pionus/arry"
	"github.com/pionus/pionus-go/internal/typecho"
)

func (a *Admin) importPage(ctx arry.Context) {
	data := a.base(ctx, "导入 Typecho")
	a.render(ctx, http.StatusOK, "import.html", data)
}

func (a *Admin) importSubmit(ctx arry.Context) {
	if err := ctx.Request().ParseMultipartForm(32 << 20); err != nil {
		a.renderImport(ctx, http.StatusBadRequest, "无法解析上传内容。", "")
		return
	}
	if ctx.Request().MultipartForm != nil {
		defer ctx.Request().MultipartForm.RemoveAll()
	}
	file, _, err := ctx.Request().FormFile("bundle")
	if err != nil {
		a.renderImport(ctx, http.StatusBadRequest, "请选择 Typecho 迁移包。", "")
		return
	}
	defer file.Close()

	tempDir, err := os.MkdirTemp("", "pionus-admin-import-")
	if err != nil {
		a.renderImport(ctx, http.StatusInternalServerError, "无法创建临时目录。", "")
		return
	}
	defer os.RemoveAll(tempDir)

	bundlePath := filepath.Join(tempDir, "typecho-bundle.zip")
	out, err := os.Create(bundlePath)
	if err != nil {
		a.renderImport(ctx, http.StatusInternalServerError, "无法保存上传文件。", "")
		return
	}
	_, copyErr := io.Copy(out, file)
	closeErr := out.Close()
	if copyErr != nil || closeErr != nil {
		a.renderImport(ctx, http.StatusInternalServerError, "无法保存上传文件。", "")
		return
	}

	importDir, exportPath, uploads, err := typecho.ExtractBundle(bundlePath)
	if err != nil {
		a.renderImport(ctx, http.StatusBadRequest, friendlyImportError(err), "")
		return
	}
	defer os.RemoveAll(importDir)

	export, err := os.Open(exportPath)
	if err != nil {
		a.renderImport(ctx, http.StatusInternalServerError, "无法读取 Typecho 导出文件。", "")
		return
	}
	defer export.Close()

	dryRun := strings.TrimSpace(ctx.Request().PostFormValue("dry_run")) != ""
	drafts := strings.TrimSpace(ctx.Request().PostFormValue("drafts")) != ""
	report, err := typecho.Import(export, typecho.Options{
		Store:         a.store,
		SourceUploads: uploads,
		DestUploads:   a.uploadDir,
		DryRun:        dryRun,
		ImportDrafts:  drafts,
		Logger:        slog.Default(),
	})
	if err != nil {
		a.renderImport(ctx, http.StatusBadRequest, friendlyImportError(err), "")
		return
	}
	encoded, _ := json.MarshalIndent(report, "", "  ")
	a.renderImport(ctx, http.StatusOK, "", string(encoded))
}

func (a *Admin) renderImport(ctx arry.Context, code int, errMsg, report string) {
	data := a.base(ctx, "导入 Typecho")
	data["Error"] = errMsg
	data["Report"] = report
	a.render(ctx, code, "import.html", data)
}

func friendlyImportError(err error) string {
	if err == nil {
		return ""
	}
	text := err.Error()
	if strings.Contains(text, "unsafe path") {
		return "迁移包包含不安全的文件路径。"
	}
	if strings.Contains(text, "does not contain typecho.json") {
		return "迁移包中缺少 typecho.json。"
	}
	return fmt.Sprintf("导入失败：%s", text)
}
