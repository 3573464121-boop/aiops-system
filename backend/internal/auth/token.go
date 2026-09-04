// Package auth 提供轻量的登录令牌：用 HMAC-SHA256 自签一个紧凑的令牌，
// 不引入 JWT 依赖。令牌格式为 base64url(payload).base64url(signature)。
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Claims 是令牌里携带的身份信息。
type Claims struct {
	UserID   string `json:"uid"`
	Username string `json:"usr"`
	Role     string `json:"role"`
	TeamID   string `json:"team,omitempty"`
	Exp      int64  `json:"exp"` // 过期时间（Unix 秒）
}

// Signer 用固定密钥签发与校验令牌。
type Signer struct {
	secret []byte
	ttl    time.Duration
}

func NewSigner(secret string, ttl time.Duration) *Signer {
	return &Signer{secret: []byte(secret), ttl: ttl}
}

var b64 = base64.RawURLEncoding

func (s *Signer) sign(payload []byte) string {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write(payload)
	return b64.EncodeToString(mac.Sum(nil))
}

// Issue 为指定身份签发一个带过期时间的令牌。now 由调用方传入，便于测试。
func (s *Signer) Issue(userID, username, role string, now time.Time) (string, error) {
	return s.IssueWithTeam(userID, username, role, "", now)
}

func (s *Signer) IssueWithTeam(userID, username, role, teamID string, now time.Time) (string, error) {
	c := Claims{UserID: userID, Username: username, Role: role, TeamID: teamID, Exp: now.Add(s.ttl).Unix()}
	raw, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	body := b64.EncodeToString(raw)
	return body + "." + s.sign([]byte(body)), nil
}

// Verify 校验令牌签名与有效期，返回其中的身份信息。now 由调用方传入。
func (s *Signer) Verify(token string, now time.Time) (Claims, error) {
	parts := strings.SplitN(strings.TrimSpace(token), ".", 2)
	if len(parts) != 2 {
		return Claims{}, fmt.Errorf("令牌格式非法")
	}
	expected := s.sign([]byte(parts[0]))
	if subtle.ConstantTimeCompare([]byte(expected), []byte(parts[1])) != 1 {
		return Claims{}, fmt.Errorf("令牌签名无效")
	}
	raw, err := b64.DecodeString(parts[0])
	if err != nil {
		return Claims{}, fmt.Errorf("令牌载荷无法解析")
	}
	var c Claims
	if err = json.Unmarshal(raw, &c); err != nil {
		return Claims{}, fmt.Errorf("令牌载荷无法解析")
	}
	if now.Unix() >= c.Exp {
		return Claims{}, fmt.Errorf("令牌已过期，请重新登录")
	}
	return c, nil
}
