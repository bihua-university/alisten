package config

import (
	"os"

	"github.com/bihua-university/alisten/internal/naconfig"
)

type File struct {
	Persist []PersistHouse `json:"persist"`
}

type PersistHouse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Desc     string `json:"desc"`
	Password string `json:"password"`
}

var ServerConfig = naconfig.Json[File](os.Getenv("LIVEHOUSE_API_CONFIG"))
