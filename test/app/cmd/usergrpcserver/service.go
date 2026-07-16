package main

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"sync"

	usermodel "m/src/user"
	usergrpc "m/src/user/grpc"
)

type inMemoryUserService struct {
	mu    sync.RWMutex
	items map[int]usermodel.User
}

func newInMemoryUserService(seed ...usermodel.User) *inMemoryUserService {
	service := &inMemoryUserService{
		items: make(map[int]usermodel.User, len(seed)),
	}
	for _, user := range seed {
		service.items[user.Id] = user
	}
	return service
}

var _ usergrpc.UserService = (*inMemoryUserService)(nil)

func (service *inMemoryUserService) Put(_ context.Context, user usermodel.User) (usermodel.User, error) {
	service.mu.Lock()
	defer service.mu.Unlock()

	service.items[user.Id] = user
	return user, nil
}

func (service *inMemoryUserService) GetByID(_ context.Context, id int) (usermodel.User, error) {
	service.mu.RLock()
	defer service.mu.RUnlock()

	user, ok := service.items[id]
	if !ok {
		return usermodel.User{}, fmt.Errorf("user %d not found", id)
	}
	return user, nil
}

func (service *inMemoryUserService) Delete(_ context.Context, id int) error {
	service.mu.Lock()
	defer service.mu.Unlock()

	if _, ok := service.items[id]; !ok {
		return fmt.Errorf("user %d not found", id)
	}
	delete(service.items, id)
	return nil
}

func (service *inMemoryUserService) List(_ context.Context) ([]usermodel.User, error) {
	service.mu.RLock()
	items := make([]usermodel.User, 0, len(service.items))
	for _, user := range service.items {
		items = append(items, user)
	}
	service.mu.RUnlock()

	sort.Slice(items, func(i int, j int) bool {
		return items[i].Id < items[j].Id
	})
	return items, nil
}

func (service *inMemoryUserService) String(_ context.Context) (string, error) {
	service.mu.RLock()
	count := len(service.items)
	service.mu.RUnlock()

	return "users:" + strconv.Itoa(count), nil
}
