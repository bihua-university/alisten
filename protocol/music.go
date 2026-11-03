package protocol

import (
	"github.com/bihua-university/alisten/internal/auth"
	"github.com/bihua-university/alisten/internal/music"
	pb "github.com/bihua-university/alisten/internal/proto/musichub"
)

func UserData(u auth.User) User { return User{Name: u.Name, EmailHash: u.Email} }

func Members(users []auth.User) MembersData {
	data := MembersData{Users: make([]User, 0, len(users)), OnlineCount: int64(len(users))}
	for _, u := range users {
		data.Users = append(data.Users, UserData(u))
	}
	return data
}

func SongData(s pb.Song) Song {
	return Song{
		ID:         s.Id,
		Source:     Source(music.ParseSource(s.Source).String()),
		Name:       s.Name,
		Artist:     s.Artist,
		Album:      s.Album,
		DurationMS: s.Duration,
		PictureURL: s.PictureUrl,
		WebURL:     s.WebUrl,
	}
}

func PlayableData(s pb.Song) PlayableSong {
	return PlayableSong{
		ID:         s.Id,
		Source:     Source(music.ParseSource(s.Source).String()),
		Name:       s.Name,
		Artist:     s.Artist,
		Album:      s.Album,
		DurationMS: s.Duration,
		PictureURL: s.PictureUrl,
		WebURL:     s.WebUrl,
		URL:        s.Url,
		Lyric:      s.Lyric,
	}
}

func SearchData(items []*pb.Music, total int64) MusicSearchResult {
	data := MusicSearchResult{Items: make([]Song, 0, len(items)), TotalSize: total}
	for _, s := range items {
		if s == nil {
			continue
		}
		source := music.Source(s.Source)
		data.Items = append(data.Items, Song{
			ID:         s.Id,
			Source:     Source(source.String()),
			Name:       s.Name,
			Artist:     s.Artist,
			Album:      s.Album,
			DurationMS: s.Duration,
			PictureURL: s.Cover,
			WebURL:     source.WebURL(s.Id),
		})
	}
	return data
}

func PlaylistData(items []*pb.Playlist, total int64, source string) PlaylistSearchResult {
	data := PlaylistSearchResult{Items: make([]Playlist, 0, len(items)), TotalSize: total}
	for _, s := range items {
		if s == nil {
			continue
		}
		data.Items = append(data.Items, Playlist{
			ID:          s.Id,
			Source:      Source(music.ParseSource(source).String()),
			Name:        s.Name,
			PictureURL:  s.PictureUrl,
			Description: s.Desc,
			Creator:     s.Creator, CreatorID: s.CreatorUid,
			PlayCount: s.PlayCount,
			SongCount: s.SongCount,
		})
	}
	return data
}
