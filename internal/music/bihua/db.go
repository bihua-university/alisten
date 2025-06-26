package bihua

import (
	"log"

	"github.com/bihua-university/alisten/internal/base"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// DB holds the database connection
var DB *gorm.DB

// InitDB initializes the database connection
func InitDB() {
	dsn := base.Config.Pgsql
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

// SaveNeteaseMusic saves the music data retrieved from Netease to the database
func SaveNeteaseMusic(music gin.H) error {
	if DB == nil {
		return nil // Database not initialized, skip saving
	}

	musicModel := MusicModel{
		MusicID:    music["id"].(string),
		Name:       music["name"].(string),
		Artist:     music["artist"].(string),
		AlbumName:  music["album"].(gin.H)["name"].(string),
		PictureURL: music["pictureUrl"].(string),
		Duration:   music["duration"].(int64),
		URL:        music["url"].(string),
		Lyric:      music["lyric"].(string),
	}

	// Check if music already exists in database
	var existingMusic MusicModel
	result := DB.Where("music_id = ?", musicModel.MusicID).First(&existingMusic)

	if result.Error == nil {
		// Music exists, update the record
		existingMusic.URL = musicModel.URL // Update URL as it might have changed
		existingMusic.PictureURL = musicModel.PictureURL
		existingMusic.Lyric = musicModel.Lyric
		return DB.Save(&existingMusic).Error
	} else if result.Error == gorm.ErrRecordNotFound {
		// Music doesn't exist, create new record
		return DB.Create(&musicModel).Error
	} else {
		// Other database error
		return result.Error
	}
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

// ConvertToGinH converts a MusicModel to the gin.H format used by the API
func ConvertToGinH(music *MusicModel) gin.H {
	return gin.H{
		"type":       "music",
		"id":         music.MusicID,
		"url":        music.URL,
		"pictureUrl": music.PictureURL,
		"duration":   music.Duration,
		"lyric":      music.Lyric,
		"artist":     music.Artist,
		"name":       music.Name,
		"album": gin.H{
			"name": music.AlbumName,
		},
		"playCount": music.PlayCount,
	}
}
