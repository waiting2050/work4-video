package model

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID         string         `gorm:"primaryKey;type:varchar(255)" json:"id"`
	Username   string         `gorm:"uniqueIndex;type:varchar(255)" json:"username"`
	Password   string         `gorm:"type:varchar(255)" json:"-"` // -忽略密码
	AvatarURL  string         `gorm:"type:varchar(512)" json:"avatar_url"`
	MFASecret  string         `gorm:"type:varchar(255)" json:"-"`
	MFAEnabled bool           `gorm:"default:false" json:"mfa_enabled"`
	CreatedAt  time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"deleted_at"`
}

type Video struct {
	ID           string         `gorm:"primaryKey;type:varchar(255)" json:"id"`
	UserID       string         `gorm:"index;type:varchar(255)" json:"user_id"`
	VideoURL     string         `gorm:"type:varchar(512)" json:"video_url"`
	CoverURL     string         `gorm:"type:varchar(512)" json:"cover_url"`
	Title        string         `gorm:"type:varchar(255)" json:"title"`
	Description  string         `gorm:"type:text" json:"description"`
	VisitCount   int            `gorm:"default:0" json:"visit_count"`
	LikeCount    int            `gorm:"default:0" json:"like_count"`
	CommentCount int            `gorm:"default:0" json:"comment_count"`
	CreatedAt    time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"deleted_at"`
}

type Comment struct {
	ID         string         `gorm:"primaryKey;type:varchar(255)" json:"id"`
	VideoID    string         `gorm:"index;type:varchar(255)" json:"video_id"`
	UserID     string         `gorm:"index;type:varchar(255)" json:"user_id"`
	ParentID   string         `gorm:"type:varchar(255)" json:"parent_id"`
	Content    string         `gorm:"type:text" json:"content"`
	LikeCount  int            `gorm:"default:0" json:"like_count"`
	ChildCount int            `gorm:"default:0" json:"child_count"`
	CreatedAt  time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"deleted_at"`
}

type Follow struct {
	ID         string         `gorm:"primaryKey;type:varchar(255)" json:"id"`
	FollowerID string         `gorm:"uniqueIndex:idx_follow_user;type:varchar(255)" json:"follower_id"`
	FolloweeID string         `gorm:"uniqueIndex:idx_follow_user;type:varchar(255)" json:"followee_id"`
	CreatedAt  time.Time      `gorm:"autoCreateTime" json:"created_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"deleted_at"`
}

type Like struct {
	ID        string         `gorm:"primaryKey;type:varchar(255)" json:"id"`
	UserID    string         `gorm:"uniqueIndex:idx_like_user_video;type:varchar(255)" json:"user_id"`
	VideoID   string         `gorm:"uniqueIndex:idx_like_user_video;type:varchar(255)" json:"video_id"`
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at"`
}

// UploadTask 分片上传任务
type UploadTask struct {
	ID             string         `gorm:"primaryKey;type:varchar(255)" json:"id"`
	UserID         string         `gorm:"index;type:varchar(255)" json:"user_id"`
	FileName       string         `gorm:"type:varchar(255)" json:"file_name"`
	FileSize       int64          `gorm:"type:bigint" json:"file_size"`
	ChunkSize      int            `gorm:"type:int" json:"chunk_size"`
	TotalChunks    int            `gorm:"type:int" json:"total_chunks"`
	UploadedChunks int            `gorm:"type:int;default:0" json:"uploaded_chunks"`
	Status         string         `gorm:"type:varchar(50);default:'pending'" json:"status"` // pending, uploading, completed, failed, cancelled
	FileURL        string         `gorm:"type:varchar(512)" json:"file_url"`
	Title          string         `gorm:"type:varchar(255)" json:"title"`
	Description    string         `gorm:"type:text" json:"description"`
	CreatedAt      time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"deleted_at"`
}

// UploadChunk 已上传的分片信息
type UploadChunk struct {
	ID         string         `gorm:"primaryKey;type:varchar(255)" json:"id"`
	TaskID     string         `gorm:"uniqueIndex:idx_chunk_task_index;type:varchar(255)" json:"task_id"`
	ChunkIndex int            `gorm:"uniqueIndex:idx_chunk_task_index" json:"chunk_index"`
	ChunkSize  int64          `gorm:"type:bigint" json:"chunk_size"`
	Checksum   string         `gorm:"type:varchar(64)" json:"checksum"`
	CreatedAt  time.Time      `gorm:"autoCreateTime" json:"created_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"deleted_at"`
}

// ChatMessage 聊天消息记录
type ChatMessage struct {
	ID        string         `gorm:"primaryKey;type:varchar(255)" json:"id"`           // 消息ID
	SenderID  string         `gorm:"index;type:varchar(255)" json:"sender_id"`         // 发送者ID
	RoomType  string         `gorm:"index;type:varchar(50)" json:"room_type"`         // 房间类型："private"私聊 | "group"群聊
	RoomID    string         `gorm:"index;type:varchar(255)" json:"room_id"`           // 房间ID
	Content   string         `gorm:"type:text" json:"content"`                         // 消息内容
	ReadBy    []string       `gorm:"type:json" json:"read_by"`                         // 已读用户ID列表
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`                 // 创建时间
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at"`                         // 删除时间（软删除）
}

// ChatRoom 聊天房间
type ChatRoom struct {
	ID          string         `gorm:"primaryKey;type:varchar(255)" json:"id"`         // 房间ID
	Type        string         `gorm:"type:varchar(50)" json:"type"`                 // 房间类型："private"私聊 | "group"群聊
	Name        string         `gorm:"type:varchar(255)" json:"name"`                 // 群聊名称
	Members     []string       `gorm:"type:json" json:"members"`                     // 群聊成员ID列表
	CreatedAt   time.Time      `gorm:"autoCreateTime" json:"created_at"`               // 创建时间
	UpdatedAt   time.Time      `gorm:"autoUpdateTime" json:"updated_at"`               // 更新时间
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"deleted_at"`                       // 删除时间（软删除）
}

func autoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&User{},
		&Video{},
		&Comment{},
		&Follow{},
		&Like{},
		&UploadTask{},
		&UploadChunk{},
		&ChatMessage{},
		&ChatRoom{},
	)
}
