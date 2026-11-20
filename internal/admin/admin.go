package admin

import "github.com/yohannes/chines-backend/internal/user"

type UserPage struct {
	Users    []user.User `json:"users"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
	Total    int         `json:"total"`
}

type CreateUserResult struct {
	User         *user.User `json:"user"`
	TemporaryPwd string     `json:"temporary_password"`
}

type Overview struct {
	Classes           int `json:"classes"`
	Students          int `json:"students"`
	Teachers          int `json:"teachers"`
	Admins            int `json:"admins"`
	ActiveStudents7d  int `json:"active_students_7d"`
	UnassignedClasses int `json:"unassigned_classes"`
}
