package bihua

import (
	"testing"
)

func TestName(t *testing.T) {
	InitDB()
	InsertMusic(&MusicModel{
		MusicID:    "M5000047IVyx34aoK0",
		Name:       "皇后大道东",
		Artist:     "罗大佑&蒋志光",
		AlbumName:  "皇后大道东",
		PictureURL: "https://bkimg.cdn.bcebos.com/pic/2f738bd4b31c87019507546e2b7f9e2f0608ff81?x-bce-process=image/format,f_auto/quality,Q_70/resize,m_lfit,limit_1,w_536",
		Duration:   250 * 1000,
		URL:        "https://bihua-oss.ggemo.com/alisten/M5000047IVyx34aoK0.mp3",
		Lyric:      "",
		PlayCount:  1,
	})
}
