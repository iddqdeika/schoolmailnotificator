package definitions

type MailProvider interface {
}

type Notificator interface {
}

// предоставляет конфигурации по данному пути
// по умолчанию разделитель - точка
type Config interface {
	GetString(path string) (string, error)
	GetInt(path string) (int, error)
	GetArray(path string) ([]Config, error)
	Child(path string) Config
}
