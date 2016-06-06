package influxql

// UserInfo is an interface that holds user information.
type UserInfo interface {
	// Name returns the name of the user.
	Name() string

	// IsAdmin returns if this user is an admin.
	IsAdmin() bool

	// Authorize checks if the user is authorized to access the database.
	Authorize(privilege Privilege, database string) bool
}

// anonymousUser is a user that has all privileges and no name.
// The anonymousUser is what gets set when no user is set in the execution context.
type anonymousUser struct{}

func (*anonymousUser) Name() string {
	return ""
}

func (*anonymousUser) IsAdmin() bool {
	return true
}

func (*anonymousUser) Authorize(privilege Privilege, database string) bool {
	return true
}

// ActAsAdmin returns a new UserInfo that will return true for IsAdmin()
// but will retain the name of the underlying user.
func ActAsAdmin(user UserInfo) UserInfo {
	return sudoUser{UserInfo: user}
}

type sudoUser struct {
	UserInfo
}

func (u sudoUser) Name() string {
	return u.UserInfo.Name()
}

func (sudoUser) IsAdmin() bool {
	return true
}

func (sudoUser) Authorize(privilege Privilege, database string) bool {
	return true
}
