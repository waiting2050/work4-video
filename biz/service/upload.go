package service

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"video/biz/model"
	"video/biz/utils"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	UploadDir           = "uploads/chunks"
	ChunkExpirationHours = 24
)

type UploadService struct {
	db *gorm.DB
}

func NewUploadService(db *gorm.DB) *UploadService {
	if err := os.MkdirAll(UploadDir, 0755); err != nil {
		log.Printf("[NewUploadService] Failed to create upload directory: %v", err)
	}
	return &UploadService{db: db}
}

type InitUploadRequest struct {
	FileName    string `form:"file_name" json:"file_name" binding:"required"`
	FileSize    int64  `form:"file_size" json:"file_size" binding:"required"`
	ChunkSize   int    `form:"chunk_size" json:"chunk_size" binding:"required"`
	TotalChunks int    `form:"total_chunks" json:"total_chunks" binding:"required"`
	Title       string `form:"title" json:"title"`
	Description string `form:"description" json:"description"`
}

type UploadChunkRequest struct {
	TaskID     string `form:"task_id" json:"task_id" binding:"required"`
	ChunkIndex int    `form:"chunk_index" json:"chunk_index" binding:"required"`
	Checksum   string `form:"checksum" json:"checksum" binding:"required"`
}

type MergeChunksRequest struct {
	TaskID string `form:"task_id" json:"task_id" binding:"required"`
}

func (s *UploadService) InitUpload(userID string, req *InitUploadRequest) (map[string]interface{}, error) {
	if req.FileSize <= 0 || req.ChunkSize <= 0 || req.TotalChunks <= 0 {
		return nil, utils.New(utils.CodeInvalidParam, "invalid upload parameters")
	}

	if req.ChunkSize > 10*1024*1024 {
		return nil, utils.New(utils.CodeInvalidParam, "chunk size too large, maximum is 10MB")
	}

	expectedChunks := int(req.FileSize + int64(req.ChunkSize) - 1) / req.ChunkSize
	if req.TotalChunks != expectedChunks {
		log.Printf("[InitUpload] Chunk count mismatch: expected %d, got %d", expectedChunks, req.TotalChunks)
	}

	task := model.UploadTask{
		ID:             uuid.New().String(),
		UserID:         userID,
		FileName:       req.FileName,
		FileSize:       req.FileSize,
		ChunkSize:      req.ChunkSize,
		TotalChunks:    req.TotalChunks,
		UploadedChunks: 0,
		Status:         "pending",
		Title:          req.Title,
		Description:    req.Description,
	}

	if err := s.db.Create(&task).Error; err != nil {
		log.Printf("[InitUpload] Failed to create task: %v", err)
		return nil, fmt.Errorf("failed to create upload task: %w", err)
	}

	taskDir := filepath.Join(UploadDir, task.ID)
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		log.Printf("[InitUpload] Failed to create task directory: %v", err)
		return nil, fmt.Errorf("failed to create task directory: %w", err)
	}

	log.Printf("[InitUpload] Successfully created upload task: %s for user %s", task.ID, userID)
	return map[string]interface{}{
		"task_id":      task.ID,
		"chunk_size":   task.ChunkSize,
		"total_chunks": task.TotalChunks,
		"upload_url":   "/upload/chunk",
	}, nil
}

func (s *UploadService) UploadChunk(userID string, req *UploadChunkRequest, chunkData []byte) error {
	var task model.UploadTask
	if err := s.db.Where("id = ? AND user_id = ?", req.TaskID, userID).First(&task).Error; err != nil {
		log.Printf("[UploadService.UploadChunk] Upload task not found: %s", req.TaskID)
		return utils.New(utils.CodeTaskNotFound, "upload task not found")
	}

	if task.Status == "completed" {
		return utils.New(utils.CodeAlreadyPublished, "upload task already completed")
	}

	if task.Status == "cancelled" {
		return utils.New(utils.CodeInvalidAction, "upload task was cancelled")
	}

	if req.ChunkIndex < 0 || req.ChunkIndex >= task.TotalChunks {
		return utils.New(utils.CodeInvalidParam, "invalid chunk index")
	}

	if int64(len(chunkData)) > int64(task.ChunkSize)*2 {
		return utils.New(utils.CodeFileTooLarge, "chunk data exceeds expected size")
	}

	hash := sha256.Sum256(chunkData)
	checksum := hex.EncodeToString(hash[:])

	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var existingChunk model.UploadChunk
	err := tx.Where("task_id = ? AND chunk_index = ?", req.TaskID, req.ChunkIndex).First(&existingChunk).Error
	if err == nil {
		chunkPath := filepath.Join(UploadDir, req.TaskID, fmt.Sprintf("chunk_%d", req.ChunkIndex))
		existingData, err := os.ReadFile(chunkPath)
		if err == nil {
			hash := sha256.Sum256(existingData)
			existingChecksum := hex.EncodeToString(hash[:])
			if existingChecksum == req.Checksum {
				tx.Rollback()
				log.Printf("[UploadChunk] Chunk %d already uploaded and verified", req.ChunkIndex)
				return nil
			}
		}
		tx.Where("task_id = ? AND chunk_index = ?", req.TaskID, req.ChunkIndex).Delete(&model.UploadChunk{})
		log.Printf("[UploadChunk] Chunk %d corrupted, re-uploading", req.ChunkIndex)
	}

	chunk := model.UploadChunk{
		ID:         uuid.New().String(),
		TaskID:     req.TaskID,
		ChunkIndex: req.ChunkIndex,
		ChunkSize:  int64(len(chunkData)),
		Checksum:   checksum,
	}

	if err := tx.Create(&chunk).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to create chunk record: %w", err)
	}

	chunkPath := filepath.Join(UploadDir, req.TaskID, fmt.Sprintf("chunk_%d", req.ChunkIndex))
	if err := os.WriteFile(chunkPath, chunkData, 0644); err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to save chunk file: %w", err)
	}

	if err := tx.Model(&model.UploadTask{}).
		Where("id = ?", req.TaskID).
		UpdateColumn("uploaded_chunks", gorm.Expr("uploaded_chunks + 1")).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to update uploaded chunks count: %w", err)
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("[UploadChunk] Successfully uploaded chunk %d for task %s", req.ChunkIndex, req.TaskID)
	return nil
}

func (s *UploadService) GetUploadStatus(userID, taskID string) (map[string]interface{}, error) {
	var task model.UploadTask
	if err := s.db.Where("id = ? AND user_id = ?", taskID, userID).First(&task).Error; err != nil {
		log.Printf("[UploadService.GetUploadStatus] Upload task not found: %s", taskID)
		return nil, utils.New(utils.CodeTaskNotFound, "upload task not found")
	}

	var chunks []model.UploadChunk
	if err := s.db.Where("task_id = ?", taskID).Find(&chunks).Error; err != nil {
		return nil, fmt.Errorf("failed to get chunks: %w", err)
	}

	uploadedIndexes := make([]int, 0, len(chunks))
	for _, c := range chunks {
		uploadedIndexes = append(uploadedIndexes, c.ChunkIndex)
	}

	log.Printf("[GetUploadStatus] Task %s status: %s, %d/%d chunks uploaded",
		taskID, task.Status, task.UploadedChunks, task.TotalChunks)

	return map[string]interface{}{
		"task_id":          task.ID,
		"file_name":        task.FileName,
		"file_size":        task.FileSize,
		"chunk_size":       task.ChunkSize,
		"total_chunks":     task.TotalChunks,
		"uploaded_chunks":  task.UploadedChunks,
		"uploaded_indexes": uploadedIndexes,
		"status":           task.Status,
		"title":            task.Title,
		"description":      task.Description,
	}, nil
}

func (s *UploadService) MergeChunks(userID string, req *MergeChunksRequest) (string, string, string, error) {
	var task model.UploadTask
	if err := s.db.Where("id = ? AND user_id = ?", req.TaskID, userID).First(&task).Error; err != nil {
		log.Printf("[UploadService.MergeChunks] Upload task not found: %s", req.TaskID)
		return "", "", "", utils.New(utils.CodeTaskNotFound, "upload task not found")
	}

	if task.Status == "completed" {
		return "", "", "", utils.New(utils.CodeAlreadyPublished, "video already published")
	}

	if task.Status == "cancelled" {
		return "", "", "", utils.New(utils.CodeInvalidAction, "upload task was cancelled")
	}

	if task.UploadedChunks < task.TotalChunks {
		return "", "", "", utils.New(utils.CodeUploadIncomplete, "upload not complete, chunks missing")
	}

	var chunks []model.UploadChunk
	if err := s.db.Where("task_id = ?", req.TaskID).Order("chunk_index").Find(&chunks).Error; err != nil {
		return "", "", "", fmt.Errorf("failed to get chunks: %w", err)
	}

	mergedFilePath := filepath.Join("uploads/videos", task.FileName)

	fileCreated := false
	var cleanupFunc func() = func() {
		if fileCreated {
			os.Remove(mergedFilePath)
		}
		s.cleanupChunks(req.TaskID)
	}

	mergedFile, err := os.Create(mergedFilePath)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to create merged file: %w", err)
	}
	fileCreated = true

	for _, chunk := range chunks {
		chunkPath := filepath.Join(UploadDir, req.TaskID, fmt.Sprintf("chunk_%d", chunk.ChunkIndex))
		chunkData, err := os.ReadFile(chunkPath)
		if err != nil {
			cleanupFunc()
			return "", "", "", fmt.Errorf("failed to read chunk %d: %w", chunk.ChunkIndex, err)
		}

		if _, err := mergedFile.Write(chunkData); err != nil {
			cleanupFunc()
			return "", "", "", fmt.Errorf("failed to write chunk %d to merged file: %w", chunk.ChunkIndex, err)
		}
	}

	mergedFile.Close()
	fileCreated = false

	fileInfo, err := os.Stat(mergedFilePath)
	if err != nil {
		cleanupFunc()
		return "", "", "", fmt.Errorf("failed to get merged file info: %w", err)
	}

	if fileInfo.Size() != task.FileSize {
		cleanupFunc()
		return "", "", "", utils.New(utils.CodeMergeFailed, "merged file size mismatch")
	}

	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Model(&model.UploadTask{}).
		Where("id = ?", req.TaskID).
		Updates(map[string]interface{}{
			"status":  "completed",
			"file_url": mergedFilePath,
		}).Error; err != nil {
		cleanupFunc()
		tx.Rollback()
		return "", "", "", fmt.Errorf("failed to update task status: %w", err)
	}

	videoURL := "/uploads/videos/" + task.FileName
	coverURL := "/uploads/videos/covers/" + task.FileName + ".jpg"

	if err := tx.Commit().Error; err != nil {
		cleanupFunc()
		return "", "", "", fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.cleanupChunks(req.TaskID)

	log.Printf("[MergeChunks] Successfully merged %d chunks for task %s", len(chunks), req.TaskID)
	return videoURL, coverURL, req.TaskID, nil
}

func (s *UploadService) CancelUpload(userID, taskID string) error {
	var task model.UploadTask
	if err := s.db.Where("id = ? AND user_id = ?", taskID, userID).First(&task).Error; err != nil {
		log.Printf("[UploadService.CancelUpload] Upload task not found: %s", taskID)
		return utils.New(utils.CodeTaskNotFound, "upload task not found")
	}

	if task.Status == "completed" {
		return utils.New(utils.CodeInvalidAction, "cannot cancel completed upload")
	}

	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Where("id = ?", taskID).Delete(&model.UploadChunk{}).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete chunks: %w", err)
	}

	if err := tx.Model(&model.UploadTask{}).
		Where("id = ?", taskID).
		Update("status", "cancelled").Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to update task status: %w", err)
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.cleanupChunks(taskID)

	log.Printf("[CancelUpload] Successfully cancelled upload task: %s", taskID)
	return nil
}

func (s *UploadService) cleanupChunks(taskID string) {
	taskDir := filepath.Join(UploadDir, taskID)
	if err := os.RemoveAll(taskDir); err != nil {
		log.Printf("[cleanupChunks] Failed to cleanup task directory: %v", err)
	}
}

func (s *UploadService) CleanupStaleTasks() error {
	expirationTime := time.Now().Add(-24 * time.Hour)

	var staleTasks []model.UploadTask
	if err := s.db.Where("status = ? AND created_at < ?", "pending", expirationTime).Find(&staleTasks).Error; err != nil {
		return fmt.Errorf("failed to find stale tasks: %w", err)
	}

	for _, task := range staleTasks {
		s.cleanupChunks(task.ID)

		if err := s.db.Where("task_id = ?", task.ID).Delete(&model.UploadChunk{}).Error; err != nil {
			log.Printf("[CleanupStaleTasks] Failed to delete chunks for task %s: %v", task.ID, err)
		}

		if err := s.db.Model(&model.UploadTask{}).
			Where("id = ?", task.ID).
			Update("status", "failed").Error; err != nil {
			log.Printf("[CleanupStaleTasks] Failed to update task %s status: %v", task.ID, err)
		}

		log.Printf("[CleanupStaleTasks] Cleaned up stale task: %s", task.ID)
	}

	return nil
}

func (s *UploadService) GetUploadProgress(taskID, userID string) (map[string]interface{}, error) {
	task, err := s.GetUploadStatus(userID, taskID)
	if err != nil {
		return nil, err
	}

	progress := float64(0)
	if total, ok := task["total_chunks"].(int); ok && total > 0 {
		uploaded, ok := task["uploaded_chunks"].(int)
		if ok {
			progress = float64(uploaded) / float64(total) * 100
		}
	}

	task["progress"] = progress

	return task, nil
}

func (s *UploadService) ReadChunkFromRequest(req *UploadChunkRequest, r io.Reader) ([]byte, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read chunk data: %w", err)
	}

	expectedSize := int64(len(data))
	hash := sha256.Sum256(data)
	checksum := hex.EncodeToString(hash[:])

	if checksum != req.Checksum {
		return nil, fmt.Errorf("checksum mismatch: expected %s, got %s", req.Checksum, checksum)
	}

	log.Printf("[ReadChunkFromRequest] Successfully read %d bytes with checksum %s", expectedSize, checksum)
	return data, nil
}
