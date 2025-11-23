package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type User struct {
	ID        uuid.UUID `gorm:"type:char(36);primary_key" json:"id"`
	Username  string    `gorm:"type:varchar(50);uniqueIndex;not null" json:"username"`
	Email     string    `gorm:"type:varchar(100);uniqueIndex;not null" json:"email"`
	Password  string    `gorm:"type:varchar(255);not null" json:"-"`
	Role      string    `gorm:"type:varchar(20);default:'user';not null" json:"role"`
	FirstName string    `gorm:"type:varchar(50)" json:"first_name"`
	LastName  string    `gorm:"type:varchar(50)" json:"last_name"`
	Avatar    string    `gorm:"type:varchar(255)" json:"avatar"`
	IsActive  bool      `gorm:"default:true" json:"is_active"`
	LastLogin *time.Time `json:"last_login"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}

func (u *User) TableName() string {
	return "users"
}

type UserRole string

const (
	RoleAdmin   UserRole = "admin"
	RoleManager UserRole = "manager"
	RoleUser    UserRole = "user"
)

func (r UserRole) String() string {
	return string(r)
}

func (r UserRole) IsValid() bool {
	switch r {
	case RoleAdmin, RoleManager, RoleUser:
		return true
	}
	return false
}

func IsValidRole(role string) bool {
	return UserRole(role).IsValid()
}