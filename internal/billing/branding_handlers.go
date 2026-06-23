package billing

import (
	_ "embed"
	"io"
	"net/http"
	"strings"

	"koffy/internal/httpx"
)

//go:embed assets/koffy.png
var defaultLogoPNG []byte

//go:embed assets/favicon.svg
var defaultFaviconSVG []byte

func (s *Server) brandingLogo(w http.ResponseWriter, r *http.Request) {
	area := brandingArea(r)
	assetKey := brandingLogoAssetKey(area)
	asset, ok, err := s.store.BrandingAsset(r.Context(), assetKey)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "load_logo_failed", err.Error())
		return
	}
	if !ok && area == "center" {
		legacyAsset, legacyOK, legacyErr := s.store.BrandingAsset(r.Context(), legacyLogoAssetKey)
		if legacyErr != nil {
			httpx.Error(w, http.StatusInternalServerError, "load_logo_failed", legacyErr.Error())
			return
		}
		asset = legacyAsset
		ok = legacyOK
	}

	data := defaultLogoPNG
	if ok {
		data = asset.Data
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (s *Server) adminUploadBrandingLogo(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	area := brandingArea(r)
	assetKey := brandingLogoAssetKey(area)

	r.Body = http.MaxBytesReader(w, r.Body, logoMaxInputSize+1024)
	if err := r.ParseMultipartForm(logoMaxInputSize); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_upload", "请上传小于 5MB 的图片文件")
		return
	}

	file, header, err := r.FormFile("logo")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "missing_logo", "logo file is required")
		return
	}
	defer file.Close()

	raw, err := io.ReadAll(file)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "read_logo_failed", err.Error())
		return
	}

	filename := ""
	if header != nil {
		filename = strings.TrimSpace(header.Filename)
	}
	asset, err := processLogoUpload(raw, filename)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_logo", err.Error())
		return
	}

	if err := s.store.SaveBrandingAsset(r.Context(), admin.ID, assetKey, asset); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "save_logo_failed", err.Error())
		return
	}

	logoURL := "/api/v1/branding/logo?area=" + area
	httpx.JSON(w, http.StatusOK, map[string]any{
		"status":     "ok",
		"area":       area,
		"logo_url":   logoURL,
		"size_bytes": asset.SizeBytes,
		"width":      asset.Width,
		"height":     asset.Height,
	})
}

func (s *Server) brandingFavicon(w http.ResponseWriter, r *http.Request) {
	area := brandingArea(r)
	asset, ok, err := s.store.BrandingAsset(r.Context(), brandingFaviconAssetKey(area))
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "load_favicon_failed", err.Error())
		return
	}

	data := defaultFaviconSVG
	contentType := "image/svg+xml"
	if ok {
		data = asset.Data
		contentType = asset.ContentType
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (s *Server) adminUploadBrandingFavicon(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	area := brandingArea(r)

	r.Body = http.MaxBytesReader(w, r.Body, faviconMaxInputSize+1024)
	if err := r.ParseMultipartForm(faviconMaxInputSize); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_upload", "请上传小于 2MB 的图片文件")
		return
	}

	file, header, err := r.FormFile("favicon")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "missing_favicon", "favicon file is required")
		return
	}
	defer file.Close()

	raw, err := io.ReadAll(file)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "read_favicon_failed", err.Error())
		return
	}
	filename := ""
	if header != nil {
		filename = strings.TrimSpace(header.Filename)
	}
	asset, err := processFaviconUpload(raw, filename)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_favicon", err.Error())
		return
	}
	if err := s.store.SaveBrandingAsset(r.Context(), admin.ID, brandingFaviconAssetKey(area), asset); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "save_favicon_failed", err.Error())
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"status":      "ok",
		"area":        area,
		"favicon_url": "/api/v1/branding/favicon?area=" + area,
		"size_bytes":  asset.SizeBytes,
		"width":       asset.Width,
		"height":      asset.Height,
	})
}

func brandingArea(r *http.Request) string {
	area := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("area")))
	if area == "admin" {
		return "admin"
	}
	return "center"
}

func brandingFaviconAssetKey(area string) string {
	if area == "admin" {
		return adminFaviconAssetKey
	}
	return centerFaviconAssetKey
}

func brandingLogoAssetKey(area string) string {
	if area == "admin" {
		return adminLogoAssetKey
	}
	return centerLogoAssetKey
}
