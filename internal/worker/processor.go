package worker

import (
	"context"
	"log"
	"time"

	"github.com/PaulBabatuyi/UploadStream-gRPC/internal/database"
)

type WorkerConfig struct {
	DB              database.PostgresDB
	StoragePath     string
	PollInterval    time.Duration
	ShutdownTimeout time.Duration
}

type ProcessingWorker struct {
	config *WorkerConfig
	done   chan struct{}
}

func NewProcessingWorker(config *WorkerConfig) *ProcessingWorker {
	if config.PollInterval == 0 {
		config.PollInterval = 2 * time.Second
	}
	if config.ShutdownTimeout == 0 {
		config.ShutdownTimeout = 10 * time.Second
	}
	return &ProcessingWorker{
		config: config,
		done:   make(chan struct{}),
	}
}

func (pw *ProcessingWorker) Start(ctx context.Context) {
	go pw.run(ctx)
	log.Println("✓ Processing worker started")
}

func (pw *ProcessingWorker) Stop() {
	close(pw.done)
	log.Println("✓ Processing worker stopped")
}

func (pw *ProcessingWorker) run(ctx context.Context) {
	ticker := time.NewTicker(pw.config.PollInterval)
	defer ticker.Stop()

	imageProc := NewImageProcessor(pw.config.StoragePath)

	for {
		select {
		case <-pw.done:
			return
		case <-ticker.C:
			pw.processNext(ctx, imageProc)
		}
	}
}

func (pw *ProcessingWorker) processNext(ctx context.Context, imageProc *ImageProcessor) {
	job, err := pw.config.DB.GetNextPendingJob(ctx)
	if err != nil {
		log.Printf("Error getting next job: %v", err)
		return
	}
	if job == nil {
		return
	}

	log.Printf("Processing job %d for file %s", job.ID, job.FileID)

	pw.config.DB.UpdateJobStatus(ctx, job.ID, "processing", "")

	file, err := pw.config.DB.GetFile(ctx, job.FileID)
	if err != nil {
		log.Printf("File not found: %v", err)
		pw.config.DB.UpdateJobStatus(ctx, job.ID, "failed", "File not found")
		return
	}

	fileType := database.DeriveFileType(file.ContentType)

	switch fileType {
	case database.FileTypeImage:
		pw.processImage(ctx, job, imageProc, file)
	default:
		log.Printf("Skipping processing for non-image: %s", fileType)
		pw.config.DB.CompleteJob(ctx, job.ID, "", "", "", 0, 0)
	}
}

func (pw *ProcessingWorker) processImage(ctx context.Context, job *database.ProcessingJob, imageProc *ImageProcessor, file *database.FileRecord) {
	thumbSmall, thumbMed, thumbLarge, width, height, err := imageProc.ProcessImage(ctx, job.FileID, file.ContentType)
	if err != nil {
		log.Printf("Image processing failed: %v", err)
		pw.config.DB.UpdateJobStatus(ctx, job.ID, "failed", err.Error())
		return
	}

	err = pw.config.DB.CompleteJob(ctx, job.ID, thumbSmall, thumbMed, thumbLarge, width, height)
	if err != nil {
		log.Printf("Failed to save job results: %v", err)
		return
	}

	log.Printf(" Completed job %d: generated thumbnails", job.ID)
}
