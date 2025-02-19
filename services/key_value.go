package services

import (
	"github.com/muety/wakapi/config"
	"github.com/muety/wakapi/models"
	"github.com/muety/wakapi/repositories"
)

type KeyValueService struct {
	config     *config.Config
	repository repositories.IKeyValueRepository
}

func NewKeyValueService(keyValueRepo repositories.IKeyValueRepository) *KeyValueService {
	return &KeyValueService{
		config:     config.Get(),
		repository: keyValueRepo,
	}
}

func (srv *KeyValueService) GetString(key string) (*models.KeyStringValue, error) {
	return srv.repository.GetString(key)
}

func (srv *KeyValueService) GetByPrefix(prefix string) ([]*models.KeyStringValue, error) {
	return srv.repository.Search(prefix + "%")
}

func (srv *KeyValueService) MustGetString(key string) *models.KeyStringValue {
	kv, err := srv.repository.GetString(key)
	if err != nil {
		return &models.KeyStringValue{
			Key:   key,
			Value: "",
		}
	}
	return kv
}

func (srv *KeyValueService) PutString(kv *models.KeyStringValue) error {
	return srv.repository.PutString(kv)
}

func (srv *KeyValueService) DeleteString(key string) error {
	return srv.repository.DeleteString(key)
}

func (srv *KeyValueService) ReplaceKeySuffix(suffixOld, suffixNew string) error {
	return srv.repository.ReplaceKeySuffix(suffixOld, suffixNew)
}
