package token

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

const maxStoreBytes = 4 << 20

type Store struct {
	Path     string
	AdminUID int
}

// verifyDirTrusted applies the directory-level precondition shared by reads
// and administrative changes: a real directory, owned by root or the effective
// user, without group or other write access. A directory an attacker can write
// to lets a crafted store be swapped in, so both paths fail closed on it.
func (s Store) verifyDirTrusted() error {
	dir := filepath.Dir(s.Path)
	st, e := os.Lstat(dir)
	if e != nil || !st.IsDir() || st.Mode().Perm()&0022 != 0 {
		return errors.New("protected token directory required")
	}
	if stat, ok := st.Sys().(*syscall.Stat_t); ok && stat.Uid != 0 && int(stat.Uid) != os.Geteuid() {
		return errors.New("protected token directory required")
	}
	return nil
}

func (s Store) read() ([]Record, error) {
	if e := s.verifyDirTrusted(); e != nil {
		return nil, e
	}
	f, e := os.OpenFile(s.Path, os.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if errors.Is(e, os.ErrNotExist) {
		return []Record{}, nil
	}
	if e != nil {
		return nil, e
	}
	defer f.Close()
	st, e := f.Stat()
	if e != nil || !st.Mode().IsRegular() || st.Mode().Perm()&0027 != 0 || st.Size() > maxStoreBytes {
		return nil, errors.New("unsafe token store")
	}
	if stat, ok := st.Sys().(*syscall.Stat_t); ok && stat.Uid != 0 && int(stat.Uid) != os.Geteuid() {
		return nil, errors.New("untrusted token store owner")
	}
	var records []Record
	d := json.NewDecoder(io.LimitReader(f, maxStoreBytes))
	d.DisallowUnknownFields()
	if e = d.Decode(&records); e != nil {
		return nil, e
	}
	var trailing any
	if e = d.Decode(&trailing); e != io.EOF {
		return nil, errors.New("exactly one token JSON document required")
	}
	return records, nil
}
func (s Store) Verify(secret string) (Record, error) {
	if len(secret) > 256 {
		return Record{}, errors.New("invalid credential")
	}
	records, e := s.read()
	if e != nil {
		return Record{}, e
	}
	h := hash(secret)
	for _, r := range records {
		if subtle.ConstantTimeCompare([]byte(h), []byte(r.Hash)) == 1 && r.Status == "active" && time.Now().Before(r.Expires) {
			r.Hash = ""
			return r, nil
		}
	}
	return Record{}, errors.New("invalid credential")
}
func (s Store) change(fn func(*[]Record) error) error {
	if os.Geteuid() != s.AdminUID {
		return errors.New("local administrator identity required")
	}
	if e := s.verifyDirTrusted(); e != nil {
		return e
	}
	lock, e := os.OpenFile(s.Path+".lock", os.O_CREATE|os.O_RDWR|unix.O_NOFOLLOW, 0600)
	if e != nil {
		return e
	}
	defer lock.Close()
	if e = unix.Flock(int(lock.Fd()), unix.LOCK_EX); e != nil {
		return e
	}
	defer unix.Flock(int(lock.Fd()), unix.LOCK_UN)
	records, e := s.read()
	if e != nil {
		return e
	}
	if e = fn(&records); e != nil {
		return e
	}
	data, e := json.MarshalIndent(records, "", "  ")
	if e != nil {
		return e
	}
	if len(data) > maxStoreBytes {
		return errors.New("token store exceeds 4 MiB")
	}
	return Atomic(s.Path, data, 0640)
}
func Atomic(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	f, e := os.CreateTemp(dir, ".hostlens-")
	if e != nil {
		return e
	}
	name := f.Name()
	defer os.Remove(name)
	if e = f.Chmod(mode); e == nil {
		_, e = f.Write(data)
	}
	if e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e == nil {
		e = ce
	}
	if e == nil {
		e = os.Rename(name, path)
	}
	if e != nil {
		return e
	}
	d, e := os.Open(dir)
	if e != nil {
		return e
	}
	defer d.Close()
	return d.Sync()
}
func (s Store) Create(name string, roles []string, expiry time.Time) (Record, string, error) {
	if strings.TrimSpace(name) == "" || len(name) > 128 || !RolesOK(roles) || !expiry.After(time.Now()) {
		return Record{}, "", errors.New("name, valid roles, and future expiry required")
	}
	secret := Random(32)
	r := Record{Random(12), name, roles, expiry, "active", hash(secret)}
	e := s.change(func(rs *[]Record) error { *rs = append(*rs, r); return nil })
	r.Hash = ""
	return r, secret, e
}
func (s Store) List(all bool) ([]Record, error) {
	if os.Geteuid() != s.AdminUID {
		return nil, errors.New("local administrator identity required")
	}
	rs, e := s.read()
	if e != nil {
		return nil, e
	}
	out := []Record{}
	for _, r := range rs {
		if r.Status == "active" && !time.Now().Before(r.Expires) {
			r.Status = "expired"
		}
		if all || r.Status == "active" {
			r.Hash = ""
			out = append(out, r)
		}
	}
	return out, nil
}
func (s Store) Update(id string, roles []string, revoke bool) error {
	if !revoke && !RolesOK(roles) {
		return errors.New("valid replacement roles required")
	}
	return s.change(func(rs *[]Record) error {
		for i := range *rs {
			if (*rs)[i].ID == id {
				if revoke {
					(*rs)[i].Status = "revoked"
				} else {
					(*rs)[i].Roles = roles
				}
				return nil
			}
		}
		return fmt.Errorf("token ID not found")
	})
}
func (s Store) Rotate(id string, expiry, overlap time.Time) (Record, string, error) {
	if !overlap.After(time.Now()) || overlap.After(expiry) {
		return Record{}, "", errors.New("explicit future overlap not later than new expiry required")
	}
	secret := Random(32)
	var out Record
	e := s.change(func(rs *[]Record) error {
		for i := range *rs {
			r := &(*rs)[i]
			if r.ID == id && r.Status == "active" && r.Expires.After(time.Now()) {
				if overlap.After(r.Expires) {
					return errors.New("overlap exceeds old expiry")
				}
				r.Expires = overlap
				out = Record{Random(12), r.Name, append([]string{}, r.Roles...), expiry, "active", hash(secret)}
				*rs = append(*rs, out)
				return nil
			}
		}
		return errors.New("active token ID not found")
	})
	out.Hash = ""
	return out, secret, e
}
