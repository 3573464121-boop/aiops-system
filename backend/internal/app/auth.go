package app

import (
	"fmt"
	"strings"
	"time"

	"aiops-mvp/internal/domain"
	"golang.org/x/crypto/bcrypt"
)

// Authenticate 校验用户名密码，成功返回用户。为避免暴露账号是否存在，
// 无论是查无此人还是密码错误，都返回同一句提示。
func (s *Service) Authenticate(username, password string) (domain.User, error) {
	username = strings.TrimSpace(username)
	u, err := s.Repo.GetUserByUsername(username)
	if err != nil {
		return domain.User{}, fmt.Errorf("用户名或密码错误")
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		return domain.User{}, fmt.Errorf("用户名或密码错误")
	}
	return u, nil
}

// CreateUser 新建一个用户（密码用 bcrypt 存哈希）。role 非 admin 一律按 viewer。
func (s *Service) CreateUser(username, password, role string) (domain.User, error) {
	username = strings.TrimSpace(username)
	if username == "" || strings.TrimSpace(password) == "" {
		return domain.User{}, fmt.Errorf("用户名与密码不能为空")
	}
	if role != "admin" {
		role = "viewer"
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return domain.User{}, err
	}
	u := domain.User{
		ID:           fmt.Sprintf("USR-%d", time.Now().UnixNano()),
		Username:     username,
		PasswordHash: string(hash),
		Role:         role,
		CreatedAt:    time.Now(),
	}
	if err := s.Repo.CreateUser(u); err != nil {
		return domain.User{}, err
	}
	return u, nil
}

// EnsureSeedAdmin 在系统还没有任何用户时，播种一个管理员账号，便于首次登录。
// 返回是否实际创建了账号，以便调用方决定要不要在日志里提示默认口令。
func (s *Service) EnsureSeedAdmin(username, password string) (bool, error) {
	n, err := s.Repo.CountUsers()
	if err != nil {
		return false, err
	}
	if n > 0 {
		return false, nil
	}
	if _, err := s.CreateUser(username, password, "admin"); err != nil {
		return false, err
	}
	return true, nil
}

func (s *Service) ListUsers() ([]domain.User, error) { return s.Repo.ListUsers() }
