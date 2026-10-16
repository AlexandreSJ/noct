package update

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// binaryName is the noct binary's name inside a release archive.
const binaryName = "noct"

// checksumsFile lists the sha256 of every archive of a release.
const checksumsFile = "checksums.txt"

// ArchiveName is the file of a release built for a system, as the release workflow names it.
func ArchiveName(version, goos, goarch string) string {
	return fmt.Sprintf("noct_%s_%s_%s.tar.gz", version, goos, goarch)
}

// ArchiveURL is where the release's archive for this system is downloaded from.
func (s Source) ArchiveURL(version string) string {
	return s.Downloads + "/" + version + "/" + ArchiveName(version, runtime.GOOS, runtime.GOARCH)
}

// Executable is the noct binary that runs, with links followed: the file an update replaces.
func Executable() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("finding the noct executable: %w", err)
	}
	return filepath.EvalSymlinks(path)
}

// CanReplace tells whether this user may replace the binary at target: whether its folder takes
// new files.
func CanReplace(target string) bool {
	probe, err := os.CreateTemp(filepath.Dir(target), ".noct-probe-*")
	if err != nil {
		return false
	}
	probe.Close()
	_ = os.Remove(probe.Name()) // a leftover empty probe file is harmless
	return true
}

// Install replaces the binary at target with the release's build for this system, once its
// checksum matches. The new file is renamed over the old one, so a noct that is running keeps
// its own code until it restarts.
func (s Source) Install(ctx context.Context, version, target string) error {
	name := ArchiveName(version, runtime.GOOS, runtime.GOARCH)
	sums, err := s.get(ctx, s.Downloads+"/"+version+"/"+checksumsFile)
	if err != nil {
		return fmt.Errorf("downloading the checksums of %s: %w", version, err)
	}
	want, err := checksumOf(sums, name)
	if err != nil {
		return err
	}
	archive, err := s.get(ctx, s.Downloads+"/"+version+"/"+name)
	if err != nil {
		return fmt.Errorf("downloading %s: %w", name, err)
	}
	if got := sha256.Sum256(archive); hex.EncodeToString(got[:]) != want {
		return fmt.Errorf("%s does not match its checksum; nothing was changed", name)
	}
	binary, err := binaryIn(archive)
	if err != nil {
		return fmt.Errorf("reading %s: %w", name, err)
	}
	return replaceFile(target, binary, 0o755)
}

// checksumOf finds a file's sha256 in a checksums list, as sha256sum writes it.
func checksumOf(sums []byte, name string) (string, error) {
	lines := bufio.NewScanner(bytes.NewReader(sums))
	for lines.Scan() {
		fields := strings.Fields(lines.Text())
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == name {
			return strings.ToLower(fields[0]), nil
		}
	}
	return "", fmt.Errorf("this release has no build for %s/%s", runtime.GOOS, runtime.GOARCH)
}

// binaryIn reads the noct binary out of a release archive.
func binaryIn(archive []byte) ([]byte, error) {
	unzipped, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, err
	}
	files := tar.NewReader(unzipped)
	for {
		header, err := files.Next()
		if errors.Is(err, io.EOF) {
			return nil, errors.New("there is no noct binary in it")
		}
		if err != nil {
			return nil, err
		}
		if header.Typeflag == tar.TypeReg && filepath.Base(header.Name) == binaryName {
			return io.ReadAll(io.LimitReader(files, maxDownload))
		}
	}
}

// replaceFile writes content next to target and renames it over target, so target is never
// left half written.
func replaceFile(target string, content []byte, mode os.FileMode) error {
	file, err := os.CreateTemp(filepath.Dir(target), ".noct-update-*")
	if err != nil {
		return fmt.Errorf("writing %s: %w", target, err)
	}
	defer os.Remove(file.Name()) // fails harmlessly once renamed
	_, err = file.Write(content)
	err = errors.Join(err, file.Chmod(mode), file.Close())
	if err != nil {
		return fmt.Errorf("writing %s: %w", target, err)
	}
	if err := os.Rename(file.Name(), target); err != nil {
		return fmt.Errorf("replacing %s: %w", target, err)
	}
	return nil
}

// ManualCommand installs the release by hand, for a binary in a folder this user cannot write:
// it downloads the archive and copies the binary over target with sudo.
func (s Source) ManualCommand(version, target string) string {
	return fmt.Sprintf("curl -fsSL %s | tar -xzf - -C /tmp %s && sudo install -m 755 /tmp/%s %s",
		s.ArchiveURL(version), binaryName, binaryName, shellQuote(target))
}

// shellQuote quotes a path for a POSIX shell when it needs it.
func shellQuote(path string) string {
	if !strings.ContainsAny(path, " \t\n'\"\\$`!*?[]{}()<>|&;#~") {
		return path
	}
	return "'" + strings.ReplaceAll(path, "'", `'\''`) + "'"
}
