package handler

import (
	"log/slog"
	"sync"

	"github.com/google/uuid"
)

type Broker struct {
	mu      sync.RWMutex
	clients map[uuid.UUID]map[chan string]bool
}

func NewBroker() *Broker {
	return &Broker{
		clients: make(map[uuid.UUID]map[chan string]bool),
	}
}

func (b *Broker) AddClient(choregroupID uuid.UUID, client chan string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.clients[choregroupID]; !ok {
		b.clients[choregroupID] = make(map[chan string]bool)
	}
	b.clients[choregroupID][client] = true
	slog.Info("Client connected to SSE", "choregroup_id", choregroupID, "total_clients", len(b.clients[choregroupID]))
}

func (b *Broker) RemoveClient(choregroupID uuid.UUID, client chan string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if group, ok := b.clients[choregroupID]; ok {
		delete(group, client)
		if len(group) == 0 {
			delete(b.clients, choregroupID)
			slog.Info("All clients disconnected from SSE group", "choregroup_id", choregroupID)
		} else {
			slog.Info("Client disconnected from SSE", "choregroup_id", choregroupID, "remaining_clients", len(group))
		}
	}
}

func (b *Broker) Broadcast(choregroupID uuid.UUID, event string) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if group, ok := b.clients[choregroupID]; ok {
		slog.Info("Broadcasting event", "choregroup_id", choregroupID, "event", event, "clients", len(group))
		for client := range group {
			select {
			case client <- event:
			default:
				// Channel full, drop message
			}
		}
	}
}
