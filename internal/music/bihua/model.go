package bihua

import (
	"gorm.io/gorm"
)

// MusicModel represents the music data in the database
type MusicModel struct {
	gorm.Model
	MusicID    string `gorm:"uniqueIndex;not null"`
	Name       string `gorm:"index;not null"`
	Artist     string `gorm:"index"`
	AlbumName  string
	PictureURL string
	WebURL     string // 添加网址字段
	Duration   int64
	URL        string
	Lyric      string `gorm:"type:text"`
	PlayCount  int    `gorm:"default:0"`
}
