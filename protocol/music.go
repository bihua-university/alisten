package protocol

import (
	"github.com/bihua-university/alisten/internal/auth"
	"github.com/bihua-university/alisten/internal/music"
)

func UserData(u auth.User) User { return User{Name: u.Name, EmailHash: u.Email} }

func Members(users []auth.User) MembersData {
	data := MembersData{Users: make([]User, 0, len(users)), OnlineCount: int64(len(users))}
	for _, u := range users {
		data.Users = append(data.Users, UserData(u))
	}
	return data
}

func SongData(s music.Song) Song {
	return Song{
		ID:         s.ID,
		Source:     Source(music.ParseSource(s.Source).String()),
		Name:       s.Name,
		Artist:     s.Artist,
		Album:      s.Album,
		DurationMS: s.Duration,
		PictureURL: s.PictureURL,
		WebURL:     s.WebURL,
	}
}

func PlayableData(s music.Song) PlayableSong {
	return PlayableSong{
		ID:         s.ID,
		Source:     Source(music.ParseSource(s.Source).String()),
		Name:       s.Name,
		Artist:     s.Artist,
		Album:      s.Album,
		DurationMS: s.Duration,
		PictureURL: s.PictureURL,
		WebURL:     s.WebURL,
		URL:        s.URL,
		Lyric:      s.Lyric,
	}
}

func SearchData(items []*music.Music, total int64) MusicSearchResult {
	data := MusicSearchResult{Items: make([]Song, 0, len(items)), TotalSize: total}
	for _, s := range items {
		if s == nil {
			continue
		}
		data.Items = append(data.Items, Song{
			ID:         s.ID,
			Source:     Source(s.Source.String()),
			Name:       s.Name,
			Artist:     s.Artist,
			Album:      s.Album,
			DurationMS: s.Duration,
			PictureURL: s.Cover,
			WebURL:     s.Source.WebURL(s.ID),
		})
	}
	return data
}

func PlaylistData(items []*music.Playlist, total int64, source string) PlaylistSearchResult {
	data := PlaylistSearchResult{Items: make([]Playlist, 0, len(items)), TotalSize: total}
	for _, s := range items {
		if s == nil {
			continue
		}
		data.Items = append(data.Items, Playlist{
			ID:          s.ID,
			Source:      Source(music.ParseSource(source).String()),
			Name:        s.Name,
			PictureURL:  s.PictureURL,
			Description: s.Desc,
			Creator:     s.Creator, CreatorID: s.CreatorUid,
			PlayCount: s.PlayCount,
			SongCount: s.SongCount,
		})
	}
	return data
}
