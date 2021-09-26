package v0

import (
	"github.com/drakos74/free-coin/client"
	"github.com/drakos74/free-coin/client/local"

	"github.com/drakos74/free-coin/internal/account"
	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/storage"
	botlocal "github.com/drakos74/free-coin/user/local"
)

type App struct {
	upstream client.Factory
	history  storage.Shard
	user     api.User
	storage  storage.Shard
	registry storage.EventRegistry
	accounts []account.Details
}

// New creates a new app with the given trade client and persistence factory.
func New(accounts ...account.Details) *App {
	user, _ := botlocal.NewUser("coin_click_bot")

	return &App{
		upstream: local.VoidFactory(),
		history:  storage.MockShard(),
		user:     user,
		storage:  storage.MockShard(),
		registry: storage.MockEventRegistry(),
		accounts: accounts,
	}
}

func (a *App) GetAccounts() []account.Details {
	return a.accounts
}

// Upstream defines the main trade source for the app.
func (a *App) Upstream(upstream func(since int64) (api.Client, error)) *App {
	a.upstream = upstream
	return a
}

// GetUpstream returns the upstream source factory.
func (a *App) GetUpstream() client.Factory {
	return a.upstream
}

// History defines the persistence implementation for trade historical data.
func (a *App) History(history storage.Shard) *App {
	a.history = history
	return a
}

// GetHistory returns the history storage factory.
func (a *App) GetHistory() storage.Shard {
	return a.history
}

// User defines the user interface implementation for the app.
func (a *App) User(user api.User) *App {
	a.user = user
	return a
}

// GetUser returns the user implementation.
func (a *App) GetUser() api.User {
	return a.user
}

// Storage defines the storage implementation for the app state.
func (a *App) Storage(shard storage.Shard) *App {
	a.storage = shard
	return a
}

// GetStorage returns the app state storage factory.
func (a *App) GetStorage() storage.Shard {
	return a.storage
}

// Registry defines the registry implementation for the app events.
func (a *App) Registry(registry storage.EventRegistry) *App {
	a.registry = registry
	return a
}

// GetRegistry returns the registry implementation factory for tracking app events.
func (a *App) GetRegistry() storage.EventRegistry {
	return a.registry
}
