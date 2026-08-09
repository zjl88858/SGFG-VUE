package sysproxy

type Controller interface {
	Available() (bool, string)
	Enable(server string) error
	Restore() error
}

func New() Controller {
	return newController()
}
