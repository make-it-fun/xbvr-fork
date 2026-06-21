package models

import (
	"strings"
	"time"
)

type Site struct {
	ID             string    `gorm:"primary_key" json:"id" xbvrbackup:"-"`
	Name           string    `json:"name"  xbvrbackup:"name"`
	AvatarURL      string    `json:"avatar_url" xbvrbackup:"-"`
	IsBuiltin      bool      `json:"is_builtin" xbvrbackup:"-"`
	IsEnabled      bool      `json:"is_enabled" xbvrbackup:"is_enabled"`
	LastUpdate     time.Time `json:"last_update" xbvrbackup:"-"`
	Subscribed     bool      `json:"subscribed" xbvrbackup:"subscribed"`
	HasScraper     bool      `gorm:"-" json:"has_scraper" xbvrbackup:"-"`
	LimitScraping  bool      `json:"limit_scraping" xbvrbackup:"limit_scraping"`
	MasterSiteID   string    `json:"master_site_id" xbvrbackup:"master_site_id"`
	MatchingParams string    `json:"matching_params" gorm:"size:1000" xbvrbackup:"matching_params"`
	ScrapeStash    bool      `json:"scrape_stash" xbvrbackup:"scrape_stash"`
	SceneCount     int       `gorm:"-" json:"scene_count" xbvrbackup:"-"`
}

func (i *Site) Save() error {
	db, _ := GetDB()
	defer db.Close()

	return SaveWithRetry(db, i)
}

func (i *Site) GetIfExist(id string) error {
	db, _ := GetDB()
	defer db.Close()

	return db.Where(&Site{ID: id}).First(i).Error
}

func InitSites() {
	db, _ := GetDB()
	defer db.Close()

	scrapers := GetScrapers()
	for i := range scrapers {
		if !strings.HasSuffix(scrapers[i].ID, "-single_scene") {
			var st Site
			db.Where(&Site{ID: scrapers[i].ID}).FirstOrCreate(&st)
			st.Name = scrapers[i].Name
			st.AvatarURL = scrapers[i].AvatarURL
			st.IsBuiltin = true
			st.MasterSiteID = scrapers[i].MasterSiteId
			st.Save()
		}
	}
}
