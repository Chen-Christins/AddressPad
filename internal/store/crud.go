package store

import (
	"fmt"

	"addresspad/internal/model"
)

// Store holds the last successfully persisted configuration. Mutations only
// commit to memory after Save succeeds. A Store is intended for one UI goroutine.
type Store struct {
	path   string
	config model.Config
}

func Open(path string) (*Store, error) {
	c, err := Load(path)
	if err != nil {
		return nil, err
	}
	return &Store{path: path, config: c}, nil
}

func (s *Store) Config() model.Config { return s.config.Clone() }

// Replace commits a complete configuration in a single transaction.
func (s *Store) Replace(c model.Config) error { return s.persist(c) }

func (s *Store) persist(c model.Config) error {
	c = c.Clone()
	c.Normalize()
	if err := Save(s.path, c); err != nil {
		return err
	}
	s.config = c
	return nil
}

func (s *Store) Add(server model.Server) error {
	c := s.config.Clone()
	c.Servers = append(c.Servers, server)
	return s.persist(c)
}

func (s *Store) index(id string) (int, error) {
	for i, server := range s.config.Servers {
		if server.ID == id {
			return i, nil
		}
	}
	return -1, fmt.Errorf("server id %q not found", id)
}

func (s *Store) Update(id string, server model.Server) error {
	i, err := s.index(id)
	if err != nil {
		return err
	}
	c := s.config.Clone()
	server.ID = id
	c.Servers[i] = server
	return s.persist(c)
}

func (s *Store) Delete(id string) error {
	i, err := s.index(id)
	if err != nil {
		return err
	}
	c := s.config.Clone()
	c.Servers = append(c.Servers[:i], c.Servers[i+1:]...)
	return s.persist(c)
}

func (s *Store) ToggleFavorite(id string) error {
	i, err := s.index(id)
	if err != nil {
		return err
	}
	c := s.config.Clone()
	c.Servers[i].Favorite = !c.Servers[i].Favorite
	return s.persist(c)
}
