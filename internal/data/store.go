package data

import "gorm.io/gorm"

type PartyStore struct {
	db *gorm.DB
}

func NewPartyStore(db *gorm.DB) *PartyStore {
	return &PartyStore{
		db: db,
	}
}

func (s *PartyStore) Create(party *Party) error {
	return s.db.Create(party).Error
}

func (s *PartyStore) Get(id string) (*Party, error) {
	var party Party
	err := s.db.Where("id = ?", id).First(&party).Error
	return &party, err
}

func (s *PartyStore) Update(party *Party) error {
	return s.db.Save(party).Error
}

func (s *PartyStore) Delete(party *Party) error {
	return s.db.Delete(party).Error
}
