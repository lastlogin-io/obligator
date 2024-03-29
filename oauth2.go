package obligator

import (
	"context"
	"errors"
	"sync"

	"github.com/lestrrat-go/jwx/v2/jwk"
)

type OAuth2MetadataManager struct {
	storage        Storage
	oidcConfigs    map[string]*OAuth2ServerMetadata
	jwksRefreshers map[string]*jwk.Cache
	mut            *sync.Mutex
}

func NewOAuth2MetadataManager(storage Storage) *OAuth2MetadataManager {
	m := &OAuth2MetadataManager{
		storage:     storage,
		oidcConfigs: make(map[string]*OAuth2ServerMetadata),
		mut:         &sync.Mutex{},
	}

	return m
}

func (m *OAuth2MetadataManager) GetMeta(providerId string) (*OAuth2ServerMetadata, error) {
	m.mut.Lock()
	defer m.mut.Unlock()

	if meta, exists := m.oidcConfigs[providerId]; exists {
		return &(*meta), nil
	}

	return nil, errors.New("No such provider")
}

func (m *OAuth2MetadataManager) GetKeyset(providerId string) (jwk.Set, error) {
	m.mut.Lock()
	defer m.mut.Unlock()

	ctx := context.Background()

	keyset, err := m.jwksRefreshers[providerId].Get(ctx, m.oidcConfigs[providerId].JwksUri)
	if err != nil {
		return nil, err
	}

	return keyset, nil
}

func (m *OAuth2MetadataManager) Update() error {
	m.mut.Lock()
	defer m.mut.Unlock()

	m.oidcConfigs = make(map[string]*OAuth2ServerMetadata)
	m.jwksRefreshers = make(map[string]*jwk.Cache)

	ctx := context.Background()

	providers, err := m.storage.GetOAuth2Providers()
	if err != nil {
		return err
	}

	for _, oidcProvider := range providers {
		if !oidcProvider.OpenIDConnect {
			continue
		}

		var err error
		m.oidcConfigs[oidcProvider.ID], err = GetOidcConfiguration(oidcProvider.URI)
		if err != nil {
			return err
		}

		m.jwksRefreshers[oidcProvider.ID] = jwk.NewCache(ctx)
		m.jwksRefreshers[oidcProvider.ID].Register(m.oidcConfigs[oidcProvider.ID].JwksUri)

		_, err = m.jwksRefreshers[oidcProvider.ID].Refresh(ctx, m.oidcConfigs[oidcProvider.ID].JwksUri)
		if err != nil {
			return err
		}
	}

	return nil
}
