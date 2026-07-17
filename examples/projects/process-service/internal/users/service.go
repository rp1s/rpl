package users

import (
	"context"
	"fmt"
	"sort"
	"sync"

	model "example.com/rpl/process-service/generated/user"
)

type Service struct {
	mu    sync.RWMutex
	items map[int]model.User
}

func NewService() *Service {
	return &Service{items: make(map[int]model.User)}
}

func (service *Service) Put(_ context.Context, user model.User) (model.User, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	service.items[user.Id] = user
	return user, nil
}

func (service *Service) List(context.Context) ([]model.User, error) {
	service.mu.RLock()
	defer service.mu.RUnlock()
	ids := make([]int, 0, len(service.items))
	for id := range service.items {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	items := make([]model.User, 0, len(ids))
	for _, id := range ids {
		items = append(items, service.items[id])
	}
	return items, nil
}

func (service *Service) GetByID(_ context.Context, id int) (model.User, error) {
	service.mu.RLock()
	defer service.mu.RUnlock()
	user, ok := service.items[id]
	if !ok {
		return model.User{}, fmt.Errorf("user %d not found", id)
	}
	return user, nil
}

func (service *Service) Delete(_ context.Context, id int) error {
	service.mu.Lock()
	defer service.mu.Unlock()
	delete(service.items, id)
	return nil
}

func (service *Service) Health(context.Context) (string, error) {
	return "ok", nil
}

func (service *Service) Label(ctx context.Context, id int) (string, error) {
	user, err := service.GetByID(ctx, id)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d:%s", user.Id, user.Name), nil
}
