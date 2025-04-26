package main

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/wdvxdr1123/alisten/music"
)

type Vote struct {
	UserID string `json:"user_id"`
	Vote   bool   `json:"vote"`
}

func deleteMusic(c *gin.Context) {
	var request struct {
		HouseID string      `json:"house_id"`
		Music   music.Music `json:"music"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	house, exists := houses[request.HouseID]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "房间未找到"})
		return
	}

	house.Mu.Lock()
	defer house.Mu.Unlock()

	for i, m := range house.Playlist {
		if m.id == request.Music.ID {
			house.Playlist = append(house.Playlist[:i], house.Playlist[i+1:]...)
			c.JSON(http.StatusOK, gin.H{"message": "歌曲已删除"})
			return
		}
	}

	c.JSON(http.StatusNotFound, gin.H{"error": "歌曲未找到"})
}

func topMusic(c *gin.Context) {
	var request struct {
		HouseID string `json:"house_id"`
		MusicID string `json:"music_id"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	house, exists := houses[request.HouseID]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "房间未找到"})
		return
	}

	house.Mu.Lock()
	defer house.Mu.Unlock()

	// var foundMusic *Music
	var foundIndex int = -1

	for i, m := range house.Playlist {
		if m.id == request.MusicID {
			// foundMusic = &m
			foundIndex = i
			break
		}
	}

	if foundIndex == -1 {
		c.JSON(http.StatusNotFound, gin.H{"error": "歌曲未找到"})
		return
	}

	// 将歌曲移动到播放列表顶部
	// house.Playlist = append([]Music{*foundMusic}, append(house.Playlist[:foundIndex], house.Playlist[foundIndex+1:]...)...)

	c.JSON(http.StatusOK, gin.H{"message": "歌曲已置顶"})
}
