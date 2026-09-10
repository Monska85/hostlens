package linux

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// SecretIsMasked verifies systemd's immutable inaccessible bind mount, including
// directory masks that return ENOENT to a caller with DAC_READ_SEARCH.
func SecretIsMasked(secret string) error {
	b, e := os.ReadFile("/proc/self/mountinfo")
	if e != nil {
		return e
	}
	unescape := strings.NewReplacer(`\040`, " ", `\011`, "\t", `\012`, "\n", `\134`, `\`)
	for _, line := range strings.Split(string(b), "\n") {
		f := strings.Fields(line)
		if len(f) < 7 {
			continue
		}
		root := unescape.Replace(f[3])
		mount := unescape.Replace(f[4])
		if !strings.Contains(root, "/systemd/inaccessible/") {
			continue
		}
		if secret != mount && !strings.HasPrefix(secret, mount+"/") {
			continue
		}
		if !strings.Contains(","+f[5]+",", ",ro,") {
			continue
		}
		st, e := os.Stat(mount)
		if e != nil {
			continue
		}
		raw, ok := st.Sys().(*syscall.Stat_t)
		if !ok || raw.Uid != 0 || st.Mode().Perm() != 0 {
			continue
		}
		reference, e := os.Stat(filepath.Join("/run/systemd/inaccessible", filepath.Base(root)))
		if e != nil || !os.SameFile(st, reference) {
			continue
		}
		if !st.IsDir() && st.Size() != 0 {
			continue
		}
		if filepath.Clean(secret) != secret {
			return errors.New("unclean configured secret path")
		}
		return nil
	}
	return errors.New("configured secret is not covered by a verified inaccessible systemd mount")
}
