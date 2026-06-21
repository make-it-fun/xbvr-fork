package models

import (
	"time"
)

// Playlist data model
type Playlist struct {
	ID        uint      `gorm:"primary_key" json:"id" xbvrbackup:"-"`
	CreatedAt time.Time `json:"-" xbvrbackup:"-"`
	UpdatedAt time.Time `json:"-" xbvrbackup:"-"`

	Name         string `json:"name" xbvrbackup:"name"`
	Ordering     int    `json:"ordering" xbvrbackup:"ordering"`
	IsSystem     bool   `json:"is_system" xbvrbackup:"is_system"`
	IsDeoEnabled bool   `json:"is_deo_enabled" xbvrbackup:"is_deo_enabled"`
	IsSmart      bool   `json:"is_smart" xbvrbackup:"is_smart"`
	PlaylistType string `json:"playlist_type" xbvrbackup:"playlist_type"`
	SearchParams string `json:"search_params" sql:"type:text;" xbvrbackup:"search_params"`
}

func (o *Playlist) Save() error {
	db, _ := GetDB()
	defer db.Close()

	return SaveWithRetry(db, o)
}
