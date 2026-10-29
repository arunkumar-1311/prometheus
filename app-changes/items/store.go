// Package items is a small in-memory CRUD API (create, read, update, delete)
// instrumented with Prometheus business metrics.
package items

import (
	"errors"
	"sort"
	"sync"
	"time"
)

var ErrNotFound = errors.New("item not found")

type Item struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Price     float64   `json:"price"`
	Quantity  int       `json:"quantity"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Input is the request body for create and update.
type Input struct {
	Name     string  `json:"name" binding:"required"`
	Price    float64 `json:"price" binding:"gte=0"`
	Quantity int     `json:"quantity" binding:"gte=0"`
}

// Store is a concurrency-safe in-memory store. Data is lost on restart.
type Store struct {
	mu    sync.RWMutex
	items map[int]Item
	next  int
}

func NewStore() *Store {
	return &Store{items: make(map[int]Item), next: 1}
}

func (s *Store) Create(in Input) Item {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	it := Item{ID: s.next, Name: in.Name, Price: in.Price, Quantity: in.Quantity, CreatedAt: now, UpdatedAt: now}
	s.items[it.ID] = it
	s.next++
	return it
}

// List returns all items ordered by ID.
func (s *Store) List() []Item {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Item, 0, len(s.items))
	for _, it := range s.items {
		out = append(out, it)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (s *Store) Get(id int) (Item, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	it, ok := s.items[id]
	if !ok {
		return Item{}, ErrNotFound
	}
	return it, nil
}

func (s *Store) Update(id int, in Input) (Item, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	it, ok := s.items[id]
	if !ok {
		return Item{}, ErrNotFound
	}
	it.Name, it.Price, it.Quantity, it.UpdatedAt = in.Name, in.Price, in.Quantity, time.Now().UTC()
	s.items[id] = it
	return it, nil
}

func (s *Store) Delete(id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.items[id]; !ok {
		return ErrNotFound
	}
	delete(s.items, id)
	return nil
}

func (s *Store) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.items)
}
