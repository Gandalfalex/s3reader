// Package config loads S3 connection profiles from settings.json.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type Profile struct {
	Name            string `json:"name"`
	Endpoint        string `json:"endpoint,omitempty"`        // custom endpoint (MinIO, R2, Wasabi, ...); empty = AWS
	Region          string `json:"region,omitempty"`          // defaults to us-east-1
	AccessKeyID     string `json:"accessKeyId,omitempty"`     // supports ${ENV_VAR}
	SecretAccessKey string `json:"secretAccessKey,omitempty"` // supports ${ENV_VAR}
	SessionToken    string `json:"sessionToken,omitempty"`    // supports ${ENV_VAR}
	AWSProfile      string `json:"awsProfile,omitempty"`      // use ~/.aws profile instead of static keys
	Bucket          string `json:"bucket,omitempty"`          // optional: open this bucket directly
	Prefix          string `json:"prefix,omitempty"`          // optional: start in this prefix
	PathStyle       bool   `json:"pathStyle,omitempty"`       // force path-style addressing (MinIO)
}

type Settings struct {
	Profiles []Profile `json:"profiles"`
}

func Path(flagPath string) (string, error) {
	if flagPath != "" {
		return flagPath, nil
	}
	candidates := []string{"settings.json"}
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		candidates = append(candidates, filepath.Join(dir, "s3reader", "settings.json"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, ".config", "s3reader", "settings.json"))
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}
	return "", fmt.Errorf("no settings.json found (looked in %v)", candidates)
}

func Load(path string) (*Settings, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s Settings
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if len(s.Profiles) == 0 {
		return nil, errors.New("settings.json has no profiles")
	}
	for i := range s.Profiles {
		p := &s.Profiles[i]
		p.AccessKeyID = os.ExpandEnv(p.AccessKeyID)
		p.SecretAccessKey = os.ExpandEnv(p.SecretAccessKey)
		p.SessionToken = os.ExpandEnv(p.SessionToken)
		p.Endpoint = os.ExpandEnv(p.Endpoint)
		if p.Name == "" {
			p.Name = fmt.Sprintf("profile-%d", i+1)
		}
	}
	return &s, nil
}
