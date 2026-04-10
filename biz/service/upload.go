package service

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"video/biz/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	DefaultChunkSize = 5 * 1024 * 1024  // 默认分片大小 5MB
	MaxChunkSize     = 50 * 1024 * 1024 // 最大分片大小 50MB
	UploadDir        = "uploads/chunks" // 上传文件存储目录
	VideoDir         = "uploads/videos" // 最终视频存储目录
)


type UploadService struct {
	db *gorm.DB
}

// NewUploadService 创建上传服务实例
func NewUploadService(db *gorm.DB) *UploadService {
	// 确保上传目录存在
	if err := os.MkdirAll(UploadDir, 0755); err != nil {
		log.Printf("[UploadService] Failed to create upload directory: %v", err)
	}
	if err := os.MkdirAll(VideoDir, 0755); err != nil {
		log.Printf("[UploadService] Failed to create video directory: %v", err)
	}
	return &UploadService{db: db}
}

// InitUploadRequest 初始化上传请求
type InitUploadRequest struct {
	FileName    string `json:"file_name" form:"file_name"`
	FileSize    int64  `json:"file_size" form:"file_size"`
	ChunkSize   int    `json:"chunk_size" form:"chunk_size"`
	Title       string `json:"title" form:"title"`
	Description string `json:"description" form:"description"`
}

// InitUploadResponse 初始化上传响应
type InitUploadResponse struct {
	TaskID      string `json:"task_id"`
	ChunkSize   int    `json:"chunk_size"`
	TotalChunks int    `json:"total_chunks"`
}

func (s *UploadService) InitUpload(userID string, req *InitUploadRequest) (*InitUploadResponse, error) {
	// 验证文件格式
	ext := strings.ToLower(filepath.Ext(req.FileName))
	if ext != ".mp4" && ext != ".mov" && ext != ".avi" && ext != ".mkv" {
		log.Printf("[UploadService.InitUpload] Invalid video format: %s", req.FileName)
		return nil, errors.New("invalid video format, supported: mp4, mov, avi, mkv")
	}

	// 确定分片大小
	chunkSize := req.ChunkSize
	if chunkSize <= 0 {
		chunkSize = DefaultChunkSize
	}
	if chunkSize > MaxChunkSize {
		chunkSize = MaxChunkSize
	}

	// 计算总分片数
	chunkSize64 := int64(chunkSize)
	totalChunks := int((req.FileSize + chunkSize64 - 1) / chunkSize64)

	if totalChunks > 10000 {
		log.Printf("[UploadService.InitUpload] Too many chunks: %d", totalChunks)
		return nil, errors.New("file too large, too many chunks")
	}

	// 创建上传任务
	task := &model.UploadTask{
		ID:          uuid.New().String(),
		UserID:      userID,
		FileName:    req.FileName,
		FileSize:    req.FileSize,
		ChunkSize:   chunkSize,
		TotalChunks: totalChunks,
		Status:      "pending",
		Title:       req.Title,
		Description: req.Description,
	}
	
	if err := s.db.Create(task).Error; err != nil {
		log.Printf("[UploadService.InitUpload] Failed to create upload task: %v", err)
		return nil, fmt.Errorf("failed to create upload task: %w", err)
	}

	// 创建任务目录
	taskDir := filepath.Join(UploadDir, task.ID)
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		log.Printf("[UploadService.InitUpload] Failed to create task directory: %v", err)
		return nil, fmt.Errorf("failed to create task directory: %w", err)
	}

	log.Printf("[UploadService.InitUpload] Upload task created: %s, total chunks: %d", task.ID, totalChunks)
	return &InitUploadResponse{
		TaskID:      task.ID,
		ChunkSize:   chunkSize,
		TotalChunks: totalChunks,
	}, nil
}

// UploadChunkRequest 上传分片请求
type UploadChunkRequest struct {
	TaskID     string `form:"task_id"`
	ChunkIndex int    `form:"chunk_index"`
	Checksum   string `form:"checksum"`
}

// UploadChunk 上传单个分片
func (s *UploadService) UploadChunk(userID string, req *UploadChunkRequest, chunkData []byte) error {
	// 查询上传任务
	var task model.UploadTask
	if err := s.db.Where("id = ? AND user_id = ?",
		req.TaskID, userID).First(&task).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("[UploadService.UploadChunk] Upload task not found: %s", req.TaskID)
			return errors.New("upload task not found")
		}
		log.Printf("[UploadService.UploadChunk] Failed to get upload task: %v", err)
		return fmt.Errorf("failed to get upload task: %w", err)
	}

	// 检查任务状态
	if task.Status == "completed" {
		return errors.New("upload task already completed")
	}
	if task.Status == "cancelled" {
		return errors.New("upload task has been cancelled")
	}

	// 验证分片索引
	if req.ChunkIndex < 0 || req.ChunkIndex >= task.TotalChunks {
		log.Printf("[UploadService.UploadChunk] Invalid chunk index: %d, total chunks: %d", req.ChunkIndex, task.TotalChunks)
		return errors.New("invalid chunk index")
	}

	// 校验分片数据完整性
	if req.Checksum != "" {
		hash := sha256.Sum256(chunkData)
		calculatedChecksum := hex.EncodeToString(hash[:])
		if calculatedChecksum != req.Checksum {
			log.Printf("[UploadService.UploadChunk] Checksum mismatch for chunk %d", req.ChunkIndex)
			return errors.New("chunk checksum mismatch")
		}
	}

	// 使用事务处理分片上传，避免并发问题
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 检查分片是否已上传（在事务内检查）
	var existingChunk model.UploadChunk
	err := tx.Where("task_id = ? AND chunk_index = ?", req.TaskID, req.ChunkIndex).First(&existingChunk).Error
	if err == nil {
		// 分片已存在，验证完整性
		chunkPath := filepath.Join(UploadDir, req.TaskID, fmt.Sprintf("chunk_%d", req.ChunkIndex))
		existingData, err := os.ReadFile(chunkPath)
		if err == nil {
			hash := sha256.Sum256(existingData)
			existingChecksum := hex.EncodeToString(hash[:])
			if existingChecksum == req.Checksum {
				tx.Rollback()
				log.Printf("[UploadService.UploadChunk] Chunk %d already uploaded and verified", req.ChunkIndex)
				return nil // 分片已上传且完整，直接返回成功
			}
		}
		// 分片损坏，删除旧记录以便重新上传
		tx.Where("task_id = ? AND chunk_index = ?", req.TaskID, req.ChunkIndex).Delete(&model.UploadChunk{})
		log.Printf("[UploadService.UploadChunk] Chunk %d corrupted, re-uploading", req.ChunkIndex)
	}

	// 保存分片数据到文件
	chunkPath := filepath.Join(UploadDir, req.TaskID, fmt.Sprintf("chunk_%d", req.ChunkIndex))
	if err := os.WriteFile(chunkPath, chunkData, 0644); err != nil {
		tx.Rollback()
		log.Printf("[UploadService.UploadChunk] Failed to save chunk %d: %v", req.ChunkIndex, err)
		return fmt.Errorf("failed to save chunk: %w", err)
	}

	// 计算校验和
	hash := sha256.Sum256(chunkData)
	checksum := hex.EncodeToString(hash[:])

	// 保存分片记录到数据库
	chunk := &model.UploadChunk{
		ID:         uuid.New().String(),
		TaskID:     req.TaskID,
		ChunkIndex: req.ChunkIndex,
		ChunkSize:  int64(len(chunkData)),
		Checksum:   checksum,
	}

	if err := tx.Create(chunk).Error; err != nil {
		tx.Rollback()
		log.Printf("[UploadService.UploadChunk] Failed to save chunk record: %v", err)
		// 删除已保存的文件
		os.Remove(chunkPath)
		return fmt.Errorf("failed to save chunk record: %w", err)
	}

	// 更新任务状态
	if task.Status == "pending" {
		task.Status = "uploading"
	}
	task.UploadedChunks++
	if err := tx.Model(&model.UploadTask{}).Where("id = ?", task.ID).Updates(map[string]interface{}{
		"status":           task.Status,
		"uploaded_chunks":  task.UploadedChunks,
	}).Error; err != nil {
		tx.Rollback()
		log.Printf("[UploadService.UploadChunk] Failed to update task status: %v", err)
		return fmt.Errorf("failed to update task status: %w", err)
	}

	if err := tx.Commit().Error; err != nil {
		log.Printf("[UploadService.UploadChunk] Failed to commit transaction: %v", err)
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("[UploadService.UploadChunk] Chunk %d uploaded successfully for task %s", req.ChunkIndex, req.TaskID)
	return nil
}

// GetUploadStatus 获取上传任务状态
func (s *UploadService) GetUploadStatus(userID, taskID string) (map[string]interface{}, error) {
	var task model.UploadTask
	if err := s.db.Where("id = ? AND user_id = ?", taskID, userID).First(&task).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("[UploadService.GetUploadStatus] Upload task not found: %s", taskID)
			return nil, errors.New("upload task not found")
		}
		log.Printf("[UploadService.GetUploadStatus] Failed to get upload task: %v", err)
		return nil, fmt.Errorf("failed to get upload task: %w", err)
	}

	// 获取已上传的分片索引
	var chunks []model.UploadChunk
	if err := s.db.Where("task_id = ?", taskID).Find(&chunks).Error; err != nil {
		log.Printf("[UploadService.GetUploadStatus] Failed to get chunks: %v", err)
		return nil, fmt.Errorf("failed to get chunks: %w", err)
	}

	uploadedIndices := make([]int, 0, len(chunks))
	for _, chunk := range chunks {
		uploadedIndices = append(uploadedIndices, chunk.ChunkIndex)
	}
	sort.Ints(uploadedIndices)

	progress := float64(task.UploadedChunks) / float64(task.TotalChunks) * 100

	return map[string]interface{}{
		"task_id":          task.ID,
		"status":           task.Status,
		"file_name":        task.FileName,
		"file_size":        task.FileSize,
		"total_chunks":     task.TotalChunks,
		"uploaded_chunks":  task.UploadedChunks,
		"progress":         progress,
		"uploaded_indices": uploadedIndices,
		"created_at":       task.CreatedAt,
		"updated_at":       task.UpdatedAt,
	}, nil
}

// MergeChunksRequest 合并分片请求
type MergeChunksRequest struct {
	TaskID string  `json:"task_id" form:"task_id"`
}

func (s *UploadService) MergeChunks(userID string, req *MergeChunksRequest) (string, string, string, error) {
	// 查询上传任务
	var task model.UploadTask
	if err := s.db.Where("id = ? AND user_id = ?", req.TaskID, userID).First(&task).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("[UploadService.MergeChunks] Upload task not found: %s", req.TaskID)
			return "", "", "", errors.New("upload task not found")
		}
		log.Printf("[UploadService.MergeChunks] Failed to get upload task: %v", err)
		return "", "", "", fmt.Errorf("failed to get upload task: %w", err)
	}

	// 检查任务状态
	if task.Status == "completed" {
		return "", "", "", errors.New("upload task already completed")
	}
	if task.Status == "cancelled" {
		return "", "", "", errors.New("upload task has been cancelled")
	}

	// 检查是否所有分片都已上传
	if task.UploadedChunks < task.TotalChunks {
		log.Printf("[UploadService.MergeChunks] Not all chunks uploaded: %d/%d", task.UploadedChunks, task.TotalChunks)
		return "", "", "", errors.New("not all chunks uploaded")
	}

	// 获取所有分片
	var chunks []model.UploadChunk
	if err := s.db.Where("task_id = ?", req.TaskID).Order("chunk_index").Find(&chunks).Error; err != nil {
		log.Printf("[UploadService.MergeChunks] Failed to get chunks: %v", err)
		return "", "", "", fmt.Errorf("failed to get chunks: %w", err)
	}

	if len(chunks) != task.TotalChunks {
		log.Printf("[UploadService.MergeChunks] Chunk count mismatch: expected %d, got %d", task.TotalChunks, len(chunks))
		return "", "", "", errors.New("chunk count mismatch")
	}

	// 生成最终文件名
	ext := strings.ToLower(filepath.Ext(task.FileName))
	finalFilename := task.ID + ext
	finalPath := filepath.Join(VideoDir, finalFilename)

	// 创建最终文件
	finalFile, err := os.Create(finalPath)
	if err != nil {
		log.Printf("[UploadService.MergeChunks] Failed to create final file: %v", err)
		return "", "", "", fmt.Errorf("failed to create final file: %w", err)
	}
	defer finalFile.Close()

	// 按顺序合并分片
	for _, chunk := range chunks {
		chunkPath := filepath.Join(UploadDir, req.TaskID, fmt.Sprintf("chunk_%d", chunk.ChunkIndex))
		chunkFile, err := os.Open(chunkPath)
		if err != nil {
			log.Printf("[UploadService.MergeChunks] Failed to open chunk %d: %v", chunk.ChunkIndex, err)
			return "", "", "", fmt.Errorf("failed to open chunk %d: %w", chunk.ChunkIndex, err)
		}

		_, err = io.Copy(finalFile, chunkFile)
		chunkFile.Close()
		if err != nil {
			log.Printf("[UploadService.MergeChunks] Failed to copy chunk %d: %v", chunk.ChunkIndex, err)
			return "", "", "", fmt.Errorf("failed to copy chunk %d: %w", chunk.ChunkIndex, err)
		}
	}

	// 生成视频URL和封面URL
	videoURL := "/uploads/videos/" + finalFilename
	coverURL := "/uploads/videos/covers/" + strings.TrimSuffix(finalFilename, ext) + ".jpg"

	// 使用事务更新任务状态
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Model(&model.UploadTask{}).Where("id = ?", task.ID).Updates(map[string]interface{}{
		"status":   "completed",
		"file_url": videoURL,
	}).Error; err != nil {
		tx.Rollback()
		log.Printf("[UploadService.MergeChunks] Failed to update task status: %v", err)
		return "", "", "", fmt.Errorf("failed to update task status: %w", err)
	}

	if err := tx.Commit().Error; err != nil {
		log.Printf("[UploadService.MergeChunks] Failed to commit transaction: %v", err)
		return "", "", "", fmt.Errorf("failed to commit transaction: %w", err)
	}

	// 清理分片文件（异步）
	go s.cleanupChunks(req.TaskID)

	log.Printf("[UploadService.MergeChunks] Chunks merged successfully: %s", finalPath)
	return videoURL, coverURL, task.ID, nil
}

// 取消分片文件
func (s *UploadService) cleanupChunks(taskID string) {
	taskDir := filepath.Join(UploadDir, taskID)
	if err := os.RemoveAll(taskDir); err != nil {
		log.Printf("[UploadService.cleanupChunks] Failed to clean up chunks for task %s: %v", taskID, err)
	} else {
		log.Printf("[UploadService.cleanupChunks] Chunks cleaned up for task %s", taskID)
	}
}

// 取消上传任务
func (s *UploadService) CancelUpload(userID, taskID string) error {
	var task model.UploadTask
	if err := s.db.Where("id = ? AND user_id = ?", taskID, userID).First(&task).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("[UploadService.CancelUpload] Upload task not found: %s", taskID)
			return errors.New("upload task not found")
		}
		log.Printf("[UploadService.CancelUpload] Failed to get upload task: %v", err)
		return fmt.Errorf("failed to get upload task: %w", err)
	}

	if task.Status == "completed" {
		return errors.New("cannot cancel completed task")
	}

	// 更新任务状态
	task.Status = "cancelled"
	if err := s.db.Save(&task).Error; err != nil {
		log.Printf("[UploadService.CancelUpload] Failed to update task status: %v", err)
		return fmt.Errorf("failed to cancel upload task: %w", err)
	}

	// 清理分片文件
	go s.cleanupChunks(taskID)

	log.Printf("[UploadService.CancelUpload] Upload task cancelled: %s", taskID)
	return nil
}

// CleanupStaleTasks 清理过期的上传任务
func (s *UploadService) CleanupStaleTasks(maxAge time.Duration) error {
	cutoff := time.Now().Add(-maxAge)

	var tasks []model.UploadTask
	if err := s.db.Where("status IN ? AND updated_at < ?", []string{"pending", "uploading"}, cutoff).Find(&tasks).Error; err != nil {
		log.Printf("[UploadService.CleanupStaleTasks] Failed to find stale tasks: %v", err)
		return err
	}

	for _, task := range tasks {
		task.Status = "cancelled"
		if err := s.db.Save(&task).Error; err != nil {
			log.Printf("[UploadService.CleanupStaleTasks] Failed to cancel task %s: %v", task.ID, err)
			continue
		}
		s.cleanupChunks(task.ID)
		log.Printf("[UploadService.CleanupStaleTasks] Cleaned up stale task: %s", task.ID)
	}

	return nil
}
