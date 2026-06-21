package models

type Action struct {
	ID uint `gorm:"primary_key" json:"id"  xbvrbackup:"-"`

	SceneID       string `json:"scene_id" xbvrbackup:"scene_id"`
	ActionType    string `json:"action_type" xbvrbackup:"action_type"`
	ChangedColumn string `json:"changed_column" xbvrbackup:"changed_column"`
	NewValue      string `json:"new_value" sql:"type:text;" xbvrbackup:"new_value"`
}

func (a *Action) GetIfExist(id uint) error {
	db, _ := GetDB()
	defer db.Close()

	return db.Where(&Action{ID: id}).First(a).Error
}

func (a *Action) Save() {
	db, _ := GetDB()
	defer db.Close()

	SaveWithRetry(db, a)
}

func AddAction(sceneID string, actionType string, changedColumn string, newValue string) {
	action := Action{
		SceneID:       sceneID,
		ActionType:    actionType,
		ChangedColumn: changedColumn,
		NewValue:      newValue,
	}

	action.Save()
}
