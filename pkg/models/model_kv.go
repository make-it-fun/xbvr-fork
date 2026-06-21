package models

type KV struct {
	Key   string `json:"key" gorm:"primary_key"`
	Value string `json:"value" sql:"type:text;"`
}

func (o *KV) Save() {
	db, _ := GetDB()
	defer db.Close()

	SaveWithRetry(db, o)
}

func (o *KV) Delete() {
	db, _ := GetDB()
	db.Delete(&o)
	db.Close()
}
