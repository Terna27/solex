package grants

import (
	"crypto/rand"
	"encoding/hex"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var (
	emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
)

type User struct {
	ID           string    `json:"id"`
	FullName     string    `json:"full_name"`
	Email        string    `json:"email"`
	Phone        string    `json:"phone"`
	PasswordHash string    `json:"password_hash,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (u User) Sanitized() User {
	u.PasswordHash = ""
	return u
}

type Session struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

func (s Session) IsExpired(now time.Time) bool {
	return now.After(s.ExpiresAt)
}

type RegisterInput struct {
	FullName        string `json:"full_name"`
	Email           string `json:"email"`
	Phone           string `json:"phone"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirm_password"`
}

type LoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func NormalizeEmail(email string) (string, error) {
	e := strings.ToLower(strings.TrimSpace(email))
	if len(e) < 5 || len(e) > 254 {
		return "", ErrInvalidEmail
	}
	if !emailRegex.MatchString(e) {
		return "", ErrInvalidEmail
	}
	if _, err := mail.ParseAddress(e); err != nil {
		return "", ErrInvalidEmail
	}
	return e, nil
}

func NormalizePhone(phone string) (string, error) {
	p := strings.TrimSpace(phone)
	p = strings.ReplaceAll(p, " ", "")
	p = strings.ReplaceAll(p, "-", "")
	if strings.HasPrefix(p, "0") && len(p) == 11 {
		p = "+234" + p[1:]
	} else if strings.HasPrefix(p, "234") && len(p) == 13 {
		p = "+" + p
	}
	if !phonePattern.MatchString(p) {
		return "", ErrInvalidPhone
	}
	return p, nil
}

func ValidatePassword(password, confirmPassword string) error {
	if len(password) < 8 {
		return ErrInvalidPassword
	}
	if password != confirmPassword {
		return ErrPasswordMismatch
	}
	return nil
}

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func CheckPassword(hash, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func generateSecureToken(byteLen int) (string, error) {
	b := make([]byte, byteLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

const tempPwCharset = "abcdefghjkmnpqrstuvwxyzABCDEFGHJKMNPQRSTUVWXYZ23456789!@#$%"

func generateSecurePassword(length int) (string, error) {
	if length < 8 {
		length = 12
	}
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	res := make([]byte, length)
	for i, b := range bytes {
		res[i] = tempPwCharset[int(b)%len(tempPwCharset)]
	}
	return string(res), nil
}
