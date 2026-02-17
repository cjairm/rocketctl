package version

import (
	"fmt"
	"os"
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
