package accounts

import (
	"context"

	model "example.com/rpl/account-service/generated/account"
	storesql "example.com/rpl/account-service/generated/account/sql"
	"example.com/rpl/account-service/generated/account/validation"
)

// Service is handwritten application code built on top of generated types.
type Service struct {
	store *storesql.Store
}

func New(db storesql.Executor) *Service {
	return &Service{store: storesql.NewStore(db)}
}

func (service *Service) Init(ctx context.Context) error {
	return service.store.Init(ctx)
}

func (service *Service) Register(ctx context.Context, account model.Account) error {
	if err := validation.Validate(account); err != nil {
		return err
	}
	return service.store.Create(ctx, account)
}

func (service *Service) FindByEmail(ctx context.Context, email string) (model.Account, error) {
	return service.store.Get(ctx, storesql.Where(storesql.ColumnEmail, email))
}
