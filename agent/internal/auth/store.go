package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	defaultIterations = 600_000
	derivedKeyBytes   = 32
	minimumPassword   = 14
)

var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{2,31}$`)

type User struct {
	Username   string    `json:"username"`
	Role       string    `json:"role"`
	Salt       string    `json:"salt"`
	Hash       string    `json:"hash"`
	Iterations int       `json:"iterations"`
	Disabled   bool      `json:"disabled"`
	CreatedAt  time.Time `json:"createdAt"`
}

type PublicUser struct {
	Username  string    `json:"username"`
	Role      string    `json:"role"`
	Disabled  bool      `json:"disabled"`
	CreatedAt time.Time `json:"createdAt"`
}

type fileData struct {
	Version int    `json:"version"`
	Users   []User `json:"users"`
}

type Store struct {
	path  string
	mu    sync.RWMutex
	users map[string]User
}

func Open(path string) (*Store, error) {
	store := &Store{path: path, users: make(map[string]User)}
	if err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *Store) Add(username, role, password string) error {
	username = strings.TrimSpace(username)
	role = strings.ToLower(strings.TrimSpace(role))
	if !usernamePattern.MatchString(username) {
		return errors.New("username must be 3-32 characters and contain only letters, numbers, dot, underscore, or hyphen")
	}
	if role != "admin" && role != "operator" && role != "viewer" {
		return errors.New("role must be admin, operator, or viewer")
	}
	if len(password) < minimumPassword {
		return fmt.Errorf("password must contain at least %d characters", minimumPassword)
	}

	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return fmt.Errorf("generate password salt: %w", err)
	}
	hash := pbkdf2SHA256([]byte(password), salt, defaultIterations, derivedKeyBytes)
	key := strings.ToLower(username)

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.users[key]; exists {
		return errors.New("user already exists")
	}
	s.users[key] = User{
		Username:   username,
		Role:       role,
		Salt:       base64.RawStdEncoding.EncodeToString(salt),
		Hash:       base64.RawStdEncoding.EncodeToString(hash),
		Iterations: defaultIterations,
		CreatedAt:  time.Now().UTC(),
	}
	return s.saveLocked()
}

func (s *Store) Authenticate(username, password string) (PublicUser, bool) {
	s.mu.RLock()
	user, ok := s.users[strings.ToLower(strings.TrimSpace(username))]
	s.mu.RUnlock()
	if !ok || user.Disabled {
		// Equalize some work for missing users without revealing account existence.
		_ = pbkdf2SHA256([]byte(password), make([]byte, 16), defaultIterations, derivedKeyBytes)
		return PublicUser{}, false
	}
	salt, err := base64.RawStdEncoding.DecodeString(user.Salt)
	if err != nil {
		return PublicUser{}, false
	}
	expected, err := base64.RawStdEncoding.DecodeString(user.Hash)
	if err != nil {
		return PublicUser{}, false
	}
	actual := pbkdf2SHA256([]byte(password), salt, user.Iterations, len(expected))
	if subtle.ConstantTimeCompare(actual, expected) != 1 {
		return PublicUser{}, false
	}
	return public(user), true
}

func (s *Store) List() []PublicUser {
	s.mu.RLock()
	defer s.mu.RUnlock()
	users := make([]PublicUser, 0, len(s.users))
	for _, user := range s.users {
		users = append(users, public(user))
	}
	sort.Slice(users, func(i, j int) bool { return users[i].Username < users[j].Username })
	return users
}

func (s *Store) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	content, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read user store: %w", err)
	}
	var data fileData
	if err := json.Unmarshal(content, &data); err != nil {
		return fmt.Errorf("decode user store: %w", err)
	}
	for _, user := range data.Users {
		s.users[strings.ToLower(user.Username)] = user
	}
	return nil
}

func (s *Store) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}
	data := fileData{Version: 1, Users: make([]User, 0, len(s.users))}
	for _, user := range s.users {
		data.Users = append(data.Users, user)
	}
	sort.Slice(data.Users, func(i, j int) bool { return data.Users[i].Username < data.Users[j].Username })
	content, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("encode user store: %w", err)
	}
	temporary := s.path + ".tmp"
	if err := os.WriteFile(temporary, append(content, '\n'), 0o600); err != nil {
		return fmt.Errorf("write temporary user store: %w", err)
	}
	if err := os.Chmod(temporary, 0o600); err != nil {
		return fmt.Errorf("restrict user store permissions: %w", err)
	}
	if err := os.Rename(temporary, s.path); err != nil {
		return fmt.Errorf("replace user store: %w", err)
	}
	return nil
}

func public(user User) PublicUser {
	return PublicUser{Username: user.Username, Role: user.Role, Disabled: user.Disabled, CreatedAt: user.CreatedAt}
}

// pbkdf2SHA256 is a small RFC 8018 PBKDF2 implementation kept local to avoid
// adding a dependency to the privileged agent. Parameters are stored per user.
func pbkdf2SHA256(password, salt []byte, iterations, keyLength int) []byte {
	blocks := (keyLength + sha256.Size - 1) / sha256.Size
	output := make([]byte, 0, blocks*sha256.Size)
	for block := 1; block <= blocks; block++ {
		mac := hmac.New(sha256.New, password)
		mac.Write(salt)
		counter := make([]byte, 4)
		binary.BigEndian.PutUint32(counter, uint32(block))
		mac.Write(counter)
		u := mac.Sum(nil)
		t := append([]byte(nil), u...)
		for i := 1; i < iterations; i++ {
			mac = hmac.New(sha256.New, password)
			mac.Write(u)
			u = mac.Sum(nil)
			for j := range t {
				t[j] ^= u[j]
			}
		}
		output = append(output, t...)
	}
	return output[:keyLength]
}
