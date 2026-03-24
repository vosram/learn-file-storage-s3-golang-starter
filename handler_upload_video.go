package main

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
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

	const maxMemory = 1 << 30
	r.ParseMultipartForm(maxMemory)

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't fetch video", err)
		return
	}

	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "You are not authorized", err)
		return
	}

	file, fileHeader, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}
	defer file.Close()

	mediaTypeStr := fileHeader.Header.Get("Content-Type")
	if mediaTypeStr == "" {
		respondWithError(w, http.StatusBadRequest, "Missing Content-Type for video", err)
		return
	}

	mediaType, _, err := mime.ParseMediaType(mediaTypeStr)
	if mediaType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "Not a valid media type", nil)
		return
	}

	tempFile, err := os.CreateTemp("", "tubely-upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to upload file", err)
		return
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()
	if _, err := io.Copy(tempFile, file); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to upload file", err)
		return
	}

	_, err = tempFile.Seek(0, io.SeekStart)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to upload file", err)
		return
	}
	key := getHexFilename(mediaType)
	bucket := cfg.s3Bucket
	inputObj := s3.PutObjectInput{
		Key:         &key,
		Bucket:      &bucket,
		Body:        tempFile,
		ContentType: &mediaType,
	}
	_, err = cfg.s3Client.PutObject(r.Context(), &inputObj)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to upload file", err)
		return
	}
	videoUrl := cfg.getObjectUrl(key)
	video.VideoURL = &videoUrl
	err = cfg.db.UpdateVideo(video)
	if err != nil {
		msg := fmt.Sprintf("failed to save file %s to DB", videoUrl)
		respondWithError(w, http.StatusInternalServerError, msg, err)
		return
	}

	respondWithJSON(w, http.StatusCreated, video)
}
