package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofrs/flock"
)

const maxStoreBytes = 1 << 20

type Token struct {
	ID      string    `json:"id"`
	Name    string    `json:"name"`
	Roles   []string  `json:"roles"`
	Expires time.Time `json:"expires"`
	Revoked bool      `json:"revoked,omitempty"`
	Hash    string    `json:"hash,omitempty"`
	Legacy  bool      `json:"legacy,omitempty"`
}

type Store struct {
	Path     string
	TokenGID int
}

func random(size int) ([]byte, error) {
	value := make([]byte, size)
	_, err := rand.Read(value)
	return value, err
}

func digest(secret string) string {
	sum := sha256.Sum256([]byte("hostlens-token-v2\x00" + secret))
	return hex.EncodeToString(sum[:])
}

func roleValid(role string) bool { return role == "observe" || role == "repair" }

func (s Store) root() (*os.Root, error) {
	dir := filepath.Dir(s.Path)
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode().Perm()&0022 != 0 {
		return nil, errors.New("protected token directory required")
	}
	if !ownerTrusted(info) {
		return nil, errors.New("untrusted token directory owner")
	}
	return os.OpenRoot(dir)
}

func (s Store) read(root *os.Root) ([]Token, error) {
	name := filepath.Base(s.Path)
	file, err := root.Open(name)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0027 != 0 || info.Size() > maxStoreBytes || info.Size() < 0 {
		return nil, errors.New("unsafe token store")
	}
	if !ownerTrusted(info) {
		return nil, errors.New("untrusted token store owner")
	}
	decoder := json.NewDecoder(io.LimitReader(file, maxStoreBytes+1))
	decoder.DisallowUnknownFields()
	var records []Token
	if err := decoder.Decode(&records); err != nil {
		return nil, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, errors.New("trailing token store data")
	}
	ids := make(map[string]struct{}, len(records))
	hashes := make(map[string]struct{}, len(records))
	for _, record := range records {
		if record.ID == "" || len(record.Hash) != 64 || record.Name == "" {
			return nil, errors.New("invalid token record")
		}
		if _, err := hex.DecodeString(record.Hash); err != nil {
			return nil, errors.New("invalid token hash")
		}
		if _, exists := ids[record.ID]; exists {
			return nil, errors.New("duplicate token ID")
		}
		if _, exists := hashes[record.Hash]; exists {
			return nil, errors.New("duplicate token hash")
		}
		ids[record.ID] = struct{}{}
		hashes[record.Hash] = struct{}{}
		if !record.Revoked && len(record.Roles) == 0 {
			return nil, errors.New("active token without role")
		}
		for _, role := range record.Roles {
			if !roleValid(role) {
				return nil, errors.New("invalid token role")
			}
		}
	}
	return records, nil
}

// Verify performs a fresh bounded read for every request. It never returns
// the stored hash to a caller.
func (s Store) Verify(secret string) (Token, error) {
	if len(secret) < 32 || len(secret) > 128 {
		return Token{}, errors.New("invalid credential")
	}
	root, err := s.root()
	if err != nil {
		return Token{}, err
	}
	defer root.Close()
	records, err := s.read(root)
	if err != nil {
		return Token{}, err
	}
	modern := digest(secret)
	legacyHash := sha256.Sum256([]byte(secret))
	legacy := hex.EncodeToString(legacyHash[:])
	for _, record := range records {
		want := modern
		if record.Legacy {
			want = legacy
		}
		if subtle.ConstantTimeCompare([]byte(want), []byte(record.Hash)) == 1 && !record.Revoked && (record.Expires.IsZero() || time.Now().Before(record.Expires)) {
			record.Hash = ""
			return record, nil
		}
	}
	return Token{}, errors.New("invalid credential")
}

func (s Store) update(ctx context.Context, change func(*[]Token) error) error {
	root, err := s.root()
	if err != nil {
		return err
	}
	defer root.Close()
	lock := flock.New(s.Path + ".lock")
	locked, err := lock.TryLockContext(ctx, 20*time.Millisecond)
	if err != nil || !locked {
		return errors.Join(err, errors.New("token store lock unavailable"))
	}
	defer lock.Unlock()
	records, err := s.read(root)
	if err != nil {
		return err
	}
	if err := change(&records); err != nil {
		return err
	}
	data, err := json.Marshal(records)
	if err != nil || len(data) > maxStoreBytes {
		return errors.New("token store too large or invalid")
	}
	nonce, err := random(12)
	if err != nil {
		return err
	}
	tmp := ".tokens-" + hex.EncodeToString(nonce)
	file, err := root.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0640)
	if err != nil {
		return err
	}
	defer root.Remove(tmp)
	if s.TokenGID > 0 && os.Geteuid() == 0 {
		if err := file.Chown(-1, s.TokenGID); err != nil {
			file.Close()
			return err
		}
	}
	_, err = file.Write(data)
	if err == nil {
		err = file.Sync()
	}
	err = errors.Join(err, file.Close())
	if err != nil {
		return err
	}
	if err := root.Rename(tmp, filepath.Base(s.Path)); err != nil {
		return err
	}
	directory, err := root.Open(".")
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

func (s Store) Create(ctx context.Context, name string, roles []string, expires time.Time) (Token, string, error) {
	if name == "" || len(name) > 128 || strings.ContainsAny(name, "\x00\n\r") || len(roles) == 0 || (!expires.IsZero() && !expires.After(time.Now())) {
		return Token{}, "", errors.New("invalid token name, roles, or expiry")
	}
	for _, role := range roles {
		if !roleValid(role) {
			return Token{}, "", fmt.Errorf("invalid role %q", role)
		}
	}
	id, err := random(12)
	if err != nil {
		return Token{}, "", err
	}
	value, err := random(32)
	if err != nil {
		return Token{}, "", err
	}
	secret := base64.RawURLEncoding.EncodeToString(value)
	record := Token{ID: hex.EncodeToString(id), Name: name, Roles: roles, Expires: expires, Hash: digest(secret)}
	if err := s.update(ctx, func(records *[]Token) error { *records = append(*records, record); return nil }); err != nil {
		return Token{}, "", err
	}
	record.Hash = ""
	return record, secret, nil
}

func (s Store) Revoke(ctx context.Context, id string) error {
	return s.update(ctx, func(records *[]Token) error {
		for index := range *records {
			if (*records)[index].ID == id {
				(*records)[index].Revoked = true
				return nil
			}
		}
		return errors.New("token not found")
	})
}

// Rotate publishes a new secret and sets a finite retirement deadline for
// the old one in one locked store update.
func (s Store) Rotate(ctx context.Context, id string, expires, overlap time.Time) (Token, string, error) {
	if id == "" || !overlap.After(time.Now()) || (!expires.IsZero() && (!expires.After(time.Now()) || overlap.After(expires))) {
		return Token{}, "", errors.New("valid ID, future overlap, and compatible new expiry required")
	}
	idBytes, err := random(12)
	if err != nil {
		return Token{}, "", err
	}
	secretBytes, err := random(32)
	if err != nil {
		return Token{}, "", err
	}
	secret := base64.RawURLEncoding.EncodeToString(secretBytes)
	newRecord := Token{ID: hex.EncodeToString(idBytes), Expires: expires, Hash: digest(secret)}
	err = s.update(ctx, func(records *[]Token) error {
		for index := range *records {
			old := &(*records)[index]
			if old.ID != id || old.Revoked || (!old.Expires.IsZero() && !old.Expires.After(time.Now())) {
				continue
			}
			if !old.Expires.IsZero() && overlap.After(old.Expires) {
				return errors.New("overlap exceeds old token expiry")
			}
			newRecord.Name = old.Name
			newRecord.Roles = append([]string(nil), old.Roles...)
			old.Expires = overlap
			*records = append(*records, newRecord)
			return nil
		}
		return errors.New("active token not found")
	})
	if err != nil {
		return Token{}, "", err
	}
	newRecord.Hash = ""
	return newRecord, secret, nil
}

func (s Store) List() ([]Token, error) {
	root, err := s.root()
	if err != nil {
		return nil, err
	}
	defer root.Close()
	records, err := s.read(root)
	for index := range records {
		records[index].Hash = ""
	}
	return records, err
}
