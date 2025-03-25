package bind

import (
	"fmt"
	"testing"
)

type Admin struct {
	ID        int
	Username  string
	Nickname  string
	Avatar    string
	Password  string
	Roles     []string
	CreatedAt int64
	UpdatedAt int64
}

type AdminPb struct {
	ID        int64
	Username  string
	Nickname  string
	Avatar    *string
	Password  string
	Roles     []string
	CreatedAt int64
	UpdatedAt int64
}

type AdminCreate struct {
	ID       int
	Username string
	Nickname string
	Avatar   string
	Password string
	Roles    []string
}

func (a *AdminCreate) SetID(id int) *AdminCreate {
	a.ID = id
	return a
}

func (a *AdminCreate) SetUsername(username string) *AdminCreate {
	a.Username = username
	return a
}

func (a *AdminCreate) SetNickname(nickname string) *AdminCreate {
	a.Nickname = nickname
	return a
}

func (a *AdminCreate) SetAvatar(avatar string) *AdminCreate {
	a.Avatar = avatar
	return a
}

//func (a *AdminCreate) SetNillableAvatar(avatar *string) *AdminCreate {
//	if avatar != nil {
//		a.Avatar = *avatar
//	}
//	return a
//}
//
//func (a *AdminCreate) ClearAvatar() *AdminCreate {
//	a.Avatar = ""
//	return a
//}

func (a *AdminCreate) SetPassword(password string) *AdminCreate {
	a.Password = password
	return a
}

func (a *AdminCreate) SetRoles(roles []string) *AdminCreate {
	a.Roles = roles
	return a
}

func TestGen(t *testing.T) {
	gen := Gen(NewGenConf(Admin{}, AdminPb{}, AdminCreate{}))
	fmt.Println(gen)
}
