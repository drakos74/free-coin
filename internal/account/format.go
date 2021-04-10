package account

import (
	"fmt"

	"github.com/drakos74/free-coin/internal/api"
)

type Format struct {
	user     Name
	exchange api.ExchangeName
}

func NewFormat(user Name, api api.ExchangeName) Format {
	return Format{
		user:     user,
		exchange: api,
	}
}

func (f Format) Key() string {
	return fmt.Sprintf("%s_%s_KEY", f.user, f.exchange)
}

func (f Format) Secret() string {
	return fmt.Sprintf("%s_%s_SECRET", f.user, f.exchange)
}

func (f Format) ChatID() string {
	return fmt.Sprintf("%s_CHAT_ID", f.user)
}

func (f Format) Token() string {
	return fmt.Sprintf("%s_BOT_TOKEN", f.user)
}
