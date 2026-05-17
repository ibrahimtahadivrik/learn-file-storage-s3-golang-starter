package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {

	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	const maxMemory int64 = 10 << 20
	err = r.ParseMultipartForm(maxMemory)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid multipart form data", err)
		return
	}

	fileData, fileHeader, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid thumbnail file", err)
		return
	}
	mediaType, _, err := mime.ParseMediaType(fileHeader.Header.Get("Content-Type"))
	if err != nil || (mediaType != "image/jpeg" && mediaType != "image/png") {
		respondWithError(w, http.StatusBadRequest, "Invalid file type", err)
		return
	}
	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Can not get video", err)
		return
	}

	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "You are not authorized to upload this video", err)
		return
	}

	if video.ThumbnailURL != nil {
		oldURL := *video.ThumbnailURL
		oldFileName := filepath.Base(oldURL)
		oldFilePath := filepath.Join(cfg.assetsRoot, oldFileName)
		os.Remove(oldFilePath)
	}

	n := make([]byte, 32)
	rand.Read(n)
	encodedName := base64.RawURLEncoding.EncodeToString(n)

	parts := strings.Split(mediaType, "/")
	filePth := filepath.Join(cfg.assetsRoot, encodedName+"."+parts[len(parts)-1])
	file, err := os.Create(filePth)
	defer file.Close()
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Can not create file", err)
		return
	}

	_, err = io.Copy(file, fileData)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Can not copy file", err)
		return
	}

	thumbnailURL := fmt.Sprintf("http://localhost:%s/assets/%s", cfg.port, encodedName+"."+parts[len(parts)-1])
	video.ThumbnailURL = &thumbnailURL

	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Can not update video", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	respondWithJSON(w, http.StatusOK, video)
}
