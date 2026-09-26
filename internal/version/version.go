package version

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

// Get reads the current version from a .rocket-version file
func Get(versionFilePath string) (string, error) {
	data, err := os.ReadFile(versionFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			// If the file doesn't exist, initialize it with 0.1.0
			if err := Set(versionFilePath, "0.1.0"); err != nil {
				return "", fmt.Errorf("failed to initialize version file: %w", err)
			}
			return "0.1.0", nil
		}
		return "", fmt.Errorf("failed to read version file: %w", err)
	}
	version := strings.TrimSpace(string(data))
	if version == "" {
		return "", fmt.Errorf("version file is empty")
	}
	// Validate semver format
	if err := validateSemver(version); err != nil {
		return "", err
	}
	return version, nil
}

// Set writes a version to a .rocket-version file
func Set(versionFilePath, version string) error {
	if err := validateSemver(version); err != nil {
		return err
	}
	err := os.WriteFile(versionFilePath, []byte(version+"\n"), 0644)
	if err != nil {
		return fmt.Errorf("failed to write version file: %w", err)
	}
	return nil
}

// CalculateBump calculates the next version based on the bump type
func CalculateBump(currentVersion, bumpType string) (string, error) {
	if err := validateSemver(currentVersion); err != nil {
		return "", err
	}
	parts := strings.Split(currentVersion, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid semver format: %s", currentVersion)
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return "", fmt.Errorf("invalid major version: %s", parts[0])
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", fmt.Errorf("invalid minor version: %s", parts[1])
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return "", fmt.Errorf("invalid patch version: %s", parts[2])
	}
	switch bumpType {
	case "major":
		major++
		minor = 0
		patch = 0
	case "minor":
		minor++
		patch = 0
	case "patch":
		patch++
	default:
		return "", fmt.Errorf("invalid bump type: %s (must be major, minor, or patch)", bumpType)
	}

	return fmt.Sprintf("%d.%d.%d", major, minor, patch), nil
}

// validateSemver checks if a version string is in valid semver format
func validateSemver(version string) error {
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return fmt.Errorf("invalid semver format: %s (expected format: X.Y.Z)", version)
	}
	for i, part := range parts {
		if _, err := strconv.Atoi(part); err != nil {
			labels := []string{"major", "minor", "patch"}
			return fmt.Errorf("invalid %s version: %s (must be a number)", labels[i], part)
		}
	}
	return nil
}

// Stale returns the X.Y.Z tags older than the version just before current,
// oldest first: what a cleanup may delete while keeping current and one
// version to roll back to. Tags newer than current, and tags that are not
// plain X.Y.Z (e.g. "latest"), are never returned.
func Stale(tags []string, current string) []string {
	cur, ok := parseSemver(current)
	if !ok {
		return nil
	}

	var older [][3]int
	byVersion := map[[3]int]string{}
	for _, tag := range tags {
		v, ok := parseSemver(tag)
		if !ok || compareSemver(v, cur) >= 0 {
			continue
		}
		older = append(older, v)
		byVersion[v] = tag
	}
	sort.Slice(older, func(i, j int) bool { return compareSemver(older[i], older[j]) < 0 })

	// The last one is the previous version: kept for rollback.
	if len(older) <= 1 {
		return nil
	}
	stale := make([]string, 0, len(older)-1)
	for _, v := range older[:len(older)-1] {
		stale = append(stale, byVersion[v])
	}
	return stale
}

// parseSemver parses a plain X.Y.Z of digits only.
func parseSemver(s string) ([3]int, bool) {
	var v [3]int
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return v, false
	}
	for i, part := range parts {
		if part == "" || strings.Trim(part, "0123456789") != "" {
			return v, false
		}
		n, err := strconv.Atoi(part)
		if err != nil {
			return v, false
		}
		v[i] = n
	}
	return v, true
}

func compareSemver(a, b [3]int) int {
	for i := range a {
		if a[i] != b[i] {
			return a[i] - b[i]
		}
	}
	return 0
}
