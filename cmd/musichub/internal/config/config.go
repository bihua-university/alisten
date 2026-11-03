package config

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/bihua-university/alisten/internal/naconfig"
	"github.com/bihua-university/alisten/internal/nacos"
	pb "github.com/bihua-university/alisten/internal/proto/nacos"
)

var dataID = os.Getenv("MUSICHUB_CONFIG")

type File struct {
	Music Music `json:"music"`
}

type Music struct {
	Cookie string `json:"cookie"`
}

var MusicConfig = naconfig.Json[File](dataID)

var publishMu sync.Mutex

// SetCookie updates the runtime snapshot before publishing the complete Nacos document.
func SetCookie(ctx context.Context, cookie string) error {
	publishMu.Lock()
	defer publishMu.Unlock()

	current := MusicConfig.Get()
	current.Music.Cookie = cookie
	MusicConfig.Update(current)

	reply, err := nacos.GetConfig(ctx, &pb.ConfigRequest{DataId: dataID})
	if err != nil {
		return err
	}
	var document map[string]any
	if err := json.Unmarshal([]byte(reply.Content), &document); err != nil {
		return err
	}
	if document == nil {
		return fmt.Errorf("Nacos config %s must be a JSON object", dataID)
	}
	music, _ := document["music"].(map[string]any)
	if music == nil {
		music = map[string]any{}
		document["music"] = music
	}
	music["cookie"] = cookie
	content, err := json.MarshalIndent(document, "", "    ")
	if err != nil {
		return err
	}
	_, err = nacos.PublishConfig(ctx, &pb.PublishConfigRequest{DataId: dataID, Content: string(content)})
	return err
}
