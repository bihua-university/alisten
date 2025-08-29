package bihua

import (
	"fmt"
	"log"

	"github.com/bihua-university/alisten/internal/music"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// DB holds the database connection
var DB *gorm.DB

// InitDB initializes the database connection
func InitDB(dsn string) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("Failed to connect to database")
	}
	DB = db
	// Auto migrate the schema
	err = DB.AutoMigrate(&MusicModel{})
	if err != nil {
		log.Printf("Failed to migrate database: %v", err)
	}
}

// MusicModel represents the music data in the database
type MusicModel struct {
	gorm.Model
	MusicID    string `gorm:"uniqueIndex;not null"`
	Name       string `gorm:"index;not null"`
	Artist     string `gorm:"index"`
	AlbumName  string
	PictureURL string
	Duration   int64
	URL        string
	Lyric      string `gorm:"type:text"`
	PlayCount  int    `gorm:"default:0"`
}

// InsertMusic inserts a single music record into the database
func InsertMusic(music *MusicModel) error {
	if DB == nil {
		return nil // Database not initialized, skip saving
	}

	// Check if music already exists in database
	var existingMusic MusicModel
	result := DB.Where("music_id = ?", music.MusicID).First(&existingMusic)

	if result.Error == nil {
		// Music exists, update the record
		existingMusic.URL = music.URL
		existingMusic.PictureURL = music.PictureURL
		existingMusic.Lyric = music.Lyric
		existingMusic.Name = music.Name
		existingMusic.Artist = music.Artist
		existingMusic.AlbumName = music.AlbumName
		existingMusic.Duration = music.Duration
		return DB.Save(&existingMusic).Error
	} else if result.Error == gorm.ErrRecordNotFound {
		// Music doesn't exist, create new record
		return DB.Create(music).Error
	} else {
		// Other database error
		return result.Error
	}
}

// GetMusicByID retrieves music from the database by ID and source
func GetMusicByID(id string) (*MusicModel, error) {
	if DB == nil {
		return nil, nil // Database not initialized
	}

	var music MusicModel
	result := DB.Where("music_id = ?", id).First(&music)
	if result.Error != nil {
		return nil, result.Error
	}

	return &music, nil
}

// SearchMusic searches for music in the database
func SearchMusicByDB(keyword string, page, pageSize int64) ([]MusicModel, int64, error) {
	if DB == nil {
		return nil, 0, nil // Database not initialized
	}

	var musics []MusicModel
	var total int64

	query := DB.Model(&MusicModel{}).
		Where("name ILIKE ? OR artist ILIKE ?", "%"+keyword+"%", "%"+keyword+"%")

	// Get total count
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Get paginated results
	if err := query.
		Order("play_count DESC").
		Offset(int((page - 1) * pageSize)).
		Limit(int(pageSize)).
		Find(&musics).Error; err != nil {
		return nil, 0, err
	}

	return musics, total, nil
}

// ConvertToMap converts a MusicModel to a map format used by the API
func ConvertToMap(music *MusicModel) map[string]string {
	// 需要从外部导入 GenerateWebURL 函数
	webUrl := ""
	switch {
	case len(music.MusicID) >= 2:
		// 动态生成 WebURL
		if music.MusicID[:2] == "BV" {
			webUrl = fmt.Sprintf("https://www.bilibili.com/video/%s", music.MusicID)
		}
	}

	return map[string]string{
		"type":       "music",
		"id":         music.MusicID,
		"url":        music.URL,
		"webUrl":     webUrl,
		"pictureUrl": music.PictureURL,
		"duration":   fmt.Sprintf("%d", music.Duration),
		"lyric":      music.Lyric,
		"artist":     music.Artist,
		"name":       music.Name,
		"album":      music.AlbumName,
		"playCount":  fmt.Sprintf("%d", music.PlayCount),
	}
}

func ConvertMusicList(musics []MusicModel) []*music.Music {
	var result []*music.Music
	for _, m := range musics {
		s := &music.Music{
			ID:       m.MusicID,
			Name:     m.Name,
			Artist:   m.Artist,
			Album:    m.AlbumName,
			Duration: m.Duration,
		}
		result = append(result, s)
	}
	return result
}
