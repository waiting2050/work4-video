package service

import (
	"log"
	"path/filepath"
	"strconv"
	"strings"
)

// UploadStrategy 上传策略类型
type UploadStrategy string

const (
	// StrategyNormal 普通上传
	StrategyNormal UploadStrategy = "normal"
	// StrategyChunked 分片上传
	StrategyChunked UploadStrategy = "chunked"
)

// UploadStrategyConfig 上传策略配置
type UploadStrategyConfig struct {
	// 文件大小阈值（字节）
	SmallFileThreshold int64
	LargeFileThreshold int64

	// 网络环境配置
	SlowNetworkThreshold int64 // 慢速网络阈值（KB/s）

	// 分片配置
	DefaultChunkSize     int // 默认分片大小
	SlowNetworkChunkSize int // 慢速网络分片大小

	// 特殊文件类型
	ForceChunkedTypes []string // 强制使用分片的文件类型
	ForceNormalTypes  []string // 强制使用普通上传的文件类型
}

// DefaultUploadStrategyConfig 默认配置
func DefaultUploadStrategyConfig() *UploadStrategyConfig {
	return &UploadStrategyConfig{
		SmallFileThreshold:   100 * 1024 * 1024,  // 100MB
		LargeFileThreshold:   100 * 1024 * 1024,  // 100MB
		SlowNetworkThreshold: 500,                // 500KB/s
		DefaultChunkSize:     5 * 1024 * 1024,    // 5MB
		SlowNetworkChunkSize: 1 * 1024 * 1024,    // 1MB
		ForceChunkedTypes:    []string{".mov", ".mkv", ".avi"},
		ForceNormalTypes:     []string{".gif", ".webp"},
	}
}

// UploadDecisionRequest 上传决策请求
type UploadDecisionRequest struct {
	FileName       string // 文件名
	FileSize       int64  // 文件大小（字节）
	ContentType    string // MIME类型
	NetworkType    string // 网络类型：wifi/4g/5g/unknown
	UserPreference string // 用户偏好：auto/normal/chunked
}

// UploadDecision 上传决策结果
type UploadDecision struct {
	Strategy   UploadStrategy `json:"strategy"`
	ChunkSize  int            `json:"chunk_size"`
	Reason     string         `json:"reason"`
	Threshold  int64          `json:"threshold"`
	CanSwitch  bool           `json:"can_switch"`
}

// UploadStrategyService 上传策略服务
type UploadStrategyService struct {
	config *UploadStrategyConfig
}

// NewUploadStrategyService 创建上传策略服务
func NewUploadStrategyService(config *UploadStrategyConfig) *UploadStrategyService {
	if config == nil {
		config = DefaultUploadStrategyConfig()
	}
	return &UploadStrategyService{config: config}
}

// DecideUploadStrategy 决策上传策略
func (s *UploadStrategyService) DecideUploadStrategy(req *UploadDecisionRequest) *UploadDecision {
	// 1. 检查用户强制偏好
	if req.UserPreference == "normal" {
		return &UploadDecision{
			Strategy:  StrategyNormal,
			Reason:    "用户强制选择普通上传",
			CanSwitch: true,
		}
	}
	if req.UserPreference == "chunked" {
		return &UploadDecision{
			Strategy:  StrategyChunked,
			ChunkSize: s.config.DefaultChunkSize,
			Reason:    "用户强制选择分片上传",
			CanSwitch: true,
		}
	}

	// 2. 检查文件类型强制规则
	ext := strings.ToLower(filepath.Ext(req.FileName))
	for _, forcedType := range s.config.ForceChunkedTypes {
		if ext == forcedType {
			return &UploadDecision{
				Strategy:  StrategyChunked,
				ChunkSize: s.config.DefaultChunkSize,
				Reason:    "文件类型强制使用分片上传: " + ext,
				CanSwitch: false,
			}
		}
	}
	for _, forcedType := range s.config.ForceNormalTypes {
		if ext == forcedType {
			return &UploadDecision{
				Strategy:  StrategyNormal,
				Reason:    "文件类型强制使用普通上传: " + ext,
				CanSwitch: false,
			}
		}
	}

	// 3. 根据文件大小判断
	if req.FileSize < s.config.SmallFileThreshold {
		// 小文件：根据网络环境决定是否使用分片
		if s.shouldUseChunkedForNetwork(req.NetworkType, req.FileSize) {
			chunkSize := s.getChunkSizeForNetwork(req.NetworkType)
			return &UploadDecision{
				Strategy:  StrategyChunked,
				ChunkSize: chunkSize,
				Reason:    "小文件但在弱网环境，使用分片上传提高成功率",
				Threshold: s.config.SmallFileThreshold,
				CanSwitch: true,
			}
		}

		return &UploadDecision{
			Strategy:  StrategyNormal,
			Reason:    "小文件（<100MB），网络良好，使用普通上传",
			Threshold: s.config.SmallFileThreshold,
			CanSwitch: true,
		}
	}

	// 4. 大文件：必须使用分片上传
	chunkSize := s.getChunkSizeForNetwork(req.NetworkType)
	return &UploadDecision{
		Strategy:  StrategyChunked,
		ChunkSize: chunkSize,
		Reason:    "大文件（≥100MB），使用分片上传",
		Threshold: s.config.LargeFileThreshold,
		CanSwitch: false,
	}
}

// shouldUseChunkedForNetwork 根据网络环境判断是否使用分片
func (s *UploadStrategyService) shouldUseChunkedForNetwork(networkType string, fileSize int64) bool {
	switch networkType {
	case "wifi", "5g":
		return false
	case "4g":
		return fileSize > 10*1024*1024
	case "3g", "2g", "slow":
		return fileSize > 1*1024*1024
	default:
		return fileSize > 5*1024*1024
	}
}

// getChunkSizeForNetwork 根据网络环境获取分片大小
func (s *UploadStrategyService) getChunkSizeForNetwork(networkType string) int {
	switch networkType {
	case "wifi", "5g":
		return s.config.DefaultChunkSize
	case "4g":
		return 2 * 1024 * 1024
	case "3g", "2g", "slow":
		return s.config.SlowNetworkChunkSize
	default:
		return s.config.DefaultChunkSize
	}
}

// GetUploadRecommendation 获取上传建议
func (s *UploadStrategyService) GetUploadRecommendation(fileSize int64, fileName string) map[string]interface{} {
	ext := strings.ToLower(filepath.Ext(fileName))

	recommendation := map[string]interface{}{
		"file_size":       fileSize,
		"file_size_human": formatFileSize(fileSize),
		"file_type":       ext,
	}

	if fileSize < 10*1024*1024 {
		recommendation["size_category"] = "small"
		recommendation["size_description"] = "小文件（<10MB）"
	} else if fileSize < 100*1024*1024 {
		recommendation["size_category"] = "medium"
		recommendation["size_description"] = "中等文件（10-100MB）"
	} else {
		recommendation["size_category"] = "large"
		recommendation["size_description"] = "大文件（≥100MB）"
	}

	if fileSize >= s.config.LargeFileThreshold {
		recommendation["recommended_strategy"] = "chunked"
		recommendation["reason"] = "文件较大，建议使用分片上传以获得更好的稳定性和断点续传支持"
		recommendation["chunk_size"] = s.config.DefaultChunkSize
		recommendation["estimated_chunks"] = (fileSize + int64(s.config.DefaultChunkSize) - 1) / int64(s.config.DefaultChunkSize)
	} else {
		recommendation["recommended_strategy"] = "normal"
		recommendation["reason"] = "文件较小，使用普通上传更简单快速"
	}

	return recommendation
}

// formatFileSize 格式化文件大小
func formatFileSize(bytes int64) string {
	if bytes == 0 {
		return "0 B"
	}
	units := []string{"B", "KB", "MB", "GB", "TB"}
	exp := 0
	size := float64(bytes)
	for size >= 1024 && exp < len(units)-1 {
		size /= 1024
		exp++
	}

	if size == float64(int64(size)) {
		return formatInt(int64(size)) + " " + units[exp]
	}
	return formatFloat(size) + " " + units[exp]
}

// formatFloat 格式化浮点数
func formatFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', 2, 64)
}

// formatInt 格式化整数
func formatInt(n int64) string {
	return strconv.FormatInt(n, 10)
}

// LogUploadDecision 记录上传决策日志
func (s *UploadStrategyService) LogUploadDecision(req *UploadDecisionRequest, decision *UploadDecision) {
	log.Printf("[UploadStrategy] File: %s, Size: %s, Network: %s -> Strategy: %s, Reason: %s",
		req.FileName,
		formatFileSize(req.FileSize),
		req.NetworkType,
		decision.Strategy,
		decision.Reason,
	)
}
