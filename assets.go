package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func (cfg apiConfig) ensureAssetsDir() error {
	if _, err := os.Stat(cfg.assetsRoot); os.IsNotExist(err) {
		return os.Mkdir(cfg.assetsRoot, 0755)
	}
	return nil
}

func getAssetPath(mediaType string) string {
	base := make([]byte, 32)
	rand.Read(base)
	id := base64.RawURLEncoding.EncodeToString(base)
	ext := mediaTypeToExt(mediaType)
	return fmt.Sprintf("%s%s", id, ext)
}

func (cfg apiConfig) getAssetDiskPath(assetPath string) string {
	return filepath.Join(cfg.assetsRoot, assetPath)
}

func (cfg apiConfig) getAssetURL(assetPath string) string {
	return fmt.Sprintf("http://localhost:%s/%s", cfg.port, cfg.getAssetDiskPath(assetPath))
}

func mediaTypeToExt(s string) string {
	subStrs := strings.Split(s, "/")
	if len(subStrs) != 2 {
		return ".bin"
	}
	return "." + subStrs[1]
}

func getHexFilename(mediaType string, prefixes ...string) string {
	base := make([]byte, 32)
	rand.Read(base)
	id := hex.EncodeToString(base)
	ext := mediaTypeToExt(mediaType)
	filename := fmt.Sprintf("%s%s", id, ext)
	if len(prefixes) == 0 {
		return filename
	}
	prefixes = append(prefixes, filename)
	return strings.Join(prefixes, "/")
}

func (cfg *apiConfig) getObjectUrl(key string) string {
	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", cfg.s3Bucket, cfg.s3Region, key)
}

type FfprobeOutput struct {
	Streams []struct {
		Width  int `json:"width"`
		Height int `json:"height"`
	} `json:"streams"`
}

func getVideoAspectRatio(filePath string) (string, error) {
	cmd := exec.Command("ffprobe",
		"-v",
		"error",
		"-print_format",
		"json",
		"-show_streams",
		filePath,
	)

	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", err
	}
	var cmdOutput FfprobeOutput
	if err := json.Unmarshal(out.Bytes(), &cmdOutput); err != nil {
		return "", err
	}

	if len(cmdOutput.Streams) == 0 {
		return "", errors.New("no video streams found")
	}

	const landscapeRatio = float64(16) / 9
	const verticalRatio = float64(9) / 16
	const allowanceRatio = float64(0.02)
	realWidth := float64(cmdOutput.Streams[0].Width)
	realHeight := float64(cmdOutput.Streams[0].Height)
	realRatio := realWidth / realHeight

	if realRatio >= landscapeRatio-allowanceRatio && realRatio <= landscapeRatio+allowanceRatio {
		return "16:9", nil
	}

	if realRatio >= verticalRatio-allowanceRatio && realRatio <= verticalRatio+allowanceRatio {
		return "9:16", nil
	}

	return "other", nil
}
