package models

import (
	"time"
)

type History struct {
	ID        uint      `gorm:"primary_key" json:"id" xbvrbackup:"-"`
	CreatedAt time.Time `json:"-" xbvrbackup:"created_at-"`
	UpdatedAt time.Time `json:"-" xbvrbackup:"updated_at"`

	SceneID   uint      `json:"scene_id" xbvrbackup:"-"`
	TimeStart time.Time `json:"time_start" xbvrbackup:"time_start"`
	TimeEnd   time.Time `json:"time_end" xbvrbackup:"time_end"`
	Duration  float64   `json:"duration" xbvrbackup:"duration"`
}

func (o *History) GetIfExist(id uint) error {
	db, _ := GetDB()
	defer db.Close()

	return db.Where(&History{ID: id}).First(o).Error
}

func (o *History) Save() {
	db, _ := GetDB()
	defer db.Close()

	SaveWithRetry(db, o)
}

func (o *History) Delete() {
	db, _ := GetDB()
	db.Delete(&o)
	db.Close()
}
