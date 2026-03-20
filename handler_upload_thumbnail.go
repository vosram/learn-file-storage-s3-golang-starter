package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"time"

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

	// TODO: implement the upload here
	const maxMemory = 10 << 20
	r.ParseMultipartForm(maxMemory)

	file, fileHeader, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}
	defer file.Close()

	mediaTypeStr := fileHeader.Header.Get("Content-Type")
	if mediaTypeStr == "" {
		respondWithError(w, http.StatusBadRequest, "Missing Content-Type for thumbnail", errors.New("Request missing Content-Type"))
		return
	}
	mediaType, _, err := mime.ParseMediaType(mediaTypeStr)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't parse Content-Type", err)
		return
	}
	acceptableMediaTypes := map[string]struct{}{
		"image/jpeg": {},
		"image/png":  {},
	}
	if _, ok := acceptableMediaTypes[mediaType]; !ok {
		respondWithError(w, http.StatusBadRequest, "Image must be either jpeg or png", nil)
		return
	}

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't get video from DB", err)
		return
	}
	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Not authorized to update this video", errors.New("User not authorized"))
		return
	}

	assetPath := getAssetPath(mediaType)
	assetDiskPath := cfg.getAssetDiskPath(assetPath)

	newFile, err := os.Create(assetDiskPath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't create file on server", err)
		return
	}
	defer newFile.Close()
	if _, err := io.Copy(newFile, file); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't save file", err)
		return
	}

	thumbnailUrl := cfg.getAssetURL(assetPath)
	video.ThumbnailURL = &thumbnailUrl
	video.UpdatedAt = time.Now()

	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't update video", err)
		err = os.Remove(assetDiskPath)
		if err != nil {
			log.Printf("Failed to remove file: %s", assetDiskPath)
		}
		log.Printf("Removed file: %s because it didn't save in the DB", assetDiskPath)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}
