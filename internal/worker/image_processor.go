package worker

import (
	"context"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"os"
	"path/filepath"

	"github.com/disintegration/imaging"
)

type ImageProcessor struct {
	storagePath string
}

func NewImageProcessor(storagePath string) *ImageProcessor {
	return &ImageProcessor{storagePath: storagePath}
}

func (ip *ImageProcessor) ProcessImage(ctx context.Context, fileID, contentType string) (
	thumbSmall, thumbMed, thumbLarge string,
	width, height int,
	err error,
) {
	filePath := filepath.Join(ip.storagePath, fileID)
	file, err := os.Open(filePath)
	if err != nil {
		return "", "", "", 0, 0, fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	img, _, err := image.DecodeConfig(file)
	if err != nil {
		return "", "", "", 0, 0, fmt.Errorf("decode image: %w", err)
	}

	width, height = img.Width, img.Height

	file.Seek(0, 0)
	origImg, _, err := image.Decode(file)
	if err != nil {
		return "", "", "", 0, 0, fmt.Errorf("decode: %w", err)
	}

	thumbSmall = ip.saveThumbnail(fileID, origImg, 150, "small")
	thumbMed = ip.saveThumbnail(fileID, origImg, 400, "medium")
	thumbLarge = ip.saveThumbnail(fileID, origImg, 800, "large")

	return thumbSmall, thumbMed, thumbLarge, width, height, nil
}

func (ip *ImageProcessor) saveThumbnail(fileID string, img image.Image, maxWidth int, size string) string {
	bounds := img.Bounds()
	origWidth := bounds.Max.X - bounds.Min.X
	origHeight := bounds.Max.Y - bounds.Min.Y

	newHeight := (origHeight * maxWidth) / origWidth

	thumb := imaging.Resize(img, maxWidth, newHeight, imaging.Lanczos)

	thumbPath := fmt.Sprintf("%s-thumb-%s.jpg", fileID, size)
	fullPath := filepath.Join(ip.storagePath, thumbPath)
	err := imaging.Save(thumb, fullPath)
	if err != nil {
		log.Printf("Failed to save thumbnail: %v", err)
		return ""
	}

	return thumbPath
}
