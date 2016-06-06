package httpd

import (
	"github.com/influxdata/influxdb/influxql"
	"github.com/influxdata/influxdb/services/meta"
)

type UserInfo struct {
	*meta.UserInfo
}

func (u UserInfo) Name() string {
	if u.UserInfo != nil {
		return u.UserInfo.Name
	}
	return ""
}

func (u UserInfo) IsAdmin() bool {
	if u.UserInfo != nil {
		return u.UserInfo.Admin
	}
	return true
}

func (u UserInfo) Authorize(privilege influxql.Privilege, database string) bool {
	if u.UserInfo != nil {
		return u.UserInfo.Authorize(privilege, database)
	}
	return true
}
